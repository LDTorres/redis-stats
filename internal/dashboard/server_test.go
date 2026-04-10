package dashboard

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/LDTorres/redis-stats/internal/config"
)

func TestTTLScanTimeout(t *testing.T) {
	cfg := config.Config{Timeout: 3 * time.Second, TTLScanSample: 200}
	if got := ttlScanTimeout(cfg); got != 30*time.Second {
		t.Fatalf("ttlScanTimeout() = %s, want %s", got, 30*time.Second)
	}

	cfg.TTLScanSample = 300000
	if got := ttlScanTimeout(cfg); got != 2*time.Minute {
		t.Fatalf("ttlScanTimeout() = %s, want %s", got, 2*time.Minute)
	}
}

func TestDescribeTTLScanError(t *testing.T) {
	cfg := config.Config{Timeout: 3 * time.Second, TTLScanSample: 300000}
	msg := describeTTLScanError(context.DeadlineExceeded, cfg)
	if !strings.Contains(msg, "TTL scan timed out") {
		t.Fatalf("unexpected error message: %q", msg)
	}
	if !strings.Contains(msg, "300000") {
		t.Fatalf("expected sample size in error message: %q", msg)
	}
}
