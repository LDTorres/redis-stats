package redisstats

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBuildTTLProfile(t *testing.T) {
	databases := map[string]KeyspaceDB{
		"db0": {Keys: 50, Expires: 40, AvgTTL: int64((10 * time.Minute) / time.Millisecond)},
		"db1": {Keys: 50, Expires: 45, AvgTTL: int64((5 * time.Minute) / time.Millisecond)},
	}

	profile := buildTTLProfile(100, 85, databases)
	if profile.Label != "short-lived TTL" {
		t.Fatalf("unexpected label: %q", profile.Label)
	}
	if diff := profile.ExpiringRatio - 0.85; diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("unexpected ratio: %f", profile.ExpiringRatio)
	}

	wantTTL := int64(((40 * int64((10*time.Minute)/time.Millisecond)) + (45 * int64((5*time.Minute)/time.Millisecond))) / 85)
	if profile.WeightedAvgTTL != wantTTL {
		t.Fatalf("unexpected weighted avg ttl: %d", profile.WeightedAvgTTL)
	}
}

func TestBuildTTLProfileLabels(t *testing.T) {
	tests := []struct {
		name     string
		total    int64
		expiring int64
		dbs      map[string]KeyspaceDB
		want     string
	}{
		{name: "no keys", total: 0, expiring: 0, dbs: nil, want: "no keys"},
		{name: "mostly persistent", total: 100, expiring: 10, dbs: map[string]KeyspaceDB{"db0": {Expires: 10, AvgTTL: 1000}}, want: "mostly persistent"},
		{name: "mixed", total: 100, expiring: 50, dbs: map[string]KeyspaceDB{"db0": {Expires: 50, AvgTTL: int64((2 * time.Hour) / time.Millisecond)}}, want: "mixed TTL"},
		{name: "mostly expiring", total: 100, expiring: 90, dbs: map[string]KeyspaceDB{"db0": {Expires: 90, AvgTTL: int64((2 * time.Hour) / time.Millisecond)}}, want: "mostly expiring"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildTTLProfile(tt.total, tt.expiring, tt.dbs).Label; got != tt.want {
				t.Fatalf("buildTTLProfile() label = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSummarizePersistentKeyScan(t *testing.T) {
	scan := summarizePersistentKeyScan(time.Unix(100, 0), 2, []ttlSample{
		{Key: "user:1", TTL: -1},
		{Key: "user:2", TTL: -1},
		{Key: "cache/home", TTL: -1},
		{Key: "job|1", TTL: 10 * time.Second},
		{Key: "lonely-key-name-without-separator-and-very-long", TTL: -1},
		{Key: "ignored", Err: errors.New("boom")},
	}, 3)

	if scan.DB != 2 {
		t.Fatalf("unexpected db: %d", scan.DB)
	}
	if scan.SampledKeys != 5 {
		t.Fatalf("unexpected sampled key count: %d", scan.SampledKeys)
	}
	if scan.PersistentSampledKeys != 4 {
		t.Fatalf("unexpected persistent sampled key count: %d", scan.PersistentSampledKeys)
	}
	if len(scan.Groups) != 3 {
		t.Fatalf("unexpected group count: %d", len(scan.Groups))
	}
	if scan.Groups[0].Prefix != "user" || scan.Groups[0].Count != 2 {
		t.Fatalf("unexpected top group: %+v", scan.Groups[0])
	}
	if !strings.HasSuffix(scan.Groups[2].Prefix, "...") {
		t.Fatalf("unexpected fallback prefix: %+v", scan.Groups[2])
	}
}

func TestSummarizePersistentKeyAudit(t *testing.T) {
	audit := summarizePersistentKeyAudit(time.Unix(100, 0), 0, []ttlSample{
		{Key: "bars:1", TTL: -1},
		{Key: "bars:2", TTL: -1},
		{Key: "bars:3", TTL: -1},
		{Key: "session:1", TTL: -1},
		{Key: "session:2", TTL: time.Minute},
	}, 10, 2)

	if audit.ScannedKeys != 5 {
		t.Fatalf("unexpected scanned keys: %d", audit.ScannedKeys)
	}
	if audit.PersistentKeys != 4 {
		t.Fatalf("unexpected persistent keys: %d", audit.PersistentKeys)
	}
	if len(audit.Groups) != 2 {
		t.Fatalf("unexpected group count: %d", len(audit.Groups))
	}
	if audit.Groups[0].Prefix != "bars" || len(audit.Groups[0].ExampleKeys) != 2 {
		t.Fatalf("unexpected top audit group: %+v", audit.Groups[0])
	}
}
