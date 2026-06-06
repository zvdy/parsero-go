package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Schedule is a recurring scan monitor for a target.
type Schedule struct {
	ID             string
	UserID         string
	Target         string
	OptionsHash    string
	Only200        bool
	SearchBing     bool
	Cron           string
	Enabled        bool
	NotifyWebhook  string
	NotifyOnChange bool
	CreatedAt      time.Time
	LastRunAt      *time.Time
}

// CreateSchedule inserts a new schedule and returns its id.
func (s *Store) CreateSchedule(ctx context.Context, sc Schedule) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO schedules (user_id, target, options_hash, only200, search_bing, cron, enabled, notify_webhook, notify_on_change)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`,
		sc.UserID, sc.Target, sc.OptionsHash, sc.Only200, sc.SearchBing, sc.Cron,
		sc.Enabled, nullify(sc.NotifyWebhook), sc.NotifyOnChange,
	).Scan(&id)
	return id, err
}

const scheduleCols = `id, user_id, target, options_hash, only200, search_bing, cron,
	enabled, COALESCE(notify_webhook, ''), notify_on_change, created_at, last_run_at`

func scanSchedule(row pgx.Row) (Schedule, error) {
	var sc Schedule
	err := row.Scan(
		&sc.ID, &sc.UserID, &sc.Target, &sc.OptionsHash, &sc.Only200, &sc.SearchBing,
		&sc.Cron, &sc.Enabled, &sc.NotifyWebhook, &sc.NotifyOnChange, &sc.CreatedAt, &sc.LastRunAt,
	)
	return sc, err
}

// GetSchedule loads a schedule by id.
func (s *Store) GetSchedule(ctx context.Context, id string) (Schedule, error) {
	sc, err := scanSchedule(s.pool.QueryRow(ctx, `SELECT `+scheduleCols+` FROM schedules WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Schedule{}, ErrNotFound
	}
	return sc, err
}

// ListSchedulesByUser returns a user's schedules, newest first.
func (s *Store) ListSchedulesByUser(ctx context.Context, userID string) ([]Schedule, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+scheduleCols+` FROM schedules WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSchedules(rows)
}

// ListEnabledSchedules returns all enabled schedules (for the scheduler).
func (s *Store) ListEnabledSchedules(ctx context.Context) ([]Schedule, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+scheduleCols+` FROM schedules WHERE enabled ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSchedules(rows)
}

func collectSchedules(rows pgx.Rows) ([]Schedule, error) {
	var out []Schedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// SetScheduleEnabled toggles a schedule on or off, scoped to its owner.
func (s *Store) SetScheduleEnabled(ctx context.Context, id, userID string, enabled bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE schedules SET enabled = $3 WHERE id = $1 AND user_id = $2`, id, userID, enabled)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkScheduleRun stamps last_run_at = now() for a schedule.
func (s *Store) MarkScheduleRun(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE schedules SET last_run_at = now() WHERE id = $1`, id)
	return err
}

// DeleteSchedule removes a schedule owned by userID.
func (s *Store) DeleteSchedule(ctx context.Context, id, userID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM schedules WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
