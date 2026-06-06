// Package store is the Postgres persistence layer — the durable source of
// truth for scans and their per-path results. Redis (see internal/cache and
// internal/queue) sits in front for queueing and caching, but everything here
// survives a restart.
package store

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// scheme
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Migrate applies embedded up-migrations under an advisory lock, so it's safe to
// run on every instance at startup.
func (s *Store) Migrate(databaseURL string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, normalizeDSN(databaseURL))
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// normalizeDSN rewrites the URL to the pgx5 scheme golang-migrate expects.
func normalizeDSN(url string) string {
	const pg = "postgres://"
	const pgql = "postgresql://"
	if len(url) >= len(pgql) && url[:len(pgql)] == pgql {
		return "pgx5://" + url[len(pgql):]
	}
	if len(url) >= len(pg) && url[:len(pg)] == pg {
		return "pgx5://" + url[len(pg):]
	}
	return url
}

// OptionsHash is the cache key for a scan request; identical requests share it.
func OptionsHash(target string, only200, searchBing bool) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%t|%t", target, only200, searchBing)))
	return hex.EncodeToString(sum[:])
}

type Scan struct {
	ID              string
	UserID          string
	Target          string
	OptionsHash     string
	Only200         bool
	SearchBing      bool
	Status          string
	DurationSeconds float64
	TotalPaths      int
	Status200       int
	OtherStatus     int
	Errors          int
	ErrorMessage    string
	CreatedAt       time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	ScheduleID      *string
	Trigger         string
}

type ResultRow struct {
	URL        string
	StatusCode int
	Status     string
	Error      string
	Source     string
}
