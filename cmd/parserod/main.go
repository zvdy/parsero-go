// Command parserod is the parsero SaaS server: an HTMX UI + REST API backed by
// Postgres (durable) and Redis (queue/cache/throttle). It also runs an in-process
// asynq worker, so a single binary both serves requests and processes scans;
// scale by running more instances behind a load balancer.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zvdy/parsero-go/internal/cache"
	"github.com/zvdy/parsero-go/internal/config"
	"github.com/zvdy/parsero-go/internal/jobs"
	"github.com/zvdy/parsero-go/internal/queue"
	"github.com/zvdy/parsero-go/internal/scheduler"
	"github.com/zvdy/parsero-go/internal/server"
	"github.com/zvdy/parsero-go/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Root context cancelled on SIGINT/SIGTERM for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Postgres: connect + migrate (advisory-locked, safe across instances).
	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(cfg.DatabaseURL); err != nil {
		return err
	}
	log.Println("database migrated")

	// Redis: cache + queue + throttle.
	c, err := cache.New(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer c.Close()

	qClient := queue.NewClient(c.Client())
	defer qClient.Close()

	instance, _ := os.Hostname()
	log.Printf("starting role=%s", cfg.Role)

	// Worker tier: process scan jobs + run scheduled scans.
	if cfg.RunsWorker() {
		proc := jobs.New(st, c, qClient, cfg, instance)
		worker := queue.NewWorker(c.Client(), cfg.WorkerCount)
		worker.HandleScan(proc.Handle)
		worker.HandleScheduled(proc.HandleScheduled)
		go func() {
			if err := worker.Run(ctx); err != nil {
				log.Printf("worker stopped: %v", err)
			}
		}()
		log.Printf("worker started (concurrency=%d)", cfg.WorkerCount)

		// Scheduler: turn DB schedules into periodic tasks. asynq.Unique guards
		// against duplicate ticks if more than one instance runs it.
		if cfg.SchedulerEnabled {
			sched, err := queue.NewScheduler(c.Client(), scheduler.NewProvider(st), cfg.SchedulerSync)
			if err != nil {
				return err
			}
			go func() {
				if err := sched.Run(ctx); err != nil {
					log.Printf("scheduler stopped: %v", err)
				}
			}()
			log.Println("scheduler started")
		}
	}

	// Web tier: HTTP server. A worker-only instance just blocks on ctx.
	if !cfg.RunsWeb() {
		log.Println("worker-only instance; not serving HTTP")
		<-ctx.Done()
		return nil
	}

	srv, err := server.New(cfg, st, c, qClient)
	if err != nil {
		return err
	}
	httpSrv := &http.Server{
		Addr:              announce(cfg.Port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		log.Println("shutting down…")
		shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	log.Printf("listening on :%s", cfg.Port)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func announce(port string) string { return ":" + port }
