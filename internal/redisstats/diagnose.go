package redisstats

import (
	"fmt"
	"math"
	"time"
)

const (
	minMeaningfulMemoryGrowth = 1 << 20
	highFragmentationRatio    = 1.5
	highPingLatency           = 50 * time.Millisecond
	slowForkThresholdUsec     = 500_000
	minTrendNetGrowth         = 512 << 10
	minTrendStepGrowth        = 64 << 10
)

func BuildReport(current Snapshot, previous *Snapshot, history []Snapshot, trendMinSamples int) Report {
	report := Report{Snapshot: current}
	if previous != nil {
		delta := ComputeDelta(*previous, current)
		report.Delta = &delta
	}
	report.MemoryTrend = AnalyzeMemoryTrend(history, trendMinSamples)
	report.Warnings = warningsFor(current, previous, report.Delta, report.MemoryTrend)
	return report
}

func AnalyzeMemoryTrend(history []Snapshot, trendMinSamples int) *MemoryTrend {
	if len(history) < 2 {
		return nil
	}
	if trendMinSamples < 2 {
		trendMinSamples = 2
	}

	first := history[0]
	last := history[len(history)-1]
	trend := &MemoryTrend{
		Samples:          len(history),
		Window:           last.CapturedAt.Sub(first.CapturedAt),
		NetUsedMemory:    int64(last.Memory.UsedMemory) - int64(first.Memory.UsedMemory),
		NetUsedMemoryRSS: int64(last.Memory.UsedMemoryRSS) - int64(first.Memory.UsedMemoryRSS),
	}

	for i := 1; i < len(history); i++ {
		stepGrowth := int64(history[i].Memory.UsedMemory) - int64(history[i-1].Memory.UsedMemory)
		switch {
		case stepGrowth >= minTrendStepGrowth:
			trend.PositiveSteps++
		case stepGrowth <= -minTrendStepGrowth:
			trend.NegativeSteps++
		}
	}

	observedSteps := len(history) - 1
	allowedNegativeSteps := 0
	if observedSteps >= trendMinSamples {
		allowedNegativeSteps = 1
	}
	trend.SustainedGrowth = len(history) >= trendMinSamples &&
		trend.NetUsedMemory >= minTrendNetGrowth &&
		trend.PositiveSteps >= observedSteps-allowedNegativeSteps-1 &&
		trend.NegativeSteps <= allowedNegativeSteps

	return trend
}

func ComputeDelta(previous, current Snapshot) Delta {
	hitRatio := currentHitRatio(current)
	return Delta{
		Elapsed:             current.CapturedAt.Sub(previous.CapturedAt),
		UsedMemory:          int64(current.Memory.UsedMemory) - int64(previous.Memory.UsedMemory),
		UsedMemoryRSS:       int64(current.Memory.UsedMemoryRSS) - int64(previous.Memory.UsedMemoryRSS),
		FragmentationRatio:  current.Memory.FragmentationRatio - previous.Memory.FragmentationRatio,
		RejectedConnections: current.Clients.RejectedConnections - previous.Clients.RejectedConnections,
		ConnectionsReceived: current.Clients.TotalConnectionsReceived - previous.Clients.TotalConnectionsReceived,
		CommandsProcessed:   current.Traffic.TotalCommandsProcessed - previous.Traffic.TotalCommandsProcessed,
		NetInputBytes:       current.Traffic.NetInputBytes - previous.Traffic.NetInputBytes,
		NetOutputBytes:      current.Traffic.NetOutputBytes - previous.Traffic.NetOutputBytes,
		EvictedKeys:         current.General.EvictedKeys - previous.General.EvictedKeys,
		ExpiredKeys:         current.General.ExpiredKeys - previous.General.ExpiredKeys,
		BlockedClients:      current.Clients.BlockedClients - previous.Clients.BlockedClients,
		PingLatency:         current.PingLatency - previous.PingLatency,
		CPUUser:             current.CPU.UsedCPUUser - previous.CPU.UsedCPUUser,
		CPUSys:              current.CPU.UsedCPUSys - previous.CPU.UsedCPUSys,
		HitRatio:            hitRatio,
	}
}

