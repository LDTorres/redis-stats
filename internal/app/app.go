package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/signal"
	"syscall"
	"time"

	"github.com/LDTorres/redis-stats/internal/config"
	"github.com/LDTorres/redis-stats/internal/dashboard"
	"github.com/LDTorres/redis-stats/internal/redisstats"
	"github.com/LDTorres/redis-stats/internal/render"
	"github.com/redis/go-redis/v9"
)

func Run(args []string, stdout, stderr io.Writer) int {
	cfg, err := config.ParseArgs(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			config.Usage(stdout)
			return 0
		}
		fmt.Fprintf(stderr, "error: %v\n\n", err)
		config.Usage(stderr)
		return 2
	}

	opts, err := cfg.RedisOptions()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	client := redis.NewClient(opts)
	defer client.Close()

	switch cfg.Command {
	case config.CommandSnapshot:
		if err := runSnapshot(client, cfg, stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case config.CommandWatch:
		if err := runWatch(client, cfg, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case config.CommandServe:
		if err := runServe(client, cfg, stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case config.CommandTTLAudit:
		if err := runTTLAudit(client, cfg, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "error: unsupported command %q\n", cfg.Command)
		return 2
	}

	return 0
}

func runSnapshot(client *redis.Client, cfg config.Config, stdout io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout*2)
	defer cancel()

	collector := redisstats.NewCollector(client)
	snapshot, err := collector.Collect(ctx)
	if err != nil {
		return err
	}

	render.Report(stdout, redisstats.BuildReport(snapshot, nil, nil, cfg.TrendMinSamples))
	return nil
}

func runServe(client *redis.Client, cfg config.Config, stdout io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(stdout, "dashboard listening on http://%s\n", cfg.Listen)
	server := dashboard.New(cfg, client)
	return server.Run(ctx)
}

func runWatch(client *redis.Client, cfg config.Config, stdout, stderr io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	collector := redisstats.NewCollector(client)
	var previous *redisstats.Snapshot
	var history []redisstats.Snapshot

	for {
		collectCtx, cancel := context.WithTimeout(ctx, cfg.Timeout*2)
		snapshot, err := collector.Collect(collectCtx)
		cancel()
		if err != nil {
			if previous == nil {
				return err
			}
			fmt.Fprintf(stderr, "warn: sample failed: %v\n", err)
		} else {
			history = appendHistory(history, snapshot, cfg.HistorySize)
			report := redisstats.BuildReport(snapshot, previous, history, cfg.TrendMinSamples)
			render.Report(stdout, report)
			previous = &snapshot
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(cfg.Interval):
		}
	}
}

func runTTLAudit(client *redis.Client, cfg config.Config, stdout, stderr io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(stderr, "ttl-audit: scanning DB %d for keys without TTL...\n", cfg.DB)

	collector := redisstats.NewCollector(client)
	lastPrintedAt := time.Now()
	lastPrintedScanned := 0
	audit, err := collector.CollectPersistentKeyAuditWithProgress(ctx, func(progress redisstats.AuditProgress) {
		if progress.ScannedKeys == 0 {
			return
		}
		if progress.ScannedKeys-lastPrintedScanned < 10000 && time.Since(lastPrintedAt) < 5*time.Second {
			return
		}
		fmt.Fprintf(
			stderr,
			"ttl-audit: still running... scanned %d keys, found %d without TTL\n",
			progress.ScannedKeys,
			progress.PersistentKeys,
		)
		lastPrintedAt = time.Now()
		lastPrintedScanned = progress.ScannedKeys
	})
	if err != nil {
		return err
	}

	render.TTLAudit(stdout, audit)
	return nil
}

func appendHistory(history []redisstats.Snapshot, snapshot redisstats.Snapshot, limit int) []redisstats.Snapshot {
	history = append(history, snapshot)
	if len(history) <= limit {
		return history
	}
	return append([]redisstats.Snapshot(nil), history[len(history)-limit:]...)
}
