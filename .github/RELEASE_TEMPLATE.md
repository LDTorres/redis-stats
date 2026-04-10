# redis-stats {{VERSION}}

## Downloads

- macOS Apple Silicon: `redis-stats_{{VERSION}}_darwin_arm64.tar.gz`
- macOS Intel: `redis-stats_{{VERSION}}_darwin_amd64.tar.gz`
- Linux x86_64: `redis-stats_{{VERSION}}_linux_amd64.tar.gz`
- Linux ARM64: `redis-stats_{{VERSION}}_linux_arm64.tar.gz`
- Windows x86_64: `redis-stats_{{VERSION}}_windows_amd64.zip`

## Highlights

- Initial Release

## Quick Start

macOS / Linux:

```bash
tar -xzf redis-stats_{{VERSION}}_<os>_<arch>.tar.gz
cd redis-stats_{{VERSION}}_<os>_<arch>

./redis-stats snapshot --redis-url redis://localhost:6379/0
./redis-stats watch --redis-url redis://localhost:6379/0
./redis-stats serve --redis-url redis://localhost:6379/0
./redis-stats ttl-audit --redis-url redis://localhost:6379/0
```

Windows PowerShell:

```powershell
Expand-Archive .\redis-stats_{{VERSION}}_windows_amd64.zip
cd .\redis-stats_{{VERSION}}_windows_amd64

.\redis-stats.exe snapshot --redis-url redis://localhost:6379/0
.\redis-stats.exe watch --redis-url redis://localhost:6379/0
.\redis-stats.exe serve --redis-url redis://localhost:6379/0
.\redis-stats.exe ttl-audit --redis-url redis://localhost:6379/0
```

## Notes

- `serve` starts a local dashboard on `127.0.0.1:8080` by default.
- The dashboard TTL scan is sampled and on-demand.
- `ttl-audit` is exhaustive for the configured DB and can be expensive on large or busy Redis instances.
- See the repository README for full configuration and safety notes.
