// Package scanner is a stateless, context-aware robots.txt audit engine: no
// package-level state, no stdout, and an injected *http.Client so callers can
// supply an SSRF-guarded transport for multi-tenant use.
package scanner

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/zvdy/parsero-go/pkg/types"
)

const (
	SourceRobots = "robots"
	SourceBing   = "bing"
)

type Options struct {
	Only200     bool
	SearchBing  bool
	Concurrency int
	MaxPaths    int // 0 = unlimited

	RobotsTimeout  time.Duration
	RequestTimeout time.Duration
}

func (o Options) withDefaults() Options {
	if o.Concurrency <= 0 {
		o.Concurrency = runtime.NumCPU()
	}
	if o.RobotsTimeout <= 0 {
		o.RobotsTimeout = 5 * time.Second
	}
	if o.RequestTimeout <= 0 {
		o.RequestTimeout = 3 * time.Second
	}
	return o
}

// RobotsCache lets bursts of scans on the same target skip the robots fetch.
// Implementations must be safe for concurrent use; a nil cache disables caching.
type RobotsCache interface {
	GetRobots(ctx context.Context, target string) ([]string, bool)
	SetRobots(ctx context.Context, target string, paths []string, ttl time.Duration)
}

// Scanner carries no per-scan state and is safe to reuse across scans.
type Scanner struct {
	client      *http.Client
	opts        Options
	progress    func(done, total int)
	robotsCache RobotsCache
	robotsTTL   time.Duration
}

// New builds a Scanner. The client's Transport is reused for every request, so
// callers wanting SSRF protection inject a guarded transport here.
func New(client *http.Client, opts Options) *Scanner {
	if client == nil {
		client = &http.Client{}
	}
	return &Scanner{client: client, opts: opts.withDefaults()}
}

func (s *Scanner) SetRobotsCache(c RobotsCache, ttl time.Duration) {
	s.robotsCache = c
	s.robotsTTL = ttl
}

func (s *Scanner) OnProgress(fn func(done, total int)) {
	s.progress = fn
}

// Run fetches robots.txt, probes each disallow path, and optionally augments with
// Bing. err is non-nil only for fatal failures (e.g. no robots.txt); per-path
// errors live in the results slice.
func (s *Scanner) Run(ctx context.Context, target string) (results []types.Result, disallow []string, err error) {
	disallow, err = s.FetchDisallowPaths(ctx, target)
	if err != nil {
		return nil, nil, err
	}
	if len(disallow) == 0 {
		return nil, nil, nil
	}

	results = s.CheckPaths(ctx, target, disallow)

	if s.opts.SearchBing {
		results = append(results, s.searchBing(ctx, target, disallow)...)
	}
	return results, disallow, nil
}
