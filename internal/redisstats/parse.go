package redisstats

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

type parsedInfo struct {
	Values   map[string]string
	Keyspace map[string]KeyspaceDB
}

func parseInfo(raw string) parsedInfo {
	result := parsedInfo{
		Values:   make(map[string]string),
		Keyspace: make(map[string]KeyspaceDB),
	}

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if isKeyspaceDBKey(key) {
			result.Keyspace[key] = parseKeyspaceDB(value)
			continue
		}

		result.Values[key] = value
	}

	return result
}

func parseKeyspaceDB(raw string) KeyspaceDB {
	var db KeyspaceDB
	for _, field := range strings.Split(raw, ",") {
		pair := strings.SplitN(strings.TrimSpace(field), "=", 2)
		if len(pair) != 2 {
			continue
		}

		switch pair[0] {
		case "keys":
			db.Keys = mustInt(pair[1])
		case "expires":
			db.Expires = mustInt(pair[1])
		case "avg_ttl":
			db.AvgTTL = mustInt(pair[1])
		}
	}
	return db
}

func isKeyspaceDBKey(key string) bool {
	if len(key) < 3 || !strings.HasPrefix(key, "db") {
		return false
	}

	for _, r := range key[2:] {
		if !unicode.IsDigit(r) {
			return false
		}
	}

	return true
}

func snapshotFromInfo(info parsedInfo, dbsize int64, pingLatency time.Duration, capturedAt time.Time) Snapshot {
	keyspace := KeyspaceStats{
		DBSize:    dbsize,
		Databases: info.Keyspace,
	}
	for _, db := range info.Keyspace {
		keyspace.TotalKeys += db.Keys
		keyspace.ExpiringKeys += db.Expires
	}
	keyspace.TTLProfile = buildTTLProfile(keyspace.TotalKeys, keyspace.ExpiringKeys, keyspace.Databases)

	rdbLastSaveTime := time.Unix(mustInt(info.Values["rdb_last_save_time"]), 0)
	if rdbLastSaveTime.Unix() <= 0 {
		rdbLastSaveTime = time.Time{}
	}

	return Snapshot{
		CapturedAt:  capturedAt,
		PingLatency: pingLatency,
		Memory: MemoryStats{
			UsedMemory:         mustUint(info.Values["used_memory"]),
			UsedMemoryRSS:      mustUint(info.Values["used_memory_rss"]),
			UsedMemoryPeak:     mustUint(info.Values["used_memory_peak"]),
			UsedMemoryDataset:  mustUint(info.Values["used_memory_dataset"]),
			MaxMemory:          mustUint(info.Values["maxmemory"]),
			FragmentationRatio: mustFloat(info.Values["mem_fragmentation_ratio"]),
			MaxMemoryPolicy:    info.Values["maxmemory_policy"],
		},
		Clients: ClientStats{
			ConnectedClients:         mustInt(info.Values["connected_clients"]),
			BlockedClients:           mustInt(info.Values["blocked_clients"]),
			RejectedConnections:      mustInt(info.Values["rejected_connections"]),
			TotalConnectionsReceived: mustInt(info.Values["total_connections_received"]),
		},
		Traffic: TrafficStats{
			TotalCommandsProcessed: mustInt(info.Values["total_commands_processed"]),
			InstantaneousOpsPerSec: mustInt(info.Values["instantaneous_ops_per_sec"]),
			NetInputBytes:          mustInt(info.Values["total_net_input_bytes"]),
			NetOutputBytes:         mustInt(info.Values["total_net_output_bytes"]),
		},
		CPU: CPUStats{
			UsedCPUSys:          mustFloat(info.Values["used_cpu_sys"]),
			UsedCPUUser:         mustFloat(info.Values["used_cpu_user"]),
			UsedCPUSysChildren:  mustFloat(info.Values["used_cpu_sys_children"]),
			UsedCPUUserChildren: mustFloat(info.Values["used_cpu_user_children"]),
		},
		Persistence: PersistenceStats{
			RDBLastBGSaveStatus:    info.Values["rdb_last_bgsave_status"],
			RDBLastSaveTime:        rdbLastSaveTime,
			AOFEnabled:             mustBool(info.Values["aof_enabled"]),
			AOFLastBGRewriteStatus: info.Values["aof_last_bgrewrite_status"],
			Role:                   info.Values["role"],
			ConnectedReplicas:      firstNonZero(mustInt(info.Values["connected_replicas"]), mustInt(info.Values["connected_slaves"])),
		},
		General: GeneralStats{
			UptimeSeconds:  mustInt(info.Values["uptime_in_seconds"]),
			EvictedKeys:    mustInt(info.Values["evicted_keys"]),
			ExpiredKeys:    mustInt(info.Values["expired_keys"]),
			KeyspaceHits:   mustInt(info.Values["keyspace_hits"]),
			KeyspaceMisses: mustInt(info.Values["keyspace_misses"]),
			LatestForkUsec: mustInt(info.Values["latest_fork_usec"]),
		},
		Keyspace: keyspace,
	}
}

func mustInt(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func mustUint(value string) uint64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func mustFloat(value string) float64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func mustBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "yes", "true":
		return true
	default:
		return false
	}
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
