// Package queue wraps asynq (a Redis-backed job queue) for dispatching scan
// jobs. Producers (the API) enqueue a scan task; any app instance running a
// worker can pick it up. asynq owns delivery, retries with backoff, and
// dead-lettering, so the app needs no bespoke claim loop or reaper.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// Task types.
const (
	// TypeScan is the asynq task type for a one-off scan job.
	TypeScan = "scan:run"
	// TypeScheduledScan fires on a schedule's cron tick; its handler creates and
	// enqueues a fresh scan for the schedule.
	TypeScheduledScan = "scan:scheduled"
)

// QueueName is the single queue all scan tasks use.
const QueueName = "scans"

// ScanPayload is the task payload: just the scan id, since the durable request
// lives in Postgres.
type ScanPayload struct {
	ScanID string `json:"scan_id"`
}

// ScheduledPayload carries the schedule id for a scheduled-scan tick.
type ScheduledPayload struct {
	ScheduleID string `json:"schedule_id"`
}

// NewScheduledTask builds a periodic task for a schedule. asynq.Unique collapses
// duplicate enqueues within the window, so it's safe even if more than one
// scheduler instance is briefly running.
func NewScheduledTask(scheduleID string) (*asynq.Task, error) {
	payload, err := json.Marshal(ScheduledPayload{ScheduleID: scheduleID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeScheduledScan, payload), nil
}

// Client enqueues scan tasks and inspects queue depth for backpressure.
type Client struct {
	client    *asynq.Client
	inspector *asynq.Inspector
}

// redisOpt adapts a go-redis client's options to asynq's connection options so
// both share the same Redis without re-parsing URLs.
func redisOpt(rdb *redis.Client) asynq.RedisClientOpt {
	o := rdb.Options()
	return asynq.RedisClientOpt{
		Addr:     o.Addr,
		DB:       o.DB,
		Username: o.Username,
		Password: o.Password,
	}
}

// NewClient builds an enqueue client sharing the given Redis connection.
func NewClient(rdb *redis.Client) *Client {
	opt := redisOpt(rdb)
	return &Client{
		client:    asynq.NewClient(opt),
		inspector: asynq.NewInspector(opt),
	}
}

// Close releases the client resources.
func (c *Client) Close() error {
	c.inspector.Close()
	return c.client.Close()
}

// Enqueue submits a scan task for the given scan id.
func (c *Client) Enqueue(ctx context.Context, scanID string) error {
	payload, err := json.Marshal(ScanPayload{ScanID: scanID})
	if err != nil {
		return err
	}
	_, err = c.client.EnqueueContext(ctx, asynq.NewTask(TypeScan, payload), asynq.Queue(QueueName))
	return err
}

// Depth returns the number of pending + active tasks in the scan queue. Used as
// the backpressure signal before accepting new work.
func (c *Client) Depth() (int, error) {
	info, err := c.inspector.GetQueueInfo(QueueName)
	if err != nil {
		return 0, err
	}
	return info.Pending + info.Active, nil
}

// ConfigProvider supplies the current set of periodic schedules to the
// Scheduler. It is polled periodically so DB changes (new/removed/toggled
// schedules) take effect without a restart.
type ConfigProvider = asynq.PeriodicTaskConfigProvider

// Scheduler turns DB-backed schedules into periodic asynq tasks. It wraps
// asynq's PeriodicTaskManager, which syncs cron entries from the provider.
type Scheduler struct {
	mgr *asynq.PeriodicTaskManager
}

// NewScheduler builds a Scheduler that re-reads the provider every syncInterval.
func NewScheduler(rdb *redis.Client, provider ConfigProvider, syncInterval time.Duration) (*Scheduler, error) {
	if syncInterval <= 0 {
		syncInterval = time.Minute
	}
	mgr, err := asynq.NewPeriodicTaskManager(asynq.PeriodicTaskManagerOpts{
		RedisConnOpt:               redisOpt(rdb),
		PeriodicTaskConfigProvider: provider,
		SyncInterval:               syncInterval,
		SchedulerOpts: &asynq.SchedulerOpts{
			Location: time.UTC,
		},
	})
	if err != nil {
		return nil, err
	}
	return &Scheduler{mgr: mgr}, nil
}

// Run starts the scheduler, blocking until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	if err := s.mgr.Start(); err != nil {
		return err
	}
	<-ctx.Done()
	s.mgr.Shutdown()
	return nil
}

// Worker runs an asynq server that processes scan tasks.
type Worker struct {
	srv *asynq.Server
	mux *asynq.ServeMux
}

// NewWorker builds a worker with the given per-instance concurrency.
func NewWorker(rdb *redis.Client, concurrency int) *Worker {
	if concurrency <= 0 {
		concurrency = 4
	}
	srv := asynq.NewServer(redisOpt(rdb), asynq.Config{
		Concurrency: concurrency,
		Queues:      map[string]int{QueueName: 1},
	})
	return &Worker{srv: srv, mux: asynq.NewServeMux()}
}

// Handler is the function invoked for each scan task.
type Handler func(ctx context.Context, scanID string) error

// ScheduledHandler is invoked for each scheduled-scan tick; it receives the
// schedule id and is responsible for creating + enqueuing a fresh scan.
type ScheduledHandler func(ctx context.Context, scheduleID string) error

// HandleScan registers the handler for one-off scan tasks.
func (w *Worker) HandleScan(h Handler) {
	w.mux.HandleFunc(TypeScan, func(ctx context.Context, t *asynq.Task) error {
		var p ScanPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err) // malformed payload: don't retry
		}
		return h(ctx, p.ScanID)
	})
}

// HandleScheduled registers the handler for scheduled-scan ticks.
func (w *Worker) HandleScheduled(h ScheduledHandler) {
	w.mux.HandleFunc(TypeScheduledScan, func(ctx context.Context, t *asynq.Task) error {
		var p ScheduledPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return h(ctx, p.ScheduleID)
	})
}

// Run starts processing registered tasks, blocking until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	if err := w.srv.Start(w.mux); err != nil {
		return err
	}
	<-ctx.Done()
	w.srv.Shutdown()
	return nil
}
