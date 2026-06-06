// Package scheduler bridges DB-stored schedules to the asynq periodic-task
// manager: it reads enabled schedules from Postgres and hands them to asynq as
// cron-driven tasks. asynq polls GetConfigs periodically, so enabling, disabling,
// or deleting a schedule takes effect without a restart.
package scheduler

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/zvdy/parsero-go/internal/queue"
	"github.com/zvdy/parsero-go/internal/store"
)

// Provider implements asynq.PeriodicTaskConfigProvider backed by the store.
type Provider struct {
	store *store.Store
}

// NewProvider builds a Provider.
func NewProvider(st *store.Store) *Provider {
	return &Provider{store: st}
}

// GetConfigs returns one periodic task per enabled schedule. asynq.Unique
// collapses duplicate enqueues within the window so multiple scheduler instances
// can't double-fire a tick.
func (p *Provider) GetConfigs() ([]*asynq.PeriodicTaskConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	schedules, err := p.store.ListEnabledSchedules(ctx)
	if err != nil {
		return nil, err
	}

	var configs []*asynq.PeriodicTaskConfig
	for _, sc := range schedules {
		task, err := queue.NewScheduledTask(sc.ID)
		if err != nil {
			return nil, err
		}
		configs = append(configs, &asynq.PeriodicTaskConfig{
			Cronspec: sc.Cron,
			Task:     task,
			Opts: []asynq.Option{
				asynq.Queue(queue.QueueName),
				asynq.Unique(55 * time.Second),
			},
		})
	}
	return configs, nil
}
