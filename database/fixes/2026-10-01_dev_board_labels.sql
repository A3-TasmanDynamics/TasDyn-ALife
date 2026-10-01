-- Project board: coloured labels. Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-10-01_dev_board_labels.sql

BEGIN;
-- Project board labels: a coloured, named tag (e.g. "Police", "Core
-- functions"). Cards keep label names in dev_tasks.labels.
CREATE TABLE IF NOT EXISTS dev_labels (
    name        TEXT PRIMARY KEY CHECK (length(name) BETWEEN 1 AND 32),
    color       TEXT NOT NULL DEFAULT 'blue'
                CHECK (color IN ('green', 'yellow', 'orange', 'red', 'purple', 'blue', 'sky', 'lime', 'pink', 'grey')),
    sort        DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dev_labels_lower_name ON dev_labels (lower(name));
COMMIT;
