package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// CreateScan inserts a new queued scan and returns its generated id.
func (s *Store) CreateScan(ctx context.Context, sc Scan) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO scans (user_id, target, options_hash, only200, search_bing, status)
		VALUES ($1, $2, $3, $4, $5, 'queued')
		RETURNING id`,
		sc.UserID, sc.Target, sc.OptionsHash, sc.Only200, sc.SearchBing,
	).Scan(&id)
	return id, err
}

// GetScan loads a single scan by id. Returns ErrNotFound if missing.
func (s *Store) GetScan(ctx context.Context, id string) (Scan, error) {
	var sc Scan
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, target, options_hash, only200, search_bing, status,
		       COALESCE(duration_seconds, 0), total_paths, status_200, other_status,
		       errors, COALESCE(error_message, ''), created_at, started_at, finished_at
		FROM scans WHERE id = $1`, id,
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

// ListScansByUser returns a user's scans, newest first, up to limit.
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

// FindCachedScan returns the newest completed scan with the given options_hash
// finished within ttl. Returns ErrNotFound when there's no fresh hit. Used as
// the Postgres fallback when the Redis cache misses.
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

// MarkRunning transitions a scan to running and stamps started_at + locker.
func (s *Store) MarkRunning(ctx context.Context, id, lockedBy string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE scans
		SET status = 'running', started_at = now(), locked_by = $2,
		    locked_at = now(), attempts = attempts + 1
		WHERE id = $1`, id, lockedBy)
	return err
}

// CompleteScan stores the summary and marks the scan done.
func (s *Store) CompleteScan(ctx context.Context, id string, sc Scan) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE scans
		SET status = 'done', finished_at = now(), duration_seconds = $2,
		    total_paths = $3, status_200 = $4, other_status = $5, errors = $6
		WHERE id = $1`,
		id, sc.DurationSeconds, sc.TotalPaths, sc.Status200, sc.OtherStatus, sc.Errors)
	return err
}

// FailScan marks the scan failed with an error message.
func (s *Store) FailScan(ctx context.Context, id, msg string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE scans
		SET status = 'failed', finished_at = now(), error_message = $2
		WHERE id = $1`, id, msg)
	return err
}
