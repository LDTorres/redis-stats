# redis-stats

Go CLI for inspecting a Redis instance and surfacing early operational signals, especially sustained memory growth.

The project also includes an embedded web dashboard with live WebSocket updates.

[Redis Stats dashboard](assets/screenshot.png)

Live dashboard view with memory, alerts, charts, and TTL diagnostics.

## What It Shows

- Memory usage and RSS to spot growth and fragmentation.
- Connected clients, blocked clients, and rejected connections.
- Throughput, operations per second, and network traffic.
- CPU totals, persistence state, and replication role.
- Evictions, expirations, hit ratio, and logical keyspace size.
- Basic `PING` latency per sample.
- Aggregate TTL profile plus a sampled manual scan for key families without TTL.

## What This Is

- A lightweight Redis troubleshooting tool for snapshots, live sampling, and quick dashboarding.
- Useful for spotting memory leaks, fragmentation, connection pressure, and persistent key families.
- Safe for routine use in `watch` and `serve` mode because the fast path relies on `INFO`, `DBSIZE`, and `PING`.

## What This Is Not

- Not a replacement for long-term metrics storage, Prometheus, or dedicated observability stacks.
- Not a cluster-aware audit tool for Redis Cluster or Sentinel in this first version.
- Not a safe continuous production probe for exhaustive TTL inspection; `ttl-audit` is intentionally a manual tool.

## Modes

### `watch`

Default mode. Samples every `5s`, computes deltas against the previous sample, and emits simple warnings.

```bash
go run ./cmd/redis-stats
go run ./cmd/redis-stats watch --addr localhost:6379 --interval 3s
```

### `snapshot`

Prints a point-in-time Redis snapshot.

```bash
go run ./cmd/redis-stats snapshot --redis-url redis://localhost:6379/0
```

### `serve`

Starts a local web dashboard and streams updates over WebSocket.

```bash
go run ./cmd/redis-stats serve
go run ./cmd/redis-stats serve --listen 127.0.0.1:8080 --interval 3s
```

### `ttl-audit`

Exhaustively scans the configured DB, identifies keys without TTL, and groups them by prefix with concrete example keys.

```bash
go run ./cmd/redis-stats ttl-audit
go run ./cmd/redis-stats ttl-audit --redis-url redis://localhost:6379/0
```

## Running from a Release Artifact

Each GitHub release includes prebuilt binaries for:

- macOS Intel: `darwin_amd64`
- macOS Apple Silicon: `darwin_arm64`
- Linux x86_64: `linux_amd64`
- Linux ARM64: `linux_arm64`
- Windows x86_64: `windows_amd64`

After downloading the archive for your platform, extract it and run the binary directly.

macOS / Linux:

```bash
tar -xzf redis-stats_<VERSION>_<os>_<arch>.tar.gz
cd redis-stats_<VERSION>_<os>_<arch>

./redis-stats snapshot --redis-url redis://localhost:6379/0
./redis-stats watch --redis-url redis://localhost:6379/0
./redis-stats serve --redis-url redis://localhost:6379/0
./redis-stats ttl-audit --redis-url redis://localhost:6379/0
```

Windows PowerShell:

```powershell
Expand-Archive .\redis-stats_<VERSION>_windows_amd64.zip
cd .\redis-stats_<VERSION>_windows_amd64

.\redis-stats.exe snapshot --redis-url redis://localhost:6379/0
.\redis-stats.exe watch --redis-url redis://localhost:6379/0
.\redis-stats.exe serve --redis-url redis://localhost:6379/0
.\redis-stats.exe ttl-audit --redis-url redis://localhost:6379/0
```

By default, `serve` listens on `127.0.0.1:8080`.

## Configuration

Available flags:

- `--addr`
- `--username`
- `--password`
- `--db`
- `--redis-url`
- `--tls`
- `--interval`
- `--timeout`
- `--listen`
- `--history-size`
- `--trend-min-samples`
- `--state-file`
- `--ttl-scan-sample-size`

Matching environment variables:

- `REDIS_ADDR`
- `REDIS_USERNAME`
- `REDIS_PASSWORD`
- `REDIS_DB`
- `REDIS_URL`
- `REDIS_TLS`
- `REDIS_INTERVAL`
- `REDIS_TIMEOUT`
- `REDIS_STATS_LISTEN`
- `REDIS_STATS_HISTORY_SIZE`
- `REDIS_STATS_TREND_MIN_SAMPLES`
- `REDIS_STATS_STATE_FILE`
- `REDIS_STATS_TTL_SCAN_SAMPLE_SIZE`

Configuration precedence:

1. Explicit flags
2. Environment variables
3. `.env`
4. Defaults

When `REDIS_URL` or `--redis-url` is set, explicit fields like `--addr`, `--db`, or `--tls` can still override parts of that URL.

## Warnings and Diagnostics

The CLI emits warnings for cases such as:

- Meaningful `used_memory` growth.
- Large `used_memory_rss` jumps.
- High or worsening `mem_fragmentation_ratio`.
- `blocked_clients` above zero.
- New `rejected_connections` or `evicted_keys`.
- Low hit ratio.
- High `PING` latency.
- `BGSAVE` or `BGREWRITEAOF` failures.

## Safety / Cost Notes

- `watch` and `serve` are lightweight and designed for routine operational use.
- The dashboard TTL scan is sampled and on-demand. It is more expensive than the live metrics loop, but still bounded.
- `ttl-audit` is exhaustive for the configured DB and can be expensive on large or busy Redis instances. Treat it as a manual investigation tool, not a continuous monitor.
- The embedded dashboard server is intended for trusted/local use by default. If you expose it beyond localhost, harden the HTTP layer and WebSocket origin policy first.

## Development

```bash
go test ./...
go run ./cmd/redis-stats snapshot
go run ./cmd/redis-stats serve
```

## Make Targets

```bash
make help
make run-cli
make run-watch
make run-server
make run-audit
make test
make build
make build-release
make release-notes VERSION=v0.1.0
make publish-release VERSION=v0.1.0
```

`publish-release` requires the GitHub CLI (`gh`) to be installed and authenticated.
By default it renders release notes from `.github/RELEASE_TEMPLATE.md`.
You can override that with `RELEASE_NOTES_FILE=/path/to/release-notes.md`.

To keep the dashboard running longer and persist history:

```bash
go run ./cmd/redis-stats serve --interval 5s --history-size 720 --trend-min-samples 24 --state-file .redis-stats-state.json

# Manual sampled scan for keys without TTL
go run ./cmd/redis-stats serve --ttl-scan-sample-size 200
```
