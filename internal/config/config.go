package config

import (
	"bufio"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultAddr            = "localhost:6379"
	DefaultDB              = 0
	DefaultInterval        = 5 * time.Second
	DefaultTimeout         = 3 * time.Second
	DefaultListen          = "127.0.0.1:8080"
	DefaultHistorySize     = 120
	DefaultTrendMinSamples = 12
	DefaultStateFile       = ".redis-stats-state.json"
	DefaultTTLScanSample   = 200
)

type Command string

const (
	CommandSnapshot Command = "snapshot"
	CommandWatch    Command = "watch"
	CommandServe    Command = "serve"
	CommandTTLAudit Command = "ttl-audit"
)

type Config struct {
	Command         Command
	Addr            string
	Username        string
	Password        string
	DB              int
	RedisURL        string
	TLS             bool
	Interval        time.Duration
	Timeout         time.Duration
	Listen          string
	HistorySize     int
	TrendMinSamples int
	StateFile       string
	TTLScanSample   int

	hasAddr            bool
	hasUsername        bool
	hasPassword        bool
	hasDB              bool
	hasRedisURL        bool
	hasTLS             bool
	hasInterval        bool
	hasTimeout         bool
	hasListen          bool
	hasHistorySize     bool
	hasTrendMinSamples bool
	hasStateFile       bool
	hasTTLScanSample   bool
}

func (c Config) ConnectionScope() string {
	host := c.connectionHost()
	if host == "" {
		return "unknown"
	}

	normalized := strings.ToLower(host)
	switch normalized {
	case "localhost", "host.docker.internal":
		return "local"
	}

	if ip := net.ParseIP(normalized); ip != nil {
		switch {
		case ip.IsLoopback():
			return "local"
		case ip.IsPrivate(), ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
			return "private"
		default:
			return "remote"
		}
	}

	if strings.HasSuffix(normalized, ".local") {
		return "local"
	}

	return "remote"
}

func (c Config) connectionHost() string {
	if c.RedisURL != "" {
		parsed, err := url.Parse(c.RedisURL)
		if err == nil {
			if host := parsed.Hostname(); host != "" {
				return host
			}
		}
	}

	host, _, err := net.SplitHostPort(c.Addr)
	if err == nil {
		return host
	}

	return c.Addr
}

func (c Config) RedisOptions() (*redis.Options, error) {
	if c.RedisURL != "" {
		opts, err := redis.ParseURL(c.RedisURL)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}

		applyOverrides(opts, c)
		return opts, nil
	}

	return &redis.Options{
		Addr:         c.Addr,
		Username:     c.Username,
		Password:     c.Password,
		DB:           c.DB,
		DialTimeout:  c.Timeout,
		ReadTimeout:  c.Timeout,
		WriteTimeout: c.Timeout,
		TLSConfig:    tlsConfig(c.TLS),
	}, nil
}

func applyOverrides(opts *redis.Options, cfg Config) {
	if cfg.hasAddr {
		opts.Addr = cfg.Addr
	}
	if cfg.hasUsername {
		opts.Username = cfg.Username
	}
	if cfg.hasPassword {
		opts.Password = cfg.Password
	}
	if cfg.hasDB {
		opts.DB = cfg.DB
	}
	if cfg.hasTimeout {
		opts.DialTimeout = cfg.Timeout
		opts.ReadTimeout = cfg.Timeout
		opts.WriteTimeout = cfg.Timeout
	}
	if cfg.hasTLS {
		opts.TLSConfig = tlsConfig(cfg.TLS)
	}
}

func tlsConfig(enabled bool) *tls.Config {
	if !enabled {
		return nil
	}

	return redisTLSConfig()
}

func ParseArgs(args []string, stderr io.Writer) (Config, error) {
	command, remaining := detectCommand(args)
	cfg := defaults()
	cfg.Command = command

	dotEnvCfg, err := loadDotEnv(".env")
	if err != nil {
		return Config{}, err
	}
	merge(&cfg, dotEnvCfg)

	envCfg, err := loadEnv()
	if err != nil {
		return Config{}, err
	}
	merge(&cfg, envCfg)

	flags, err := parseFlags(command, remaining, stderr)
	if err != nil {
		return Config{}, err
	}
	merge(&cfg, flags)

	return cfg, nil
}

