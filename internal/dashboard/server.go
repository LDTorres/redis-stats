package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/LDTorres/redis-stats/internal/config"
	"github.com/LDTorres/redis-stats/internal/redisstats"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

//go:embed static/*
var staticFiles embed.FS

type ttlScanResponse struct {
	Status         string                 `json:"status"`
	PersistentScan *persistentScanPayload `json:"persistent_scan,omitempty"`
	Error          string                 `json:"error,omitempty"`
}

type Server struct {
	cfg       config.Config
	collector *redisstats.Collector
	upgrader  websocket.Upgrader

	mu             sync.RWMutex
	clients        map[*websocket.Conn]struct{}
	stateMu        sync.RWMutex
	history        []redisstats.Snapshot
	persistentScan *redisstats.PersistentKeyScan
}

func New(cfg config.Config, client *redis.Client) *Server {
	return &Server{
		cfg:       cfg,
		collector: redisstats.NewCollector(client),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		clients: make(map[*websocket.Conn]struct{}),
	}
}

func (s *Server) Run(ctx context.Context) error {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("load embedded static files: %w", err)
	}

	state, err := loadState(s.cfg.StateFile, s.cfg.HistorySize)
	if err != nil {
		return err
	}
	s.history = state.History
	s.persistentScan = state.PersistentScan

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/api/ttl-scan", s.handleTTLScan)

	server := &http.Server{
		Addr:    s.cfg.Listen,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.streamLoop(ctx)
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		s.closeClients()
	}()

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	return <-errCh
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.mu.Lock()
	s.clients[conn] = struct{}{}
	s.mu.Unlock()

	go func() {
		defer s.removeClient(conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (s *Server) streamLoop(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	var previous *redisstats.Snapshot
	s.stateMu.RLock()
	history := append([]redisstats.Snapshot(nil), s.history...)
	s.stateMu.RUnlock()
	if len(history) > 0 {
		last := history[len(history)-1]
		previous = &last
	}

	send := func() error {
		collectCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeout*2)
		snapshot, err := s.collector.Collect(collectCtx)
		cancel()
		if err != nil {
			s.broadcast(streamMessage{Status: "error", Error: err.Error()})
			return nil
		}

		history = appendHistory(history, snapshot, s.cfg.HistorySize)
		s.stateMu.Lock()
		s.history = append([]redisstats.Snapshot(nil), history...)
		s.stateMu.Unlock()
		if err := s.persistState(); err != nil {
			s.broadcast(streamMessage{Status: "error", Error: err.Error()})
		}
		report := redisstats.BuildReport(snapshot, previous, history, s.cfg.TrendMinSamples)
		previous = &snapshot
		s.broadcast(newStreamMessage(report, history, s.getPersistentScan(), s.cfg.ConnectionScope(), s.cfg.StateFile))
		return nil
	}

	if err := send(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := send(); err != nil {
				return err
			}
		}
	}
}

func (s *Server) handleTTLScan(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.writeJSON(w, ttlScanResponse{
			Status:         "ok",
			PersistentScan: toPersistentScanPayload(s.getPersistentScan(), true),
		})
	case http.MethodPost:
		ctx, cancel := context.WithTimeout(r.Context(), ttlScanTimeout(s.cfg))
		defer cancel()

		scan, err := s.collector.CollectPersistentKeyScan(ctx, s.cfg.TTLScanSample)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			s.writeJSON(w, ttlScanResponse{Status: "error", Error: describeTTLScanError(err, s.cfg)})
			return
		}

		s.stateMu.Lock()
		s.persistentScan = &scan
		s.stateMu.Unlock()
		if err := s.persistState(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.writeJSON(w, ttlScanResponse{Status: "error", Error: err.Error()})
			return
		}

		s.writeJSON(w, ttlScanResponse{
			Status:         "ok",
			PersistentScan: toPersistentScanPayload(&scan, false),
		})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func ttlScanTimeout(cfg config.Config) time.Duration {
	timeout := cfg.Timeout * 4
	if timeout < 30*time.Second {
		timeout = 30 * time.Second
	}

	if cfg.TTLScanSample > 5000 {
		timeout += time.Duration(cfg.TTLScanSample/5000) * 2 * time.Second
	}
	if timeout > 2*time.Minute {
		timeout = 2 * time.Minute
	}

	return timeout
}

func describeTTLScanError(err error, cfg config.Config) string {
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		return fmt.Sprintf(
			"TTL scan timed out after %s while inspecting up to %d keys. Try a smaller --ttl-scan-sample-size or use ttl-audit for a full offline-style pass.",
			ttlScanTimeout(cfg).Round(time.Second),
			cfg.TTLScanSample,
		)
	}
	return err.Error()
}

func (s *Server) broadcast(payload streamMessage) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	s.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(s.clients))
	for conn := range s.clients {
		clients = append(clients, conn)
	}
	s.mu.RUnlock()

	for _, conn := range clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			s.removeClient(conn)
		}
	}
}

func (s *Server) getPersistentScan() *redisstats.PersistentKeyScan {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	if s.persistentScan == nil {
		return nil
	}
	scan := *s.persistentScan
	scan.Groups = append([]redisstats.PersistentKeyGroup(nil), s.persistentScan.Groups...)
	return &scan
}

func (s *Server) persistState() error {
	s.stateMu.RLock()
	state := persistedState{
		History:        append([]redisstats.Snapshot(nil), s.history...),
		PersistentScan: s.getPersistentScanLocked(),
	}
	s.stateMu.RUnlock()
	return saveState(s.cfg.StateFile, state)
}

func (s *Server) getPersistentScanLocked() *redisstats.PersistentKeyScan {
	if s.persistentScan == nil {
		return nil
	}
	scan := *s.persistentScan
	scan.Groups = append([]redisstats.PersistentKeyGroup(nil), s.persistentScan.Groups...)
	return &scan
}

func (s *Server) writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) removeClient(conn *websocket.Conn) {
	s.mu.Lock()
	delete(s.clients, conn)
	s.mu.Unlock()
	_ = conn.Close()
}

func (s *Server) closeClients() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.clients {
		_ = conn.Close()
		delete(s.clients, conn)
	}
}

func appendHistory(history []redisstats.Snapshot, snapshot redisstats.Snapshot, limit int) []redisstats.Snapshot {
	if limit <= 0 {
		limit = 1
	}
	history = append(history, snapshot)
	if len(history) <= limit {
		return history
	}
	return append([]redisstats.Snapshot(nil), history[len(history)-limit:]...)
}
