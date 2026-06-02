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

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// Store wraps a pgx connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New opens a connection pool to the given Postgres URL.
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

// Close releases the connection pool.
func (s *Store) Close() { s.pool.Close() }

// Pool exposes the underlying pool for advanced callers (e.g. health checks).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Migrate applies all embedded up-migrations. golang-migrate holds an advisory
// lock for the duration, so running this on every instance at startup is safe.
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

// normalizeDSN ensures the URL uses the pgx5 scheme golang-migrate expects.
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

// OptionsHash derives the cache key for a scan request from its target and
// options. Identical requests share a hash and therefore a cached result.
func OptionsHash(target string, only200, searchBing bool) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%t|%t", target, only200, searchBing)))
	return hex.EncodeToString(sum[:])
}

// Scan is the durable record of a scan request and its summary.
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
}

// ResultRow is a single probed path belonging to a scan.
type ResultRow struct {
	URL        string
	StatusCode int
	Status     string
	Error      string
	Source     string
}
