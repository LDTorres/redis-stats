package dashboard

import (
	"sort"
	"time"

	"github.com/LDTorres/redis-stats/internal/redisstats"
)

type streamMessage struct {
	Status      string                 `json:"status"`
	GeneratedAt string                 `json:"generated_at,omitempty"`
	Connection  connectionMeta         `json:"connection"`
	Snapshot    *snapshotPayload       `json:"snapshot,omitempty"`
	Delta       *deltaPayload          `json:"delta,omitempty"`
	MemoryTrend *trendPayload          `json:"memory_trend,omitempty"`
	Series      *seriesPayload         `json:"series,omitempty"`
	Persistent  *persistentScanPayload `json:"persistent_scan,omitempty"`
	Alerts      []alertView            `json:"alerts,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

type connectionMeta struct {
	Scope     string `json:"scope"`
	StateFile string `json:"state_file,omitempty"`
}

type snapshotPayload struct {
	CapturedAt          string         `json:"captured_at"`
	PingLatencyMS       int64          `json:"ping_latency_ms"`
	UsedMemory          uint64         `json:"used_memory"`
	UsedMemoryRSS       uint64         `json:"used_memory_rss"`
	UsedMemoryPeak      uint64         `json:"used_memory_peak"`
	UsedMemoryDataset   uint64         `json:"used_memory_dataset"`
	MemFragmentation    float64        `json:"mem_fragmentation_ratio"`
	MaxMemory           uint64         `json:"maxmemory"`
	MaxMemoryPolicy     string         `json:"maxmemory_policy"`
	ConnectedClients    int64          `json:"connected_clients"`
	BlockedClients      int64          `json:"blocked_clients"`
	RejectedConnections int64          `json:"rejected_connections"`
	ConnectionsReceived int64          `json:"total_connections_received"`
	InstantOpsPerSec    int64          `json:"instantaneous_ops_per_sec"`
	CommandsProcessed   int64          `json:"total_commands_processed"`
	NetInputBytes       int64          `json:"total_net_input_bytes"`
	NetOutputBytes      int64          `json:"total_net_output_bytes"`
	UsedCPUUser         float64        `json:"used_cpu_user"`
	UsedCPUSys          float64        `json:"used_cpu_sys"`
	Role                string         `json:"role"`
	ConnectedReplicas   int64          `json:"connected_replicas"`
	AOFEnabled          bool           `json:"aof_enabled"`
	RDBLastBGSaveStatus string         `json:"rdb_last_bgsave_status"`
	RDBLastSaveTime     string         `json:"rdb_last_save_time,omitempty"`
	UptimeSeconds       int64          `json:"uptime_seconds"`
	EvictedKeys         int64          `json:"evicted_keys"`
	ExpiredKeys         int64          `json:"expired_keys"`
	KeyspaceHits        int64          `json:"keyspace_hits"`
	KeyspaceMisses      int64          `json:"keyspace_misses"`
	HitRatio            float64        `json:"hit_ratio"`
	LatestForkUsec      int64          `json:"latest_fork_usec"`
	DBSize              int64          `json:"dbsize"`
	TotalKeys           int64          `json:"total_keys"`
	ExpiringKeys        int64          `json:"expiring_keys"`
	TTLPercent          float64        `json:"ttl_percent"`
	WeightedAvgTTL      int64          `json:"weighted_avg_ttl"`
	TTLLabel            string         `json:"ttl_label"`
	Databases           []databaseInfo `json:"databases"`
}

type databaseInfo struct {
	Name    string `json:"name"`
	Keys    int64  `json:"keys"`
	Expires int64  `json:"expires"`
	AvgTTL  int64  `json:"avg_ttl"`
}

type deltaPayload struct {
	ElapsedMS           int64   `json:"elapsed_ms"`
	UsedMemory          int64   `json:"used_memory"`
	UsedMemoryRSS       int64   `json:"used_memory_rss"`
	FragmentationRatio  float64 `json:"fragmentation_ratio"`
	RejectedConnections int64   `json:"rejected_connections"`
	ConnectionsReceived int64   `json:"connections_received"`
	CommandsProcessed   int64   `json:"commands_processed"`
	NetInputBytes       int64   `json:"net_input_bytes"`
	NetOutputBytes      int64   `json:"net_output_bytes"`
	EvictedKeys         int64   `json:"evicted_keys"`
	ExpiredKeys         int64   `json:"expired_keys"`
	BlockedClients      int64   `json:"blocked_clients"`
	PingLatencyMS       int64   `json:"ping_latency_ms"`
	CPUUser             float64 `json:"cpu_user"`
	CPUSys              float64 `json:"cpu_sys"`
}

type trendPayload struct {
	Samples          int   `json:"samples"`
	WindowMS         int64 `json:"window_ms"`
	NetUsedMemory    int64 `json:"net_used_memory"`
	NetUsedMemoryRSS int64 `json:"net_used_memory_rss"`
	PositiveSteps    int   `json:"positive_steps"`
	NegativeSteps    int   `json:"negative_steps"`
	SustainedGrowth  bool  `json:"sustained_growth"`
}

type seriesPayload struct {
	Points []seriesPoint `json:"points"`
}

type seriesPoint struct {
	CapturedAt    string  `json:"captured_at"`
	UsedMemory    uint64  `json:"used_memory"`
	UsedMemoryRSS uint64  `json:"used_memory_rss"`
	OpsPerSec     int64   `json:"ops_per_sec"`
	HitRatio      float64 `json:"hit_ratio"`
}

type persistentScanPayload struct {
	CapturedAt              string                   `json:"captured_at,omitempty"`
	DB                      int                      `json:"db"`
	SampledKeys             int                      `json:"sampled_keys"`
	PersistentSampledKeys   int                      `json:"persistent_sampled_keys"`
	PersistentRatioInSample float64                  `json:"persistent_ratio_in_sample"`
	Groups                  []persistentGroupPayload `json:"groups,omitempty"`
	Cached                  bool                     `json:"cached"`
}

type persistentGroupPayload struct {
	Prefix   string   `json:"prefix"`
	Count    int      `json:"count"`
	Share    float64  `json:"share"`
	Examples []string `json:"examples,omitempty"`
}

func newStreamMessage(report redisstats.Report, history []redisstats.Snapshot, persistentScan *redisstats.PersistentKeyScan, scope, stateFile string) streamMessage {
	msg := streamMessage{
		Status:      "ok",
		GeneratedAt: time.Now().Format(time.RFC3339),
		Connection:  connectionMeta{Scope: scope, StateFile: stateFile},
		Snapshot:    toSnapshotPayload(report.Snapshot),
		Series:      toSeriesPayload(history),
		Persistent:  toPersistentScanPayload(persistentScan, true),
		Alerts:      prioritizedAlerts(report, scope),
		Warnings:    report.Warnings,
	}

	if report.Delta != nil {
		msg.Delta = &deltaPayload{
			ElapsedMS:           report.Delta.Elapsed.Milliseconds(),
			UsedMemory:          report.Delta.UsedMemory,
			UsedMemoryRSS:       report.Delta.UsedMemoryRSS,
			FragmentationRatio:  report.Delta.FragmentationRatio,
			RejectedConnections: report.Delta.RejectedConnections,
			ConnectionsReceived: report.Delta.ConnectionsReceived,
			CommandsProcessed:   report.Delta.CommandsProcessed,
			NetInputBytes:       report.Delta.NetInputBytes,
			NetOutputBytes:      report.Delta.NetOutputBytes,
			EvictedKeys:         report.Delta.EvictedKeys,
			ExpiredKeys:         report.Delta.ExpiredKeys,
			BlockedClients:      report.Delta.BlockedClients,
			PingLatencyMS:       report.Delta.PingLatency.Milliseconds(),
			CPUUser:             report.Delta.CPUUser,
			CPUSys:              report.Delta.CPUSys,
		}
	}

	if report.MemoryTrend != nil {
		msg.MemoryTrend = &trendPayload{
			Samples:          report.MemoryTrend.Samples,
			WindowMS:         report.MemoryTrend.Window.Milliseconds(),
			NetUsedMemory:    report.MemoryTrend.NetUsedMemory,
			NetUsedMemoryRSS: report.MemoryTrend.NetUsedMemoryRSS,
			PositiveSteps:    report.MemoryTrend.PositiveSteps,
			NegativeSteps:    report.MemoryTrend.NegativeSteps,
			SustainedGrowth:  report.MemoryTrend.SustainedGrowth,
		}
	}

	return msg
}

func toSeriesPayload(history []redisstats.Snapshot) *seriesPayload {
	if len(history) == 0 {
		return nil
	}

	points := make([]seriesPoint, 0, len(history))
	for _, snapshot := range history {
		points = append(points, seriesPoint{
			CapturedAt:    snapshot.CapturedAt.Format(time.RFC3339),
			UsedMemory:    snapshot.Memory.UsedMemory,
			UsedMemoryRSS: snapshot.Memory.UsedMemoryRSS,
			OpsPerSec:     snapshot.Traffic.InstantaneousOpsPerSec,
			HitRatio:      hitRatio(snapshot),
		})
	}

	return &seriesPayload{Points: points}
}

func toSnapshotPayload(snapshot redisstats.Snapshot) *snapshotPayload {
	dbs := make([]databaseInfo, 0, len(snapshot.Keyspace.Databases))
	names := make([]string, 0, len(snapshot.Keyspace.Databases))
	for name := range snapshot.Keyspace.Databases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		db := snapshot.Keyspace.Databases[name]
		dbs = append(dbs, databaseInfo{Name: name, Keys: db.Keys, Expires: db.Expires, AvgTTL: db.AvgTTL})
	}

	return &snapshotPayload{
		CapturedAt:          snapshot.CapturedAt.Format(time.RFC3339),
		PingLatencyMS:       snapshot.PingLatency.Milliseconds(),
		UsedMemory:          snapshot.Memory.UsedMemory,
		UsedMemoryRSS:       snapshot.Memory.UsedMemoryRSS,
		UsedMemoryPeak:      snapshot.Memory.UsedMemoryPeak,
		UsedMemoryDataset:   snapshot.Memory.UsedMemoryDataset,
		MemFragmentation:    snapshot.Memory.FragmentationRatio,
		MaxMemory:           snapshot.Memory.MaxMemory,
		MaxMemoryPolicy:     snapshot.Memory.MaxMemoryPolicy,
		ConnectedClients:    snapshot.Clients.ConnectedClients,
		BlockedClients:      snapshot.Clients.BlockedClients,
		RejectedConnections: snapshot.Clients.RejectedConnections,
		ConnectionsReceived: snapshot.Clients.TotalConnectionsReceived,
		InstantOpsPerSec:    snapshot.Traffic.InstantaneousOpsPerSec,
		CommandsProcessed:   snapshot.Traffic.TotalCommandsProcessed,
		NetInputBytes:       snapshot.Traffic.NetInputBytes,
		NetOutputBytes:      snapshot.Traffic.NetOutputBytes,
		UsedCPUUser:         snapshot.CPU.UsedCPUUser,
		UsedCPUSys:          snapshot.CPU.UsedCPUSys,
		Role:                snapshot.Persistence.Role,
		ConnectedReplicas:   snapshot.Persistence.ConnectedReplicas,
		AOFEnabled:          snapshot.Persistence.AOFEnabled,
		RDBLastBGSaveStatus: snapshot.Persistence.RDBLastBGSaveStatus,
		RDBLastSaveTime:     formatOptionalTime(snapshot.Persistence.RDBLastSaveTime),
		UptimeSeconds:       snapshot.General.UptimeSeconds,
		EvictedKeys:         snapshot.General.EvictedKeys,
		ExpiredKeys:         snapshot.General.ExpiredKeys,
		KeyspaceHits:        snapshot.General.KeyspaceHits,
		KeyspaceMisses:      snapshot.General.KeyspaceMisses,
		HitRatio:            hitRatio(snapshot),
		LatestForkUsec:      snapshot.General.LatestForkUsec,
		DBSize:              snapshot.Keyspace.DBSize,
		TotalKeys:           snapshot.Keyspace.TotalKeys,
		ExpiringKeys:        snapshot.Keyspace.ExpiringKeys,
		TTLPercent:          snapshot.Keyspace.TTLProfile.ExpiringRatio,
		WeightedAvgTTL:      snapshot.Keyspace.TTLProfile.WeightedAvgTTL,
		TTLLabel:            snapshot.Keyspace.TTLProfile.Label,
		Databases:           dbs,
	}
}

func toPersistentScanPayload(scan *redisstats.PersistentKeyScan, cached bool) *persistentScanPayload {
	if scan == nil {
		return nil
	}

	groups := make([]persistentGroupPayload, 0, len(scan.Groups))
	for _, group := range scan.Groups {
		groups = append(groups, persistentGroupPayload{
			Prefix:   group.Prefix,
			Count:    group.Count,
			Share:    group.Share,
			Examples: append([]string(nil), group.ExampleKeys...),
		})
	}

	return &persistentScanPayload{
		CapturedAt:              formatOptionalTime(scan.CapturedAt),
		DB:                      scan.DB,
		SampledKeys:             scan.SampledKeys,
		PersistentSampledKeys:   scan.PersistentSampledKeys,
		PersistentRatioInSample: scan.PersistentRatioInSample,
		Groups:                  groups,
		Cached:                  cached,
	}
}

func hitRatio(snapshot redisstats.Snapshot) float64 {
	total := snapshot.General.KeyspaceHits + snapshot.General.KeyspaceMisses
	if total == 0 {
		return 0
	}
	return float64(snapshot.General.KeyspaceHits) / float64(total)
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
