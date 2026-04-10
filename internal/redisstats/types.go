package redisstats

import "time"

type Snapshot struct {
	CapturedAt  time.Time
	PingLatency time.Duration
	Memory      MemoryStats
	Clients     ClientStats
	Traffic     TrafficStats
	CPU         CPUStats
	Persistence PersistenceStats
	General     GeneralStats
	Keyspace    KeyspaceStats
}

type MemoryStats struct {
	UsedMemory         uint64
	UsedMemoryRSS      uint64
	UsedMemoryPeak     uint64
	UsedMemoryDataset  uint64
	MaxMemory          uint64
	FragmentationRatio float64
	MaxMemoryPolicy    string
}

type ClientStats struct {
	ConnectedClients         int64
	BlockedClients           int64
	RejectedConnections      int64
	TotalConnectionsReceived int64
}

type TrafficStats struct {
	TotalCommandsProcessed int64
	InstantaneousOpsPerSec int64
	NetInputBytes          int64
	NetOutputBytes         int64
}

type CPUStats struct {
	UsedCPUSys          float64
	UsedCPUUser         float64
	UsedCPUSysChildren  float64
	UsedCPUUserChildren float64
}

type PersistenceStats struct {
	RDBLastBGSaveStatus    string
	RDBLastSaveTime        time.Time
	AOFEnabled             bool
	AOFLastBGRewriteStatus string
	Role                   string
	ConnectedReplicas      int64
}

type GeneralStats struct {
	UptimeSeconds  int64
	EvictedKeys    int64
	ExpiredKeys    int64
	KeyspaceHits   int64
	KeyspaceMisses int64
	LatestForkUsec int64
}

type KeyspaceStats struct {
	DBSize       int64
	Databases    map[string]KeyspaceDB
	TotalKeys    int64
	ExpiringKeys int64
	TTLProfile   TTLProfile
}

type KeyspaceDB struct {
	Keys    int64
	Expires int64
	AvgTTL  int64
}

type TTLProfile struct {
	ExpiringRatio  float64
	WeightedAvgTTL int64
	Label          string
}

type PersistentKeyScan struct {
	CapturedAt              time.Time
	DB                      int
	SampledKeys             int
	PersistentSampledKeys   int
	PersistentRatioInSample float64
	Groups                  []PersistentKeyGroup
}

type PersistentKeyGroup struct {
	Prefix      string
	Count       int
	Share       float64
	ExampleKeys []string
}

type PersistentKeyAudit struct {
	CapturedAt      time.Time
	DB              int
	ScannedKeys     int
	PersistentKeys  int
	PersistentRatio float64
	Groups          []PersistentKeyAuditGroup
}

type PersistentKeyAuditGroup struct {
	Prefix      string
	Count       int
	Share       float64
	ExampleKeys []string
}

type Delta struct {
	Elapsed             time.Duration
	UsedMemory          int64
	UsedMemoryRSS       int64
	FragmentationRatio  float64
	RejectedConnections int64
	ConnectionsReceived int64
	CommandsProcessed   int64
	NetInputBytes       int64
	NetOutputBytes      int64
	EvictedKeys         int64
	ExpiredKeys         int64
	BlockedClients      int64
	PingLatency         time.Duration
	CPUUser             float64
	CPUSys              float64
	HitRatio            float64
}

type Report struct {
	Snapshot    Snapshot
	Delta       *Delta
	MemoryTrend *MemoryTrend
	Warnings    []string
}

type MemoryTrend struct {
	Samples          int
	Window           time.Duration
	NetUsedMemory    int64
	NetUsedMemoryRSS int64
	PositiveSteps    int
	NegativeSteps    int
	SustainedGrowth  bool
}
