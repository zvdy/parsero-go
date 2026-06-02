// Package jobs contains the asynq task handler that actually runs a scan:
// load the request from Postgres, run the SSRF-guarded scanner, persist results
// and summary, populate the Redis cache, and release the throttle slot.
package jobs

import (
	"context"
	"time"

	"github.com/zvdy/parsero-go/internal/cache"
	"github.com/zvdy/parsero-go/internal/config"
	"github.com/zvdy/parsero-go/internal/safety"
	"github.com/zvdy/parsero-go/internal/scanner"
	"github.com/zvdy/parsero-go/internal/store"
	"github.com/zvdy/parsero-go/pkg/types"
)

// Processor handles scan tasks.
type Processor struct {
	store    *store.Store
	cache    *cache.Cache
	cfg      config.Config
	instance string // identifier recorded as locked_by for audit
}

// New builds a Processor.
func New(st *store.Store, c *cache.Cache, cfg config.Config, instance string) *Processor {
	return &Processor{store: st, cache: c, cfg: cfg, instance: instance}
}

// Handle runs a single scan job. It is the asynq Handler. Errors returned here
// trigger asynq's retry; once retries are exhausted asynq drops the task, so we
// also persist a failed status for visibility.
func (p *Processor) Handle(ctx context.Context, scanID string) (err error) {
	sc, err := p.store.GetScan(ctx, scanID)
	if err != nil {
		return err
	}

	// Always release the throttle slot this job holds, even on panic/timeout.
	defer p.cache.Release(context.Background(), sc.UserID)

	// Already finished (e.g. duplicate delivery)? Nothing to do.
	if sc.Status == "done" || sc.Status == "failed" {
		return nil
	}

	if err := p.store.MarkRunning(ctx, scanID, p.instance); err != nil {
		return err
	}

	// Bound the scan and guard against SSRF via the dial-time-validated client.
	runCtx, cancel := context.WithTimeout(ctx, p.cfg.ScanTimeout)
	defer cancel()

	if err := safety.ResolveAndCheck(runCtx, sc.Target); err != nil {
		return p.fail(ctx, scanID, "target rejected: "+err.Error())
	}

	client := safety.GuardedClient(p.cfg.ScanTimeout)
	s := scanner.New(client, scanner.Options{
		Only200:     sc.Only200,
		SearchBing:  sc.SearchBing && p.cfg.BingEnabled,
		Concurrency: p.cfg.DefaultConcurrency,
		MaxPaths:    p.cfg.MaxPaths,
	})
	s.SetRobotsCache(p.cache, p.cfg.RobotsCacheTTL)
	s.OnProgress(func(done, total int) {
		p.cache.SetProgress(runCtx, scanID, done, total)
	})

	start := time.Now()
	results, disallow, err := s.Run(runCtx, sc.Target)
	if err != nil {
		return p.fail(ctx, scanID, err.Error())
	}
	p.cache.SetProgress(ctx, scanID, len(results), len(disallow))

	if err := p.persist(ctx, scanID, sc, results, time.Since(start)); err != nil {
		return err
	}

	// Populate the result cache so identical requests skip the queue.
	_ = p.cache.PutScanID(ctx, sc.OptionsHash, scanID, p.cfg.ScanCacheTTL)
	return nil
}

// persist writes per-path results and the scan summary to Postgres.
func (p *Processor) persist(ctx context.Context, scanID string, sc store.Scan, results []types.Result, dur time.Duration) error {
	rows := make([]store.ResultRow, 0, len(results))
	var status200, other, errs int
	for _, r := range results {
		row := store.ResultRow{
			URL:        r.URL,
			StatusCode: r.StatusCode,
			Status:     r.Status,
			Source:     r.Source,
		}
		if r.Error != nil {
			row.Error = r.Error.Error()
			errs++
		} else if r.StatusCode == 200 {
			status200++
		} else {
			other++
		}
		rows = append(rows, row)
	}

	if err := p.store.InsertResults(ctx, scanID, rows); err != nil {
		return err
	}

	sc.DurationSeconds = dur.Seconds()
	sc.TotalPaths = len(results)
	sc.Status200 = status200
	sc.OtherStatus = other
	sc.Errors = errs
	return p.store.CompleteScan(ctx, scanID, sc)
}

// fail records a terminal failure and swallows the error so asynq stops retrying
// a request that can never succeed (e.g. a blocked target).
func (p *Processor) fail(ctx context.Context, scanID, msg string) error {
	_ = p.store.FailScan(ctx, scanID, msg)
	return nil
}
