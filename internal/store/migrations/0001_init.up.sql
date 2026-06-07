-- Initial schema for the parsero SaaS: durable scans and per-path results.
-- Postgres is the source of truth; Redis holds the queue, cache, and throttle
-- counters. The job-claim columns (locked_by/locked_at/attempts) are retained
-- for audit/reaper even though asynq now owns delivery + retries.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$ BEGIN
    CREATE TYPE job_status AS ENUM ('queued', 'running', 'done', 'failed');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS scans (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          TEXT NOT NULL,
    target           TEXT NOT NULL,
    options_hash     TEXT NOT NULL,
    only200          BOOLEAN NOT NULL DEFAULT false,
    search_bing      BOOLEAN NOT NULL DEFAULT false,
    status           job_status NOT NULL DEFAULT 'queued',
    duration_seconds DOUBLE PRECISION,
    total_paths      INTEGER NOT NULL DEFAULT 0,
    status_200       INTEGER NOT NULL DEFAULT 0,
    other_status     INTEGER NOT NULL DEFAULT 0,
    errors           INTEGER NOT NULL DEFAULT 0,
    error_message    TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at       TIMESTAMPTZ,
    finished_at      TIMESTAMPTZ,
    locked_by        TEXT,
    locked_at        TIMESTAMPTZ,
    attempts         INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS scan_results (
    id          BIGSERIAL PRIMARY KEY,
    scan_id     UUID NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
    url         TEXT NOT NULL,
    status_code INTEGER,
    status      TEXT,
    error       TEXT,
    source      TEXT NOT NULL DEFAULT 'robots'
);

CREATE INDEX IF NOT EXISTS idx_scans_user_created ON scans (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_scans_cache ON scans (options_hash, status, finished_at DESC);
CREATE INDEX IF NOT EXISTS idx_scans_queue ON scans (status, created_at) WHERE status IN ('queued', 'running');
CREATE INDEX IF NOT EXISTS idx_scan_results_scan ON scan_results (scan_id);