func Usage(w io.Writer) {
	fmt.Fprintf(w, `redis-stats inspects Redis and highlights early memory-growth and health signals.

Usage:
  redis-stats [flags]
  redis-stats watch [flags]
  redis-stats snapshot [flags]
  redis-stats serve [flags]
  redis-stats ttl-audit [flags]

Commands:
  watch      Continuous mode with deltas and warnings between samples.
  snapshot   Prints a point-in-time snapshot of the current Redis state.
  serve      Starts a local dashboard with WebSocket updates.
  ttl-audit  Scans the configured DB and summarizes keys without TTL.

Flags:
  --addr         Redis address, default %q
  --username     Redis ACL username
  --password     Redis ACL password
  --db           Redis DB number, default %d
  --redis-url    Full redis:// or rediss:// URL
  --tls          Force TLS when not using rediss://
  --interval     Sampling interval for watch/serve, default %s
  --timeout      Network timeout, default %s
  --listen       Dashboard HTTP listen address, default %q
  --history-size Retained sample count for charts and trend analysis, default %d
  --trend-min-samples Minimum samples required to confirm a trend, default %d
  --state-file   JSON file used to persist dashboard history, default %q
  --ttl-scan-sample-size Number of keys to inspect in the manual TTL dashboard scan, default %d

Environment variables:
  REDIS_ADDR, REDIS_USERNAME, REDIS_PASSWORD, REDIS_DB,
  REDIS_URL, REDIS_TLS, REDIS_INTERVAL, REDIS_TIMEOUT,
  REDIS_STATS_LISTEN, REDIS_STATS_HISTORY_SIZE, REDIS_STATS_TREND_MIN_SAMPLES,
  REDIS_STATS_STATE_FILE, REDIS_STATS_TTL_SCAN_SAMPLE_SIZE

Precedence:
  flags > environment variables > .env > defaults
`, DefaultAddr, DefaultDB, DefaultInterval, DefaultTimeout, DefaultListen, DefaultHistorySize, DefaultTrendMinSamples, DefaultStateFile, DefaultTTLScanSample)
}

func defaults() Config {
	return Config{
		Command:         CommandWatch,
		Addr:            DefaultAddr,
		DB:              DefaultDB,
		Interval:        DefaultInterval,
		Timeout:         DefaultTimeout,
		Listen:          DefaultListen,
		HistorySize:     DefaultHistorySize,
		TrendMinSamples: DefaultTrendMinSamples,
		StateFile:       DefaultStateFile,
		TTLScanSample:   DefaultTTLScanSample,
	}
}

func detectCommand(args []string) (Command, []string) {
	if len(args) == 0 {
		return CommandWatch, nil
	}

	switch args[0] {
	case string(CommandSnapshot):
		return CommandSnapshot, args[1:]
	case string(CommandWatch):
		return CommandWatch, args[1:]
	case string(CommandServe):
		return CommandServe, args[1:]
	case string(CommandTTLAudit):
		return CommandTTLAudit, args[1:]
	default:
		return CommandWatch, args
	}
}

func loadEnv() (Config, error) {
	return loadConfigFromLookup(os.LookupEnv)
}

func loadDotEnv(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	values, err := parseDotEnv(file)
	if err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}

	return loadConfigFromLookup(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
}