func warningsFor(current Snapshot, previous *Snapshot, delta *Delta, trend *MemoryTrend) []string {
	var warnings []string

	if delta != nil && delta.UsedMemory > minMeaningfulMemoryGrowth {
		warnings = append(warnings, fmt.Sprintf("used_memory grew by %s since the previous sample", formatBytesSigned(delta.UsedMemory)))
	}
	if delta != nil && delta.UsedMemoryRSS > minMeaningfulMemoryGrowth*2 {
		warnings = append(warnings, fmt.Sprintf("used_memory_rss grew by %s; possible fragmentation or allocator pressure", formatBytesSigned(delta.UsedMemoryRSS)))
	}
	if current.Memory.FragmentationRatio >= highFragmentationRatio {
		warnings = append(warnings, fmt.Sprintf("mem_fragmentation_ratio is high: %.2fx", current.Memory.FragmentationRatio))
	} else if delta != nil && delta.FragmentationRatio >= 0.2 {
		warnings = append(warnings, fmt.Sprintf("mem_fragmentation_ratio worsened by %.2f points", delta.FragmentationRatio))
	}
	if current.Clients.BlockedClients > 0 {
		warnings = append(warnings, fmt.Sprintf("there are %d blocked_clients", current.Clients.BlockedClients))
	}
	if delta != nil && delta.RejectedConnections > 0 {
		warnings = append(warnings, fmt.Sprintf("there were %d new rejected_connections", delta.RejectedConnections))
	}
	if delta != nil && delta.EvictedKeys > 0 {
		warnings = append(warnings, fmt.Sprintf("there were %d new evicted_keys", delta.EvictedKeys))
	}
	if ratio := currentHitRatio(current); ratio > 0 && ratio < 0.80 {
		warnings = append(warnings, fmt.Sprintf("hit ratio is low: %.1f%%", ratio*100))
	}
	if current.PingLatency >= highPingLatency {
		warnings = append(warnings, fmt.Sprintf("ping latency is high: %s", current.PingLatency.Round(time.Millisecond)))
	}
	if current.Persistence.RDBLastBGSaveStatus != "" && current.Persistence.RDBLastBGSaveStatus != "ok" {
		warnings = append(warnings, fmt.Sprintf("latest bgsave reported %q", current.Persistence.RDBLastBGSaveStatus))
	}
	if current.Persistence.AOFEnabled && current.Persistence.AOFLastBGRewriteStatus != "" && current.Persistence.AOFLastBGRewriteStatus != "ok" {
		warnings = append(warnings, fmt.Sprintf("latest bgrewriteaof reported %q", current.Persistence.AOFLastBGRewriteStatus))
	}
	if current.General.LatestForkUsec >= slowForkThresholdUsec {
		warnings = append(warnings, fmt.Sprintf("latest_fork_usec is high: %d us", current.General.LatestForkUsec))
	}
	if trend != nil && trend.SustainedGrowth {
		warnings = append(warnings, fmt.Sprintf("used_memory shows sustained growth: %s over %s (%d samples)", formatBytesSigned(trend.NetUsedMemory), trend.Window.Round(time.Second), trend.Samples))
	}
	if previous == nil && len(warnings) == 0 && current.Memory.MaxMemory > 0 && current.Memory.UsedMemory >= current.Memory.MaxMemory {
		warnings = append(warnings, "used_memory has already reached maxmemory")
	}

	return warnings
}

func currentHitRatio(snapshot Snapshot) float64 {
	total := snapshot.General.KeyspaceHits + snapshot.General.KeyspaceMisses
	if total <= 0 {
		return 0
	}
	return math.Max(0, float64(snapshot.General.KeyspaceHits)/float64(total))
}
