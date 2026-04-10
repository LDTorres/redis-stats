package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LDTorres/redis-stats/internal/redisstats"
)

type persistedState struct {
	History        []redisstats.Snapshot         `json:"history"`
	PersistentScan *redisstats.PersistentKeyScan `json:"persistent_scan,omitempty"`
}

func loadState(path string, limit int) (persistedState, error) {
	if path == "" {
		return persistedState{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return persistedState{}, nil
		}
		return persistedState{}, fmt.Errorf("read state file: %w", err)
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return persistedState{}, fmt.Errorf("decode state file: %w", err)
	}

	if limit > 0 && len(state.History) > limit {
		state.History = append([]redisstats.Snapshot(nil), state.History[len(state.History)-limit:]...)
	}

	return state, nil
}

func saveState(path string, state persistedState) error {
	if path == "" {
		return nil
	}

	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create state directory: %w", err)
		}
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state file: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}

	return nil
}

func loadHistory(path string, limit int) ([]redisstats.Snapshot, error) {
	state, err := loadState(path, limit)
	if err != nil {
		return nil, err
	}
	return state.History, nil
}

func saveHistory(path string, history []redisstats.Snapshot) error {
	return saveState(path, persistedState{History: history})
}
