// Package config loads server configuration from the environment, with
// sensible defaults so the service runs out of the box for local development.
package config

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"
)

// Config holds all tunables for the parserod server.
type Config struct {
	Port        string
	DatabaseURL string
	RedisURL    string

	ScanCacheTTL   time.Duration
	RobotsCacheTTL time.Duration
	ScanTimeout    time.Duration
	JobStaleAfter  time.Duration

	MaxPaths      int
	WorkerCount   int // per-instance asynq concurrency
	MaxInflight   int // global in-flight scan cap (0 = unlimited)
	MaxPerUser    int // per-user concurrent/queued cap (0 = unlimited)
	MaxQueueDepth int // backpressure threshold (0 = unlimited)

	RateLimitRPS   float64
	RateLimitBurst int

	IdentityHeader     string
	DefaultConcurrency int
	BingEnabled        bool

	// Role selects what this instance runs: "web" (HTTP only), "worker" (job
	// processing + scheduler only), or "all" (both). Splitting roles lets the
	// web tier and worker tier scale independently (see the Helm chart).
	Role             string
	SchedulerEnabled bool          // run the periodic-scan scheduler on this instance
	SchedulerSync    time.Duration // how often the scheduler re-reads schedules from the DB
}

// RunsWeb reports whether this instance should serve HTTP.
func (c Config) RunsWeb() bool { return c.Role == "web" || c.Role == "all" }

// RunsWorker reports whether this instance should process jobs.
func (c Config) RunsWorker() bool { return c.Role == "worker" || c.Role == "all" }

// Load reads configuration from the environment.
func Load() (Config, error) {
	c := Config{
		Port:               getStr("PORT", "8080"),
		DatabaseURL:        getStr("DATABASE_URL", "postgres://parsero:parsero@localhost:5432/parsero?sslmode=disable"),
		RedisURL:           getStr("REDIS_URL", "redis://localhost:6379/0"),
		ScanCacheTTL:       getDur("SCAN_CACHE_TTL", 10*time.Minute),
		RobotsCacheTTL:     getDur("ROBOTS_CACHE_TTL", 5*time.Minute),
		ScanTimeout:        getDur("SCAN_TIMEOUT", 120*time.Second),
		JobStaleAfter:      getDur("JOB_STALE_TIMEOUT", 5*time.Minute),
		MaxPaths:           getInt("MAX_PATHS", 500),
		WorkerCount:        getInt("WORKER_COUNT", 4),
		MaxInflight:        getInt("MAX_INFLIGHT", 50),
		MaxPerUser:         getInt("MAX_PER_USER", 2),
		MaxQueueDepth:      getInt("MAX_QUEUE_DEPTH", 100),
		RateLimitRPS:       getFloat("RATE_LIMIT_RPS", 5),
		RateLimitBurst:     getInt("RATE_LIMIT_BURST", 10),
		IdentityHeader:     getStr("IDENTITY_HEADER", "X-Auth-Request-Email"),
		DefaultConcurrency: getInt("DEFAULT_CONCURRENCY", runtime.NumCPU()),
		BingEnabled:        getBool("BING_ENABLED", false),
		Role:               getStr("ROLE", "all"),
		SchedulerEnabled:   getBool("SCHEDULER_ENABLED", true),
		SchedulerSync:      getDur("SCHEDULER_SYNC", time.Minute),
	}
	switch c.Role {
	case "web", "worker", "all":
	default:
		return c, fmt.Errorf("invalid ROLE %q (want web|worker|all)", c.Role)
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		return c, fmt.Errorf("REDIS_URL is required")
	}
	return c, nil
}

func getStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getDur(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
