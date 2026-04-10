package redisstats

import (
	"strings"
	"testing"
	"time"
)

func TestComputeDelta(t *testing.T) {
	prev := Snapshot{
		CapturedAt: time.Unix(100, 0),
		Memory: MemoryStats{
			UsedMemory:         10 << 20,
			UsedMemoryRSS:      12 << 20,
			FragmentationRatio: 1.1,
		},
		Clients: ClientStats{
			RejectedConnections:      3,
			TotalConnectionsReceived: 10,
		},
		Traffic: TrafficStats{
			TotalCommandsProcessed: 100,
			NetInputBytes:          1000,
			NetOutputBytes:         2000,
		},
		CPU: CPUStats{
			UsedCPUUser: 1,
			UsedCPUSys:  2,
		},
		General: GeneralStats{
			EvictedKeys:    1,
			ExpiredKeys:    5,
			KeyspaceHits:   80,
			KeyspaceMisses: 20,
		},
		PingLatency: 5 * time.Millisecond,
	}
	curr := prev
	curr.CapturedAt = time.Unix(105, 0)
	curr.Memory.UsedMemory += 3 << 20
	curr.Memory.UsedMemoryRSS += 5 << 20
	curr.Memory.FragmentationRatio = 1.4
	curr.Clients.RejectedConnections += 2
	curr.Clients.TotalConnectionsReceived += 7
	curr.Traffic.TotalCommandsProcessed += 200
	curr.Traffic.NetInputBytes += 400
	curr.Traffic.NetOutputBytes += 900
	curr.CPU.UsedCPUUser += 0.5
	curr.CPU.UsedCPUSys += 0.25
	curr.General.EvictedKeys += 4
	curr.General.ExpiredKeys += 2
	curr.PingLatency = 15 * time.Millisecond

	delta := ComputeDelta(prev, curr)

	if delta.UsedMemory != 3<<20 {
		t.Fatalf("unexpected memory delta: %d", delta.UsedMemory)
	}
	if delta.CommandsProcessed != 200 {
		t.Fatalf("unexpected commands delta: %d", delta.CommandsProcessed)
	}
	if delta.Elapsed != 5*time.Second {
		t.Fatalf("unexpected elapsed: %s", delta.Elapsed)
	}
}

func TestWarningsForCommonProblems(t *testing.T) {
	prev := Snapshot{
		CapturedAt: time.Unix(100, 0),
		Memory: MemoryStats{
			UsedMemory:         10 << 20,
			UsedMemoryRSS:      12 << 20,
			FragmentationRatio: 1.1,
		},
		Clients: ClientStats{
			RejectedConnections: 1,
		},
		General: GeneralStats{
			EvictedKeys:    1,
			KeyspaceHits:   10,
			KeyspaceMisses: 10,
		},
		PingLatency: 10 * time.Millisecond,
	}
	curr := prev
	curr.CapturedAt = time.Unix(105, 0)
	curr.Memory.UsedMemory += 4 << 20
	curr.Memory.UsedMemoryRSS += 6 << 20
	curr.Memory.FragmentationRatio = 1.7
	curr.Clients.BlockedClients = 2
	curr.Clients.RejectedConnections += 3
	curr.General.EvictedKeys += 2
	curr.General.KeyspaceHits = 10
	curr.General.KeyspaceMisses = 30
	curr.General.LatestForkUsec = 700000
	curr.PingLatency = 80 * time.Millisecond

	report := BuildReport(curr, &prev, nil, 12)
	joined := strings.Join(report.Warnings, "\n")

	for _, expected := range []string{
		"used_memory grew",
		"used_memory_rss grew",
		"mem_fragmentation_ratio is high",
		"blocked_clients",
		"rejected_connections",
		"evicted_keys",
		"hit ratio is low",
		"ping latency is high",
		"latest_fork_usec is high",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected warning containing %q, got:\n%s", expected, joined)
		}
	}
}

func TestAnalyzeMemoryTrendDetectsSustainedGrowth(t *testing.T) {
	base := Snapshot{
		CapturedAt: time.Unix(100, 0),
		Memory: MemoryStats{
			UsedMemory:    10 << 20,
			UsedMemoryRSS: 12 << 20,
		},
	}

	history := make([]Snapshot, 0, 5)
	for i := range 5 {
		snapshot := base
		snapshot.CapturedAt = base.CapturedAt.Add(time.Duration(i) * 5 * time.Second)
		snapshot.Memory.UsedMemory += uint64(i) * (256 << 10)
		snapshot.Memory.UsedMemoryRSS += uint64(i) * (128 << 10)
		history = append(history, snapshot)
	}

	trend := AnalyzeMemoryTrend(history, 4)
	if trend == nil {
		t.Fatal("expected trend")
	}
	if !trend.SustainedGrowth {
		t.Fatalf("expected sustained growth, got %+v", *trend)
	}
	if trend.NetUsedMemory <= 0 {
		t.Fatalf("expected positive net growth, got %d", trend.NetUsedMemory)
	}

	report := BuildReport(history[len(history)-1], &history[len(history)-2], history, 4)
	joined := strings.Join(report.Warnings, "\n")
	if !strings.Contains(joined, "sustained growth") {
		t.Fatalf("expected sustained trend warning, got:\n%s", joined)
	}
}

func TestAnalyzeMemoryTrendIgnoresNoise(t *testing.T) {
	base := Snapshot{
		CapturedAt: time.Unix(100, 0),
		Memory: MemoryStats{
			UsedMemory: 10 << 20,
		},
	}

	history := []Snapshot{
		base,
		{CapturedAt: base.CapturedAt.Add(5 * time.Second), Memory: MemoryStats{UsedMemory: (10 << 20) + (8 << 10)}},
		{CapturedAt: base.CapturedAt.Add(10 * time.Second), Memory: MemoryStats{UsedMemory: (10 << 20) - (4 << 10)}},
		{CapturedAt: base.CapturedAt.Add(15 * time.Second), Memory: MemoryStats{UsedMemory: (10 << 20) + (12 << 10)}},
	}

	trend := AnalyzeMemoryTrend(history, 4)
	if trend == nil {
		t.Fatal("expected trend")
	}
	if trend.SustainedGrowth {
		t.Fatalf("expected noise, got %+v", *trend)
	}
}
