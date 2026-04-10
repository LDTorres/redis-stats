package render

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/LDTorres/redis-stats/internal/redisstats"
)

func Report(w io.Writer, report redisstats.Report) {
	fmt.Fprintf(w, "Snapshot: %s\tPing: %s\n", report.Snapshot.CapturedAt.Format(time.RFC3339), report.Snapshot.PingLatency.Round(time.Millisecond))
	if report.Delta != nil {
		fmt.Fprintf(w, "Delta window: %s\n", report.Delta.Elapsed.Round(time.Millisecond))
	}
	if report.MemoryTrend != nil {
		fmt.Fprintf(w, "Memory trend: %s\n", formatMemoryTrend(*report.MemoryTrend))
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	section(tw, "Memory")
	row(tw, "used_memory", formatBytes(report.Snapshot.Memory.UsedMemory), deltaBytes(report.Delta, func(d redisstats.Delta) int64 { return d.UsedMemory }))
	row(tw, "used_memory_rss", formatBytes(report.Snapshot.Memory.UsedMemoryRSS), deltaBytes(report.Delta, func(d redisstats.Delta) int64 { return d.UsedMemoryRSS }))
	row(tw, "used_memory_peak", formatBytes(report.Snapshot.Memory.UsedMemoryPeak), "-")
	row(tw, "used_memory_dataset", formatBytes(report.Snapshot.Memory.UsedMemoryDataset), "-")
	row(tw, "mem_fragmentation_ratio", fmt.Sprintf("%.2fx", report.Snapshot.Memory.FragmentationRatio), deltaFloat(report.Delta, func(d redisstats.Delta) float64 { return d.FragmentationRatio }, " pts"))
	row(tw, "maxmemory", maxMemory(report.Snapshot.Memory.MaxMemory), report.Snapshot.Memory.MaxMemoryPolicy)
	row(tw, "used_memory_trend", trendCurrent(report.MemoryTrend), trendNotes(report.MemoryTrend))

	section(tw, "Load")
	row(tw, "ops_per_sec", fmt.Sprintf("%d", report.Snapshot.Traffic.InstantaneousOpsPerSec), deltaInt(report.Delta, func(d redisstats.Delta) int64 { return d.CommandsProcessed }, " cmds"))
	row(tw, "total_net_input_bytes", formatSignedBytesFromCurrent(report.Snapshot.Traffic.NetInputBytes), deltaBytes(report.Delta, func(d redisstats.Delta) int64 { return d.NetInputBytes }))
	row(tw, "total_net_output_bytes", formatSignedBytesFromCurrent(report.Snapshot.Traffic.NetOutputBytes), deltaBytes(report.Delta, func(d redisstats.Delta) int64 { return d.NetOutputBytes }))
	row(tw, "used_cpu_user", fmt.Sprintf("%.2fs", report.Snapshot.CPU.UsedCPUUser), deltaFloat(report.Delta, func(d redisstats.Delta) float64 { return d.CPUUser }, " s"))
	row(tw, "used_cpu_sys", fmt.Sprintf("%.2fs", report.Snapshot.CPU.UsedCPUSys), deltaFloat(report.Delta, func(d redisstats.Delta) float64 { return d.CPUSys }, " s"))

	section(tw, "Clients")
	row(tw, "connected_clients", fmt.Sprintf("%d", report.Snapshot.Clients.ConnectedClients), "-")
	row(tw, "blocked_clients", fmt.Sprintf("%d", report.Snapshot.Clients.BlockedClients), deltaInt(report.Delta, func(d redisstats.Delta) int64 { return d.BlockedClients }, ""))
	row(tw, "rejected_connections", fmt.Sprintf("%d", report.Snapshot.Clients.RejectedConnections), deltaInt(report.Delta, func(d redisstats.Delta) int64 { return d.RejectedConnections }, ""))
	row(tw, "total_connections_received", fmt.Sprintf("%d", report.Snapshot.Clients.TotalConnectionsReceived), deltaInt(report.Delta, func(d redisstats.Delta) int64 { return d.ConnectionsReceived }, ""))

	section(tw, "Persistence and Replication")
	row(tw, "role", defaultString(report.Snapshot.Persistence.Role, "unknown"), "-")
	row(tw, "connected_replicas", fmt.Sprintf("%d", report.Snapshot.Persistence.ConnectedReplicas), "-")
	row(tw, "rdb_last_bgsave_status", defaultString(report.Snapshot.Persistence.RDBLastBGSaveStatus, "n/a"), formatTime(report.Snapshot.Persistence.RDBLastSaveTime))
	row(tw, "aof_enabled", formatBool(report.Snapshot.Persistence.AOFEnabled), defaultString(report.Snapshot.Persistence.AOFLastBGRewriteStatus, "n/a"))

	section(tw, "General Health")
	row(tw, "uptime", formatDurationSeconds(report.Snapshot.General.UptimeSeconds), "-")
	row(tw, "evicted_keys", fmt.Sprintf("%d", report.Snapshot.General.EvictedKeys), deltaInt(report.Delta, func(d redisstats.Delta) int64 { return d.EvictedKeys }, ""))
	row(tw, "expired_keys", fmt.Sprintf("%d", report.Snapshot.General.ExpiredKeys), deltaInt(report.Delta, func(d redisstats.Delta) int64 { return d.ExpiredKeys }, ""))
	row(tw, "hit_ratio", formatHitRatio(report.Snapshot), "-")
	row(tw, "latest_fork_usec", fmt.Sprintf("%d", report.Snapshot.General.LatestForkUsec), "-")

	section(tw, "Keyspace")
	row(tw, "dbsize", fmt.Sprintf("%d", report.Snapshot.Keyspace.DBSize), "-")
	row(tw, "keys_reported", fmt.Sprintf("%d", report.Snapshot.Keyspace.TotalKeys), "-")
	row(tw, "expiring_keys", fmt.Sprintf("%d", report.Snapshot.Keyspace.ExpiringKeys), "-")
	row(tw, "dbs", formatDBSummary(report.Snapshot.Keyspace.Databases), "-")

	_ = tw.Flush()

	fmt.Fprintln(w)
	if len(report.Warnings) == 0 {
		fmt.Fprintln(w, "Warnings: none")
	} else {
		fmt.Fprintln(w, "Warnings:")
		for _, warning := range report.Warnings {
			fmt.Fprintf(w, "  - %s\n", warning)
		}
	}
	fmt.Fprintln(w, strings.Repeat("-", 72))
}

func section(tw *tabwriter.Writer, title string) {
	fmt.Fprintf(tw, "\n%s\tCurrent\tDelta/Notes\n", title)
}

func row(tw *tabwriter.Writer, label, current, delta string) {
	fmt.Fprintf(tw, "%s\t%s\t%s\n", label, current, delta)
}

func deltaBytes(delta *redisstats.Delta, selector func(redisstats.Delta) int64) string {
	if delta == nil {
		return "-"
	}
	return formatBytesSigned(selector(*delta))
}

func deltaFloat(delta *redisstats.Delta, selector func(redisstats.Delta) float64, suffix string) string {
	if delta == nil {
		return "-"
	}
	return fmt.Sprintf("%+.2f%s", selector(*delta), suffix)
}

func deltaInt(delta *redisstats.Delta, selector func(redisstats.Delta) int64, suffix string) string {
	if delta == nil {
		return "-"
	}
	return fmt.Sprintf("%+d%s", selector(*delta), suffix)
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}

func formatBytesSigned(value int64) string {
	if value == 0 {
		return "0 B"
	}
	sign := "+"
	if value < 0 {
		sign = "-"
		value = -value
	}
	return sign + formatBytes(uint64(value))
}

func maxMemory(value uint64) string {
	if value == 0 {
		return "unlimited"
	}
	return formatBytes(value)
}

func formatBool(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func formatDurationSeconds(seconds int64) string {
	return (time.Duration(seconds) * time.Second).String()
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "n/a"
	}
	return value.Format(time.RFC3339)
}

func formatHitRatio(snapshot redisstats.Snapshot) string {
	total := snapshot.General.KeyspaceHits + snapshot.General.KeyspaceMisses
	if total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", (float64(snapshot.General.KeyspaceHits)/float64(total))*100)
}

func formatDBSummary(dbs map[string]redisstats.KeyspaceDB) string {
	if len(dbs) == 0 {
		return "n/a"
	}
	names := make([]string, 0, len(dbs))
	for name := range dbs {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		db := dbs[name]
		parts = append(parts, fmt.Sprintf("%s=%d keys/%d exp", name, db.Keys, db.Expires))
	}
	return strings.Join(parts, ", ")
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func formatSignedBytesFromCurrent(value int64) string {
	if value <= 0 {
		return fmt.Sprintf("%d B", value)
	}
	return formatBytes(uint64(value))
}

func formatMemoryTrend(trend redisstats.MemoryTrend) string {
	return fmt.Sprintf("%s over %s (%d samples)", trendDirection(trend), trend.Window.Round(time.Second), trend.Samples)
}

func trendCurrent(trend *redisstats.MemoryTrend) string {
	if trend == nil {
		return "warming up"
	}
	return trendDirection(*trend)
}

func trendNotes(trend *redisstats.MemoryTrend) string {
	if trend == nil {
		return "-"
	}
	return fmt.Sprintf("net %s, +steps=%d, -steps=%d", formatBytesSigned(trend.NetUsedMemory), trend.PositiveSteps, trend.NegativeSteps)
}

func trendDirection(trend redisstats.MemoryTrend) string {
	switch {
	case trend.SustainedGrowth:
		return "growing"
	case trend.NetUsedMemory > 0:
		return "slightly up"
	case trend.NetUsedMemory < 0:
		return "down"
	default:
		return "flat"
	}
}
