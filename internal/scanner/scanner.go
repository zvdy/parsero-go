// Package scanner is a stateless, context-aware robots.txt audit engine.
//
// Unlike the original internal/check + internal/search packages, it holds no
// package-level mutable state, never writes to stdout, and takes an injected
// *http.Client so callers (CLI, server jobs) can supply their own transport —
// e.g. an SSRF-guarded one for the multi-tenant service. This makes it safe to
// run many concurrent scans for different tenants in the same process.
package scanner

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/zvdy/parsero-go/pkg/types"
)

// Source values attached to results.
const (
	SourceRobots = "robots"
	SourceBing   = "bing"
)

// Options controls a single scan run.
type Options struct {
	Only200     bool // only retain HTTP 200 results
	SearchBing  bool // also discover paths via Bing search
	Concurrency int  // worker pool size for path probing
	MaxPaths    int  // cap on disallow paths processed (0 = unlimited)

	RobotsTimeout  time.Duration // per-request timeout for robots.txt fetch
	RequestTimeout time.Duration // per-request timeout for path probes
}

// withDefaults returns a copy of o with sensible fallbacks applied.
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

// Scanner runs robots.txt audits. Construct it with New and reuse it across
// scans; it carries no per-scan mutable state.
type Scanner struct {
	client   *http.Client
	opts     Options
	progress func(done, total int)
}

// New builds a Scanner. If client is nil a default client is used. The client's
// Transport is reused for all requests, so callers wanting SSRF protection
// should inject a guarded transport here.
func New(client *http.Client, opts Options) *Scanner {
	if client == nil {
		client = &http.Client{}
	}
	return &Scanner{client: client, opts: opts.withDefaults()}
}

// OnProgress registers a callback invoked as path probes complete. It is called
// from the result-collecting goroutine; keep it cheap and non-blocking.
func (s *Scanner) OnProgress(fn func(done, total int)) {
	s.progress = fn
}

// Run performs a full scan: fetch robots.txt, probe each disallow path, and
// optionally augment with Bing search. It returns the probe results, the list
// of disallow paths discovered, and an error only for fatal failures (e.g. no
// robots.txt). Per-path errors are reported inside the results slice.
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
