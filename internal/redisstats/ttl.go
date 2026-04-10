package redisstats

import (
	"sort"
	"strings"
	"time"
)

const (
	shortLivedTTLThreshold = 15 * time.Minute
	persistentGroupLimit   = 5
	auditGroupLimit        = 10
	auditExampleLimit      = 3
)

type ttlSample struct {
	Key string
	TTL time.Duration
	Err error
}

func buildTTLProfile(totalKeys, expiringKeys int64, databases map[string]KeyspaceDB) TTLProfile {
	if totalKeys <= 0 {
		return TTLProfile{Label: "no keys"}
	}

	var weightedTTL int64
	for _, db := range databases {
		if db.Expires <= 0 || db.AvgTTL <= 0 {
			continue
		}
		weightedTTL += db.AvgTTL * db.Expires
	}

	profile := TTLProfile{
		ExpiringRatio: float64(expiringKeys) / float64(totalKeys),
		Label:         "mostly persistent",
	}
	if expiringKeys > 0 {
		profile.WeightedAvgTTL = weightedTTL / expiringKeys
	}

	switch {
	case profile.ExpiringRatio >= 0.8 && profile.WeightedAvgTTL > 0 && profile.WeightedAvgTTL <= int64(shortLivedTTLThreshold/time.Millisecond):
		profile.Label = "short-lived TTL"
	case profile.ExpiringRatio >= 0.8:
		profile.Label = "mostly expiring"
	case profile.ExpiringRatio >= 0.2:
		profile.Label = "mixed TTL"
	}

	return profile
}

func summarizePersistentKeyScan(capturedAt time.Time, db int, samples []ttlSample, groupLimit int) PersistentKeyScan {
	if groupLimit <= 0 {
		groupLimit = persistentGroupLimit
	}

	scan := PersistentKeyScan{
		CapturedAt: capturedAt,
		DB:         db,
	}

	type aggregate struct {
		count    int
		examples []string
	}

	counts := make(map[string]*aggregate)
	for _, sample := range samples {
		if sample.Err != nil {
			continue
		}

		scan.SampledKeys++
		if sample.TTL != -1 {
			continue
		}

		scan.PersistentSampledKeys++
		prefix := groupKeyPrefix(sample.Key)
		group := counts[prefix]
		if group == nil {
			group = &aggregate{}
			counts[prefix] = group
		}
		group.count++
		if len(group.examples) < auditExampleLimit {
			group.examples = append(group.examples, sample.Key)
		}
	}

	if scan.SampledKeys > 0 {
		scan.PersistentRatioInSample = float64(scan.PersistentSampledKeys) / float64(scan.SampledKeys)
	}

	groups := make([]PersistentKeyGroup, 0, len(counts))
	for prefix, aggregate := range counts {
		group := PersistentKeyGroup{
			Prefix:      prefix,
			Count:       aggregate.count,
			ExampleKeys: append([]string(nil), aggregate.examples...),
		}
		if scan.PersistentSampledKeys > 0 {
			group.Share = float64(aggregate.count) / float64(scan.PersistentSampledKeys)
		}
		groups = append(groups, group)
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Count == groups[j].Count {
			return groups[i].Prefix < groups[j].Prefix
		}
		return groups[i].Count > groups[j].Count
	})

	if len(groups) > groupLimit {
		groups = groups[:groupLimit]
	}
	scan.Groups = groups
	return scan
}

func groupKeyPrefix(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "(empty)"
	}

	for _, sep := range []string{":", "|", "/"} {
		if idx := strings.Index(trimmed, sep); idx > 0 {
			return trimmed[:idx]
		}
	}

	if len(trimmed) > 32 {
		return trimmed[:32] + "..."
	}

	return trimmed
}

func summarizePersistentKeyAudit(capturedAt time.Time, db int, samples []ttlSample, groupLimit, exampleLimit int) PersistentKeyAudit {
	if groupLimit <= 0 {
		groupLimit = auditGroupLimit
	}
	if exampleLimit <= 0 {
		exampleLimit = auditExampleLimit
	}

	audit := PersistentKeyAudit{
		CapturedAt: capturedAt,
		DB:         db,
	}

	type aggregate struct {
		count    int
		examples []string
	}

	counts := make(map[string]*aggregate)
	for _, sample := range samples {
		if sample.Err != nil {
			continue
		}

		audit.ScannedKeys++
		if sample.TTL != -1 {
			continue
		}

		audit.PersistentKeys++
		prefix := groupKeyPrefix(sample.Key)
		group := counts[prefix]
		if group == nil {
			group = &aggregate{}
			counts[prefix] = group
		}
		group.count++
		if len(group.examples) < exampleLimit {
			group.examples = append(group.examples, sample.Key)
		}
	}

	if audit.ScannedKeys > 0 {
		audit.PersistentRatio = float64(audit.PersistentKeys) / float64(audit.ScannedKeys)
	}

	groups := make([]PersistentKeyAuditGroup, 0, len(counts))
	for prefix, aggregate := range counts {
		group := PersistentKeyAuditGroup{
			Prefix:      prefix,
			Count:       aggregate.count,
			ExampleKeys: append([]string(nil), aggregate.examples...),
		}
		if audit.PersistentKeys > 0 {
			group.Share = float64(group.Count) / float64(audit.PersistentKeys)
		}
		groups = append(groups, group)
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Count == groups[j].Count {
			return groups[i].Prefix < groups[j].Prefix
		}
		return groups[i].Count > groups[j].Count
	})
	if len(groups) > groupLimit {
		groups = groups[:groupLimit]
	}
	audit.Groups = groups

	return audit
}
