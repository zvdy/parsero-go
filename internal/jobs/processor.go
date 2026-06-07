// Package jobs contains the asynq task handler that actually runs a scan:
// load the request from Postgres, run the SSRF-guarded scanner, persist results
// and summary, populate the Redis cache, and release the throttle slot.
package jobs

import (
	"context"
	"log"
	"time"

	"github.com/zvdy/parsero-go/internal/cache"
	"github.com/zvdy/parsero-go/internal/config"
	"github.com/zvdy/parsero-go/internal/diff"
	"github.com/zvdy/parsero-go/internal/notify"
	"github.com/zvdy/parsero-go/internal/queue"
	"github.com/zvdy/parsero-go/internal/safety"
	"github.com/zvdy/parsero-go/internal/scanner"
	"github.com/zvdy/parsero-go/internal/store"
	"github.com/zvdy/parsero-go/pkg/types"
)

type Processor struct {
	store    *store.Store
	cache    *cache.Cache
	queue    *queue.Client
	notifier *notify.Notifier
	cfg      config.Config
	instance string // recorded as locked_by for audit
}

func New(st *store.Store, c *cache.Cache, q *queue.Client, cfg config.Config, instance string) *Processor {
	return &Processor{
		store:    st,
		cache:    c,
		queue:    q,
		notifier: notify.New(),
		cfg:      cfg,
		instance: instance,
	}
}

// HandleScheduled creates a fresh scan per cron tick — trigger=scheduled,
// bypassing the result cache so changes are detected — and enqueues it.
func (p *Processor) HandleScheduled(ctx context.Context, scheduleID string) error {
	sch, err := p.store.GetSchedule(ctx, scheduleID)
	if err != nil {
		return err
	}
	if !sch.Enabled {
		return nil
	}
	id, err := p.store.CreateScan(ctx, store.Scan{
		UserID:      sch.UserID,
		Target:      sch.Target,
		OptionsHash: sch.OptionsHash,
		Only200:     sch.Only200,
		SearchBing:  sch.SearchBing,
		ScheduleID:  &sch.ID,
		Trigger:     "scheduled",
	})
	if err != nil {
		return err
	}
	_ = p.store.MarkScheduleRun(ctx, scheduleID)
	return p.queue.Enqueue(ctx, id)
}

// Handle runs a scan job. A returned error triggers asynq's retry; terminal
// failures are persisted as 'failed' instead (see fail).
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

	// For scheduled scans, diff against the previous run and alert on changes.
	if sc.ScheduleID != nil {
		p.diffAndNotify(ctx, sc, results)
	}
	return nil
}

// diffAndNotify alerts on changes vs the previous run, per the schedule's
// settings. Best-effort: failures here never fail the scan.
func (p *Processor) diffAndNotify(ctx context.Context, sc store.Scan, results []types.Result) {
	sch, err := p.store.GetSchedule(ctx, *sc.ScheduleID)
	if err != nil || sch.NotifyWebhook == "" {
		return
	}

	prev, err := p.store.PreviousDoneScan(ctx, sc.OptionsHash, sc.ID)
	if err != nil {
		return // no prior scan to diff against (first run) — nothing to alert
	}
	prevRows, err := p.store.ListResults(ctx, prev.ID)
	if err != nil {
		return
	}

	d := diff.Compute(toProbes(prevRows), probesFromResults(results))
	if sch.NotifyOnChange && !d.HasChanges() {
		return
	}

	alert := notify.Alert{
		Event:             "scan.changed",
		Target:            sc.Target,
		ScanID:            sc.ID,
		ScheduleID:        sch.ID,
		NewlyReachable:    d.NewlyReachable,
		NoLongerReachable: d.NoLongerReachable,
	}
	if err := p.notifier.Send(ctx, sch.NotifyWebhook, alert); err != nil {
		log.Printf("notify schedule %s: %v", sch.ID, err)
	}
}

func toProbes(rows []store.ResultRow) []diff.Probe {
	out := make([]diff.Probe, len(rows))
	for i, r := range rows {
		out[i] = diff.Probe{URL: r.URL, StatusCode: r.StatusCode}
	}
	return out
}

func probesFromResults(results []types.Result) []diff.Probe {
	out := make([]diff.Probe, 0, len(results))
	for _, r := range results {
		out = append(out, diff.Probe{URL: r.URL, StatusCode: r.StatusCode})
	}
	return out
}

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