func loadConfigFromLookup(lookup func(string) (string, bool)) (Config, error) {
	var cfg Config
	var err error

	if value, ok := lookup("REDIS_ADDR"); ok {
		cfg.Addr = value
		cfg.hasAddr = true
	}
	if value, ok := lookup("REDIS_USERNAME"); ok {
		cfg.Username = value
		cfg.hasUsername = true
	}
	if value, ok := lookup("REDIS_PASSWORD"); ok {
		cfg.Password = value
		cfg.hasPassword = true
	}
	if value, ok := lookup("REDIS_URL"); ok {
		cfg.RedisURL = value
		cfg.hasRedisURL = true
	}

	if db, ok := lookup("REDIS_DB"); ok {
		cfg.DB, err = strconv.Atoi(db)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REDIS_DB: %w", err)
		}
		cfg.hasDB = true
	}

	if tlsValue, ok := lookup("REDIS_TLS"); ok {
		cfg.TLS, err = strconv.ParseBool(tlsValue)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REDIS_TLS: %w", err)
		}
		cfg.hasTLS = true
	}

	if interval, ok := lookup("REDIS_INTERVAL"); ok {
		cfg.Interval, err = time.ParseDuration(interval)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REDIS_INTERVAL: %w", err)
		}
		cfg.hasInterval = true
	}

	if timeout, ok := lookup("REDIS_TIMEOUT"); ok {
		cfg.Timeout, err = time.ParseDuration(timeout)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REDIS_TIMEOUT: %w", err)
		}
		cfg.hasTimeout = true
	}
	if value, ok := lookup("REDIS_STATS_LISTEN"); ok {
		cfg.Listen = value
		cfg.hasListen = true
	}
	if value, ok := lookup("REDIS_STATS_HISTORY_SIZE"); ok {
		cfg.HistorySize, err = strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REDIS_STATS_HISTORY_SIZE: %w", err)
		}
		cfg.hasHistorySize = true
	}
	if value, ok := lookup("REDIS_STATS_TREND_MIN_SAMPLES"); ok {
		cfg.TrendMinSamples, err = strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REDIS_STATS_TREND_MIN_SAMPLES: %w", err)
		}
		cfg.hasTrendMinSamples = true
	}
	if value, ok := lookup("REDIS_STATS_STATE_FILE"); ok {
		cfg.StateFile = value
		cfg.hasStateFile = true
	}
	if value, ok := lookup("REDIS_STATS_TTL_SCAN_SAMPLE_SIZE"); ok {
		cfg.TTLScanSample, err = strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REDIS_STATS_TTL_SCAN_SAMPLE_SIZE: %w", err)
		}
		cfg.hasTTLScanSample = true
	}

	return cfg, nil
}

func parseDotEnv(r io.Reader) (map[string]string, error) {
	scanner := bufio.NewScanner(r)
	values := make(map[string]string)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid assignment at line %d", lineNo)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("empty key at line %d", lineNo)
		}

		if len(value) >= 2 {
			if value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
			}
			if value[0] == '\'' && value[len(value)-1] == '\'' {
				value = value[1 : len(value)-1]
			}
		}

		values[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

func parseFlags(command Command, args []string, stderr io.Writer) (Config, error) {
	fs := flag.NewFlagSet(string(command), flag.ContinueOnError)
	fs.SetOutput(stderr)

	var cfg Config
	var addr trackedString
	var username trackedString
	var password trackedString
	var redisURL trackedString
	var db trackedInt
	var tls trackedBool
	var interval trackedDuration
	var timeout trackedDuration
	var listen trackedString
	var historySize trackedInt
	var trendMinSamples trackedInt
	var stateFile trackedString
	var ttlScanSample trackedInt

	fs.Var(&addr, "addr", "Redis address")
	fs.Var(&username, "username", "Redis ACL username")
	fs.Var(&password, "password", "Redis ACL password")
	fs.Var(&redisURL, "redis-url", "Redis URL")
	fs.Var(&db, "db", "Redis database")
	fs.Var(&tls, "tls", "Enable TLS")
	fs.Var(&interval, "interval", "Sample interval")
	fs.Var(&timeout, "timeout", "Network timeout")
	fs.Var(&listen, "listen", "Dashboard listen address")
	fs.Var(&historySize, "history-size", "Retained sample count")
	fs.Var(&trendMinSamples, "trend-min-samples", "Minimum samples for trend confirmation")
	fs.Var(&stateFile, "state-file", "Dashboard state file")
	fs.Var(&ttlScanSample, "ttl-scan-sample-size", "Persistent key TTL scan sample size")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return Config{}, err
		}
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}

	if len(fs.Args()) > 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	if addr.set {
		cfg.Addr = addr.value
		cfg.hasAddr = true
	}
	if username.set {
		cfg.Username = username.value
		cfg.hasUsername = true
	}
	if password.set {
		cfg.Password = password.value
		cfg.hasPassword = true
	}
	if redisURL.set {
		cfg.RedisURL = redisURL.value
		cfg.hasRedisURL = true
	}
	if db.set {
		cfg.DB = db.value
		cfg.hasDB = true
	}
	if tls.set {
		cfg.TLS = tls.value
		cfg.hasTLS = true
	}
	if interval.set {
		cfg.Interval = interval.value
		cfg.hasInterval = true
	}
	if timeout.set {
		cfg.Timeout = timeout.value
		cfg.hasTimeout = true
	}
	if listen.set {
		cfg.Listen = listen.value
		cfg.hasListen = true
	}
	if historySize.set {
		cfg.HistorySize = historySize.value
		cfg.hasHistorySize = true
	}
	if trendMinSamples.set {
		cfg.TrendMinSamples = trendMinSamples.value
		cfg.hasTrendMinSamples = true
	}
	if stateFile.set {
		cfg.StateFile = stateFile.value
		cfg.hasStateFile = true
	}
	if ttlScanSample.set {
		cfg.TTLScanSample = ttlScanSample.value
		cfg.hasTTLScanSample = true
	}

	return cfg, nil
}

