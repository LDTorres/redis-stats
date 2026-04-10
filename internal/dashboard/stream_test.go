package dashboard

import (
	"testing"
	"time"

	"github.com/LDTorres/redis-stats/internal/redisstats"
)

func TestToSnapshotPayloadIncludesTTLProfile(t *testing.T) {
	payload := toSnapshotPayload(redisstats.Snapshot{
		CapturedAt: time.Unix(100, 0),
		Keyspace: redisstats.KeyspaceStats{
			DBSize:       10,
			TotalKeys:    10,
			ExpiringKeys: 4,
			TTLProfile: redisstats.TTLProfile{
				ExpiringRatio:  0.4,
				WeightedAvgTTL: int64((5 * time.Minute) / time.Millisecond),
				Label:          "mixed TTL",
			},
		},
	})

	if diff := payload.TTLPercent - 0.4; diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("unexpected ttl percent: %f", payload.TTLPercent)
	}
	if payload.WeightedAvgTTL != int64((5*time.Minute)/time.Millisecond) {
		t.Fatalf("unexpected weighted avg ttl: %d", payload.WeightedAvgTTL)
	}
	if payload.TTLLabel != "mixed TTL" {
		t.Fatalf("unexpected ttl label: %q", payload.TTLLabel)
	}
}

func TestToPersistentScanPayload(t *testing.T) {
	payload := toPersistentScanPayload(&redisstats.PersistentKeyScan{
		CapturedAt:              time.Unix(100, 0),
		DB:                      0,
		SampledKeys:             20,
		PersistentSampledKeys:   5,
		PersistentRatioInSample: 0.25,
		Groups: []redisstats.PersistentKeyGroup{
			{Prefix: "user", Count: 3, Share: 0.6},
		},
	}, true)

	if !payload.Cached {
		t.Fatal("expected cached payload")
	}
	if payload.PersistentSampledKeys != 5 {
		t.Fatalf("unexpected persistent sampled keys: %d", payload.PersistentSampledKeys)
	}
	if len(payload.Groups) != 1 || payload.Groups[0].Prefix != "user" {
		t.Fatalf("unexpected groups: %+v", payload.Groups)
	}
}
