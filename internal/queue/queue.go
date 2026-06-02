// Package queue wraps asynq (a Redis-backed job queue) for dispatching scan
// jobs. Producers (the API) enqueue a scan task; any app instance running a
// worker can pick it up. asynq owns delivery, retries with backoff, and
// dead-lettering, so the app needs no bespoke claim loop or reaper.
package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// TypeScan is the asynq task type for a scan job.
const TypeScan = "scan:run"

// QueueName is the single queue all scan tasks use.
const QueueName = "scans"

// ScanPayload is the task payload: just the scan id, since the durable request
// lives in Postgres.
type ScanPayload struct {
	ScanID string `json:"scan_id"`
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

// Worker runs an asynq server that processes scan tasks.
type Worker struct {
	srv *asynq.Server
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
	return &Worker{srv: srv}
}

// Handler is the function invoked for each scan task.
type Handler func(ctx context.Context, scanID string) error

// Run starts processing tasks, blocking until ctx is cancelled. The handler
// receives the scan id decoded from the task payload.
func (w *Worker) Run(ctx context.Context, h Handler) error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeScan, func(ctx context.Context, t *asynq.Task) error {
		var p ScanPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			// Malformed payload is not retryable.
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return h(ctx, p.ScanID)
	})

	if err := w.srv.Start(mux); err != nil {
		return err
	}
	<-ctx.Done()
	w.srv.Shutdown()
	return nil
}
