package redisstats

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Collector struct {
	client *redis.Client
}

type AuditProgress struct {
	ScannedKeys    int
	PersistentKeys int
}

func NewCollector(client *redis.Client) *Collector {
	return &Collector{client: client}
}

func (c *Collector) Collect(ctx context.Context) (Snapshot, error) {
	start := time.Now()
	if err := c.client.Ping(ctx).Err(); err != nil {
		return Snapshot{}, fmt.Errorf("ping redis: %w", err)
	}
	pingLatency := time.Since(start)

	infoRaw, err := c.client.Info(ctx, "all").Result()
	if err != nil {
		return Snapshot{}, fmt.Errorf("collect info: %w", err)
	}

	dbsize, err := c.client.DBSize(ctx).Result()
	if err != nil {
		return Snapshot{}, fmt.Errorf("collect dbsize: %w", err)
	}

	return snapshotFromInfo(parseInfo(infoRaw), dbsize, pingLatency, time.Now()), nil
}

func (c *Collector) CollectPersistentKeyScan(ctx context.Context, sampleSize int) (PersistentKeyScan, error) {
	if sampleSize <= 0 {
		sampleSize = 1
	}

	keys, err := c.sampleKeys(ctx, sampleSize)
	if err != nil {
		return PersistentKeyScan{}, fmt.Errorf("sample keys: %w", err)
	}

	samples, err := c.sampleKeyTTLs(ctx, keys)
	if err != nil {
		return PersistentKeyScan{}, fmt.Errorf("sample key ttl: %w", err)
	}

	return summarizePersistentKeyScan(time.Now(), c.client.Options().DB, samples, persistentGroupLimit), nil
}

func (c *Collector) CollectPersistentKeyAudit(ctx context.Context) (PersistentKeyAudit, error) {
	return c.CollectPersistentKeyAuditWithProgress(ctx, nil)
}

func (c *Collector) CollectPersistentKeyAuditWithProgress(ctx context.Context, progress func(AuditProgress)) (PersistentKeyAudit, error) {
	const scanCount int64 = 500

	var (
		cursor         uint64
		samples        []ttlSample
		scannedKeys    int
		persistentKeys int
	)

	for {
		batch, next, err := c.client.Scan(ctx, cursor, "", scanCount).Result()
		if err != nil {
			return PersistentKeyAudit{}, fmt.Errorf("scan keys: %w", err)
		}

		ttls, err := c.sampleKeyTTLs(ctx, batch)
		if err != nil {
			return PersistentKeyAudit{}, fmt.Errorf("sample key ttl: %w", err)
		}
		samples = append(samples, ttls...)
		for _, sample := range ttls {
			if sample.Err != nil {
				continue
			}
			scannedKeys++
			if sample.TTL == -1 {
				persistentKeys++
			}
		}
		if progress != nil {
			progress(AuditProgress{
				ScannedKeys:    scannedKeys,
				PersistentKeys: persistentKeys,
			})
		}

		cursor = next
		if cursor == 0 {
			break
		}
	}

	return summarizePersistentKeyAudit(time.Now(), c.client.Options().DB, samples, auditGroupLimit, auditExampleLimit), nil
}

func (c *Collector) sampleKeys(ctx context.Context, sampleSize int) ([]string, error) {
	keys := make([]string, 0, sampleSize)
	seen := make(map[string]struct{}, sampleSize)
	var cursor uint64

	for len(keys) < sampleSize {
		count := int64(sampleSize - len(keys))
		if count > 100 {
			count = 100
		}

		batch, next, err := c.client.Scan(ctx, cursor, "", count).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range batch {
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
			if len(keys) >= sampleSize {
				break
			}
		}

		cursor = next
		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

func (c *Collector) sampleKeyTTLs(ctx context.Context, keys []string) ([]ttlSample, error) {
	samples := make([]ttlSample, 0, len(keys))
	if len(keys) == 0 {
		return samples, nil
	}

	pipe := c.client.Pipeline()
	cmds := make([]*redis.DurationCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.PTTL(ctx, key)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	for i, key := range keys {
		ttl, err := cmds[i].Result()
		samples = append(samples, ttlSample{
			Key: key,
			TTL: ttl,
			Err: err,
		})
	}

	return samples, nil
}