func merge(dst *Config, src Config) {
	if src.hasAddr {
		dst.Addr = src.Addr
		dst.hasAddr = true
	}
	if src.hasUsername {
		dst.Username = src.Username
		dst.hasUsername = true
	}
	if src.hasPassword {
		dst.Password = src.Password
		dst.hasPassword = true
	}
	if src.hasRedisURL {
		dst.RedisURL = src.RedisURL
		dst.hasRedisURL = true
	}
	if src.hasDB {
		dst.DB = src.DB
		dst.hasDB = true
	}
	if src.hasInterval {
		dst.Interval = src.Interval
		dst.hasInterval = true
	}
	if src.hasTimeout {
		dst.Timeout = src.Timeout
		dst.hasTimeout = true
	}
	if src.hasTLS {
		dst.TLS = src.TLS
		dst.hasTLS = true
	}
	if src.hasListen {
		dst.Listen = src.Listen
		dst.hasListen = true
	}
	if src.hasHistorySize {
		dst.HistorySize = src.HistorySize
		dst.hasHistorySize = true
	}
	if src.hasTrendMinSamples {
		dst.TrendMinSamples = src.TrendMinSamples
		dst.hasTrendMinSamples = true
	}
	if src.hasStateFile {
		dst.StateFile = src.StateFile
		dst.hasStateFile = true
	}
	if src.hasTTLScanSample {
		dst.TTLScanSample = src.TTLScanSample
		dst.hasTTLScanSample = true
	}
}

type trackedString struct {
	value string
	set   bool
}

func (t *trackedString) Set(value string) error {
	t.value = value
	t.set = true
	return nil
}

func (t *trackedString) String() string { return t.value }

type trackedInt struct {
	value int
	set   bool
}

func (t *trackedInt) Set(value string) error {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	t.value = parsed
	t.set = true
	return nil
}

func (t *trackedInt) String() string { return strconv.Itoa(t.value) }

type trackedBool struct {
	value bool
	set   bool
}

func (t *trackedBool) IsBoolFlag() bool { return true }

func (t *trackedBool) Set(value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	t.value = parsed
	t.set = true
	return nil
}

func (t *trackedBool) String() string { return strconv.FormatBool(t.value) }

type trackedDuration struct {
	value time.Duration
	set   bool
}

func (t *trackedDuration) Set(value string) error {
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	t.value = parsed
	t.set = true
	return nil
}

func (t *trackedDuration) String() string { return t.value.String() }
