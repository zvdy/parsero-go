package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateScan(ctx context.Context, sc Scan) (string, error) {
	trigger := sc.Trigger
	if trigger == "" {
		trigger = "manual"
	}
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO scans (user_id, target, options_hash, only200, search_bing, status, schedule_id, trigger)
		VALUES ($1, $2, $3, $4, $5, 'queued', $6, $7)
		RETURNING id`,
		sc.UserID, sc.Target, sc.OptionsHash, sc.Only200, sc.SearchBing, sc.ScheduleID, trigger,
	).Scan(&id)
	return id, err
}

// PreviousDoneScan returns the prior completed scan for the same options_hash
// (excluding excludeID), for diffing. ErrNotFound when there's no prior scan.
func (s *Store) PreviousDoneScan(ctx context.Context, optionsHash, excludeID string) (Scan, error) {
	var sc Scan
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, target, options_hash, only200, search_bing, status,
		       COALESCE(duration_seconds, 0), total_paths, status_200, other_status,
		       errors, COALESCE(error_message, ''), created_at, started_at, finished_at,
		       COALESCE(trigger, 'manual')
		FROM scans
		WHERE options_hash = $1 AND status = 'done' AND id <> $2
		ORDER BY finished_at DESC LIMIT 1`,
		optionsHash, excludeID,
	).Scan(
		&sc.ID, &sc.UserID, &sc.Target, &sc.OptionsHash, &sc.Only200, &sc.SearchBing,
		&sc.Status, &sc.DurationSeconds, &sc.TotalPaths, &sc.Status200, &sc.OtherStatus,
		&sc.Errors, &sc.ErrorMessage, &sc.CreatedAt, &sc.StartedAt, &sc.FinishedAt, &sc.Trigger,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Scan{}, ErrNotFound
	}
	return sc, err
}

func (s *Store) GetScan(ctx context.Context, id string) (Scan, error) {
	var sc Scan
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, target, options_hash, only200, search_bing, status,
		       COALESCE(duration_seconds, 0), total_paths, status_200, other_status,
		       errors, COALESCE(error_message, ''), created_at, started_at, finished_at,
		       schedule_id, COALESCE(trigger, 'manual')
		FROM scans WHERE id = $1`, id,
	).Scan(
		&sc.ID, &sc.UserID, &sc.Target, &sc.OptionsHash, &sc.Only200, &sc.SearchBing,
		&sc.Status, &sc.DurationSeconds, &sc.TotalPaths, &sc.Status200, &sc.OtherStatus,
		&sc.Errors, &sc.ErrorMessage, &sc.CreatedAt, &sc.StartedAt, &sc.FinishedAt,
		&sc.ScheduleID, &sc.Trigger,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Scan{}, ErrNotFound
	}
	return sc, err
}

func (s *Store) ListScansByUser(ctx context.Context, userID string, limit int) ([]Scan, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, target, options_hash, only200, search_bing, status,
		       COALESCE(duration_seconds, 0), total_paths, status_200, other_status,
		       errors, COALESCE(error_message, ''), created_at, started_at, finished_at
		FROM scans WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Scan
	for rows.Next() {
		var sc Scan
		if err := rows.Scan(
			&sc.ID, &sc.UserID, &sc.Target, &sc.OptionsHash, &sc.Only200, &sc.SearchBing,
			&sc.Status, &sc.DurationSeconds, &sc.TotalPaths, &sc.Status200, &sc.OtherStatus,
			&sc.Errors, &sc.ErrorMessage, &sc.CreatedAt, &sc.StartedAt, &sc.FinishedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// FindCachedScan is the Postgres fallback for the Redis result cache: the newest
// done scan for options_hash finished within ttl, else ErrNotFound.
func (s *Store) FindCachedScan(ctx context.Context, optionsHash string, ttl time.Duration) (Scan, error) {
	var sc Scan
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, target, options_hash, only200, search_bing, status,
		       COALESCE(duration_seconds, 0), total_paths, status_200, other_status,
		       errors, COALESCE(error_message, ''), created_at, started_at, finished_at
		FROM scans
		WHERE options_hash = $1 AND status = 'done' AND finished_at > now() - $2::interval
		ORDER BY finished_at DESC LIMIT 1`,
		optionsHash, ttl.String(),
	).Scan(
		&sc.ID, &sc.UserID, &sc.Target, &sc.OptionsHash, &sc.Only200, &sc.SearchBing,
		&sc.Status, &sc.DurationSeconds, &sc.TotalPaths, &sc.Status200, &sc.OtherStatus,
		&sc.Errors, &sc.ErrorMessage, &sc.CreatedAt, &sc.StartedAt, &sc.FinishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Scan{}, ErrNotFound
	}
	return sc, err
}

func (s *Store) MarkRunning(ctx context.Context, id, lockedBy string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE scans
		SET status = 'running', started_at = now(), locked_by = $2,
		    locked_at = now(), attempts = attempts + 1
		WHERE id = $1`, id, lockedBy)
	return err
}

func (s *Store) CompleteScan(ctx context.Context, id string, sc Scan) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE scans
		SET status = 'done', finished_at = now(), duration_seconds = $2,
		    total_paths = $3, status_200 = $4, other_status = $5, errors = $6
		WHERE id = $1`,
		id, sc.DurationSeconds, sc.TotalPaths, sc.Status200, sc.OtherStatus, sc.Errors)
	return err
}

func (s *Store) FailScan(ctx context.Context, id, msg string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE scans
		SET status = 'failed', finished_at = now(), error_message = $2
		WHERE id = $1`, id, msg)
	return err
}
