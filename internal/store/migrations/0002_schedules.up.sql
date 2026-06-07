-- Scheduled scans (recurring monitors) + diff/notification support.
-- A schedule periodically enqueues a fresh scan of a target; the worker diffs
-- each completed scheduled scan against the previous one to detect Disallow
-- paths that have *become reachable* (a security regression) and notifies.

CREATE TABLE IF NOT EXISTS schedules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         TEXT NOT NULL,
    target          TEXT NOT NULL,
    options_hash    TEXT NOT NULL,
    only200         BOOLEAN NOT NULL DEFAULT false,
    search_bing     BOOLEAN NOT NULL DEFAULT false,
    cron            TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    notify_webhook  TEXT,
    notify_on_change BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_run_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_schedules_user ON schedules (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_schedules_enabled ON schedules (enabled) WHERE enabled;

-- Link scans back to the schedule that spawned them (NULL for manual scans) and
-- record how each scan was triggered.
ALTER TABLE scans ADD COLUMN IF NOT EXISTS schedule_id UUID REFERENCES schedules(id) ON DELETE SET NULL;
ALTER TABLE scans ADD COLUMN IF NOT EXISTS trigger TEXT NOT NULL DEFAULT 'manual';

CREATE INDEX IF NOT EXISTS idx_scans_schedule ON scans (schedule_id, finished_at DESC);
