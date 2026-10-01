-- Project board: free-text tags beside the coloured labels. Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-10-02_dev_board_tags.sql

BEGIN;
-- Free-text tags on project board cards (e.g. "altis-life"), separate
-- from the coloured labels.
ALTER TABLE dev_tasks ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}';
COMMIT;
