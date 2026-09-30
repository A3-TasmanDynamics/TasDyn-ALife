-- Division applications (faction members apply to join a division).
-- Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-09-30_division_applications.sql

BEGIN;

-- Division applications: a faction member applies to join a division.
-- Reviewed by Administration, the division's command (its top two roles,
-- or the Commander alone in a two-role division), cabinet and Management;
-- the Administration division is reviewed only by cabinet, Management and
-- its Commander. Accepting posts them at the division's entry role.
CREATE TABLE IF NOT EXISTS faction_division_applications (
    id             BIGSERIAL PRIMARY KEY,
    faction        TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    division_key   TEXT NOT NULL,
    player_id      BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    why            TEXT NOT NULL,
    availability   TEXT NOT NULL DEFAULT '',
    experience     TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'withdrawn')),
    decided_by     BIGINT REFERENCES players(id) ON DELETE SET NULL,
    decision_note  TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at     TIMESTAMPTZ,
    FOREIGN KEY (faction, division_key) REFERENCES faction_divisions(faction, key) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_faction_division_apps_open ON faction_division_applications(faction, division_key, player_id) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_faction_division_apps ON faction_division_applications(faction, division_key, id DESC);
COMMIT;
