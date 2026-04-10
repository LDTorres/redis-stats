package redisstats

import (
	"testing"
	"time"
)

func TestParseInfoAndSnapshotFromInfo(t *testing.T) {
	raw := `
# Memory
used_memory:10485760
used_memory_rss:12582912
used_memory_peak:16777216
used_memory_dataset:7340032
mem_fragmentation_ratio:1.20
maxmemory:268435456
maxmemory_policy:allkeys-lru
# Clients
connected_clients:23
blocked_clients:2
rejected_connections:3
total_connections_received:50
# Stats
total_commands_processed:500
instantaneous_ops_per_sec:25
total_net_input_bytes:1000
total_net_output_bytes:2000
keyspace_hits:80
keyspace_misses:20
expired_keys:5
evicted_keys:1
# CPU
used_cpu_sys:1.5
used_cpu_user:2.5
used_cpu_sys_children:0.1
used_cpu_user_children:0.2
# Persistence
rdb_last_bgsave_status:ok
rdb_last_save_time:1710000000
aof_enabled:1
aof_last_bgrewrite_status:ok
# Replication
role:master
connected_replicas:2
# Server
uptime_in_seconds:100
latest_fork_usec:200
# Keyspace
db0:keys=10,expires=3,avg_ttl=5000
db1:keys=7,expires=2,avg_ttl=1000
db0_distrib_hashes_items:keys=999,expires=999,avg_ttl=999
`

	snapshot := snapshotFromInfo(parseInfo(raw), 17, 5*time.Millisecond, time.Unix(1710000100, 0))

	if snapshot.Memory.UsedMemory != 10485760 {
		t.Fatalf("unexpected used_memory: %d", snapshot.Memory.UsedMemory)
	}
	if snapshot.Clients.BlockedClients != 2 {
		t.Fatalf("unexpected blocked_clients: %d", snapshot.Clients.BlockedClients)
	}
	if snapshot.Persistence.ConnectedReplicas != 2 {
		t.Fatalf("unexpected connected replicas: %d", snapshot.Persistence.ConnectedReplicas)
	}
	if snapshot.Keyspace.TotalKeys != 17 {
		t.Fatalf("unexpected key count: %d", snapshot.Keyspace.TotalKeys)
	}
	if snapshot.Keyspace.ExpiringKeys != 5 {
		t.Fatalf("unexpected expiring keys: %d", snapshot.Keyspace.ExpiringKeys)
	}
	if snapshot.Keyspace.TTLProfile.Label != "mixed TTL" {
		t.Fatalf("unexpected ttl label: %q", snapshot.Keyspace.TTLProfile.Label)
	}
	if snapshot.Keyspace.TTLProfile.WeightedAvgTTL != 3400 {
		t.Fatalf("unexpected weighted avg ttl: %d", snapshot.Keyspace.TTLProfile.WeightedAvgTTL)
	}
	if snapshot.PingLatency != 5*time.Millisecond {
		t.Fatalf("unexpected ping latency: %s", snapshot.PingLatency)
	}
	if _, exists := snapshot.Keyspace.Databases["db0_distrib_hashes_items"]; exists {
		t.Fatal("expected non-keyspace metrics to be ignored")
	}
}
