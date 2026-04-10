package dashboard

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/LDTorres/redis-stats/internal/redisstats"
)

func TestSaveAndLoadHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	history := []redisstats.Snapshot{
		{CapturedAt: time.Unix(100, 0), Memory: redisstats.MemoryStats{UsedMemory: 10}},
		{CapturedAt: time.Unix(105, 0), Memory: redisstats.MemoryStats{UsedMemory: 20}},
		{CapturedAt: time.Unix(110, 0), Memory: redisstats.MemoryStats{UsedMemory: 30}},
	}

	if err := saveHistory(path, history); err != nil {
		t.Fatalf("saveHistory() error = %v", err)
	}

	loaded, err := loadHistory(path, 2)
	if err != nil {
		t.Fatalf("loadHistory() error = %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(loaded))
	}
	if loaded[0].Memory.UsedMemory != 20 || loaded[1].Memory.UsedMemory != 30 {
		t.Fatalf("unexpected loaded history: %+v", loaded)
	}
}

func TestSaveAndLoadStateWithPersistentScan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	state := persistedState{
		History: []redisstats.Snapshot{
			{CapturedAt: time.Unix(100, 0), Memory: redisstats.MemoryStats{UsedMemory: 10}},
		},
		PersistentScan: &redisstats.PersistentKeyScan{
			CapturedAt:              time.Unix(120, 0),
			DB:                      0,
			SampledKeys:             12,
			PersistentSampledKeys:   4,
			PersistentRatioInSample: 0.3333,
			Groups: []redisstats.PersistentKeyGroup{
				{Prefix: "user", Count: 3, Share: 0.75},
			},
		},
	}

	if err := saveState(path, state); err != nil {
		t.Fatalf("saveState() error = %v", err)
	}

	loaded, err := loadState(path, 10)
	if err != nil {
		t.Fatalf("loadState() error = %v", err)
	}

	if loaded.PersistentScan == nil {
		t.Fatal("expected persistent scan to be loaded")
	}
	if loaded.PersistentScan.SampledKeys != 12 {
		t.Fatalf("unexpected sampled keys: %d", loaded.PersistentScan.SampledKeys)
	}
	if len(loaded.PersistentScan.Groups) != 1 || loaded.PersistentScan.Groups[0].Prefix != "user" {
		t.Fatalf("unexpected groups: %+v", loaded.PersistentScan.Groups)
	}
}
