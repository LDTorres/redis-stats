package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseArgsPrefersFlagsOverEnv(t *testing.T) {
	t.Setenv("REDIS_ADDR", "env-host:6379")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("REDIS_TLS", "true")
	t.Setenv("REDIS_INTERVAL", "15s")
	t.Setenv("REDIS_TIMEOUT", "4s")
	t.Setenv("REDIS_STATS_LISTEN", "127.0.0.1:9999")
	t.Setenv("REDIS_STATS_HISTORY_SIZE", "33")
	t.Setenv("REDIS_STATS_TREND_MIN_SAMPLES", "7")
	t.Setenv("REDIS_STATS_STATE_FILE", "/tmp/redis-stats.json")
	t.Setenv("REDIS_STATS_TTL_SCAN_SAMPLE_SIZE", "150")

	cfg, err := ParseArgs([]string{"snapshot", "--addr", "flag-host:6380", "--db", "4", "--tls=false", "--interval", "2s"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if cfg.Command != CommandSnapshot {
		t.Fatalf("expected snapshot command, got %q", cfg.Command)
	}
	if cfg.Addr != "flag-host:6380" {
		t.Fatalf("expected addr from flags, got %q", cfg.Addr)
	}
	if cfg.DB != 4 {
		t.Fatalf("expected db 4, got %d", cfg.DB)
	}
	if cfg.TLS {
		t.Fatalf("expected tls=false from flags")
	}
	if cfg.Interval != 2*time.Second {
		t.Fatalf("expected interval 2s, got %s", cfg.Interval)
	}
	if cfg.Timeout != 4*time.Second {
		t.Fatalf("expected timeout from env, got %s", cfg.Timeout)
	}
	if cfg.Listen != "127.0.0.1:9999" {
		t.Fatalf("expected listen from env, got %q", cfg.Listen)
	}
	if cfg.HistorySize != 33 {
		t.Fatalf("expected history size from env, got %d", cfg.HistorySize)
	}
	if cfg.TrendMinSamples != 7 {
		t.Fatalf("expected trend min samples from env, got %d", cfg.TrendMinSamples)
	}
	if cfg.StateFile != "/tmp/redis-stats.json" {
		t.Fatalf("expected state file from env, got %q", cfg.StateFile)
	}
	if cfg.TTLScanSample != 150 {
		t.Fatalf("expected ttl scan sample from env, got %d", cfg.TTLScanSample)
	}
}

func TestParseArgsDefaultsToWatch(t *testing.T) {
	cfg, err := ParseArgs(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if cfg.Command != CommandWatch {
		t.Fatalf("expected watch default, got %q", cfg.Command)
	}
	if cfg.Addr != DefaultAddr {
		t.Fatalf("expected default addr, got %q", cfg.Addr)
	}
	if cfg.Listen != DefaultListen {
		t.Fatalf("expected default listen, got %q", cfg.Listen)
	}
	if cfg.HistorySize != DefaultHistorySize {
		t.Fatalf("expected default history size, got %d", cfg.HistorySize)
	}
	if cfg.TrendMinSamples != DefaultTrendMinSamples {
		t.Fatalf("expected default trend min samples, got %d", cfg.TrendMinSamples)
	}
	if cfg.StateFile != DefaultStateFile {
		t.Fatalf("expected default state file, got %q", cfg.StateFile)
	}
	if cfg.TTLScanSample != DefaultTTLScanSample {
		t.Fatalf("expected default ttl scan sample, got %d", cfg.TTLScanSample)
	}
}

func TestParseArgsTTLAuditCommand(t *testing.T) {
	cfg, err := ParseArgs([]string{"ttl-audit"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if cfg.Command != CommandTTLAudit {
		t.Fatalf("expected ttl-audit command, got %q", cfg.Command)
	}
}

func TestParseArgsLoadsDotEnv(t *testing.T) {
	tempDir := t.TempDir()
	writeFile(t, filepath.Join(tempDir, ".env"), "REDIS_URL=redis://from-dotenv:6379/0\nREDIS_TIMEOUT=6s\n")

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		_ = os.Chdir(previousWD)
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	cfg, err := ParseArgs(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if cfg.RedisURL != "redis://from-dotenv:6379/0" {
		t.Fatalf("expected redis url from .env, got %q", cfg.RedisURL)
	}
	if cfg.Timeout != 6*time.Second {
		t.Fatalf("expected timeout from .env, got %s", cfg.Timeout)
	}
}

func TestParseArgsPrefersEnvOverDotEnv(t *testing.T) {
	tempDir := t.TempDir()
	writeFile(t, filepath.Join(tempDir, ".env"), "REDIS_URL=redis://from-dotenv:6379/0\n")
	t.Setenv("REDIS_URL", "redis://from-env:6379/1")

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		_ = os.Chdir(previousWD)
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	cfg, err := ParseArgs(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	if cfg.RedisURL != "redis://from-env:6379/1" {
		t.Fatalf("expected env to override .env, got %q", cfg.RedisURL)
	}
}

func TestLoadEnvFailsOnInvalidDuration(t *testing.T) {
	t.Setenv("REDIS_TIMEOUT", "invalid")
	_, err := loadEnv()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRedisOptionsUsesURLThenOverrides(t *testing.T) {
	cfg := Config{
		RedisURL:   "redis://user:pass@localhost:6379/1",
		Addr:       "other-host:6380",
		DB:         9,
		TLS:        true,
		Timeout:    7 * time.Second,
		hasAddr:    true,
		hasDB:      true,
		hasTLS:     true,
		hasTimeout: true,
	}

	opts, err := cfg.RedisOptions()
	if err != nil {
		t.Fatalf("RedisOptions() error = %v", err)
	}

	if opts.Addr != "other-host:6380" {
		t.Fatalf("expected addr override, got %q", opts.Addr)
	}
	if opts.DB != 9 {
		t.Fatalf("expected db override, got %d", opts.DB)
	}
	if opts.TLSConfig == nil {
		t.Fatal("expected TLSConfig")
	}
}

func TestConnectionScope(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "localhost", cfg: Config{Addr: "localhost:6379"}, want: "local"},
		{name: "private ip", cfg: Config{Addr: "10.0.0.5:6379"}, want: "private"},
		{name: "remote host", cfg: Config{RedisURL: "redis://cache.example.com:6379/0"}, want: "remote"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.ConnectionScope(); got != tt.want {
				t.Fatalf("ConnectionScope() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
