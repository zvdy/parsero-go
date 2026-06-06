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

const (
	TypeScan          = "scan:run"
	TypeScheduledScan = "scan:scheduled" // fires on a schedule's cron tick
	QueueName         = "scans"
)

// Payloads carry only ids; the durable records live in Postgres.
type ScanPayload struct {
	ScanID string `json:"scan_id"`
}

type ScheduledPayload struct {
	ScheduleID string `json:"schedule_id"`
}

func NewScheduledTask(scheduleID string) (*asynq.Task, error) {
	payload, err := json.Marshal(ScheduledPayload{ScheduleID: scheduleID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeScheduledScan, payload), nil
}

type Client struct {
	client    *asynq.Client
	inspector *asynq.Inspector
}

// redisOpt reuses a go-redis client's connection settings for asynq.
func redisOpt(rdb *redis.Client) asynq.RedisClientOpt {
	o := rdb.Options()
	return asynq.RedisClientOpt{
		Addr:     o.Addr,
		DB:       o.DB,
		Username: o.Username,
		Password: o.Password,
	}
}

func NewClient(rdb *redis.Client) *Client {
	opt := redisOpt(rdb)
	return &Client{
		client:    asynq.NewClient(opt),
		inspector: asynq.NewInspector(opt),
	}
}

func (c *Client) Close() error {
	c.inspector.Close()
	return c.client.Close()
}

func (c *Client) Enqueue(ctx context.Context, scanID string) error {
	payload, err := json.Marshal(ScanPayload{ScanID: scanID})
	if err != nil {
		return err
	}
	_, err = c.client.EnqueueContext(ctx, asynq.NewTask(TypeScan, payload), asynq.Queue(QueueName))
	return err
}

// Depth (pending + active) is the backpressure signal before accepting work.
func (c *Client) Depth() (int, error) {
	info, err := c.inspector.GetQueueInfo(QueueName)
	if err != nil {
		return 0, err
	}
	return info.Pending + info.Active, nil
}

// ConfigProvider is polled by the Scheduler so DB schedule changes take effect
// without a restart.
type ConfigProvider = asynq.PeriodicTaskConfigProvider

type Scheduler struct {
	mgr *asynq.PeriodicTaskManager
}

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

func (s *Scheduler) Run(ctx context.Context) error {
	if err := s.mgr.Start(); err != nil {
		return err
	}
	<-ctx.Done()
	s.mgr.Shutdown()
	return nil
}

type Worker struct {
	srv *asynq.Server
	mux *asynq.ServeMux
}

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

type Handler func(ctx context.Context, scanID string) error

type ScheduledHandler func(ctx context.Context, scheduleID string) error

func (w *Worker) HandleScan(h Handler) {
	w.mux.HandleFunc(TypeScan, func(ctx context.Context, t *asynq.Task) error {
		var p ScanPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err) // malformed payload: don't retry
		}
		return h(ctx, p.ScanID)
	})
}

func (w *Worker) HandleScheduled(h ScheduledHandler) {
	w.mux.HandleFunc(TypeScheduledScan, func(ctx context.Context, t *asynq.Task) error {
		var p ScheduledPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return h(ctx, p.ScheduleID)
	})
}

func (w *Worker) Run(ctx context.Context) error {
	if err := w.srv.Start(w.mux); err != nil {
		return err
	}
	<-ctx.Done()
	w.srv.Shutdown()
	return nil
}
