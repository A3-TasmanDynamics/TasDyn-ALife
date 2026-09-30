-- The Administration division: members administrate and maintain the
-- command panel. Only cabinet, Management and the division's Commander
-- appoint to it. Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-09-30_admin_division.sql

BEGIN;
ALTER TABLE faction_divisions ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;
CREATE UNIQUE INDEX IF NOT EXISTS idx_faction_divisions_admin ON faction_divisions(faction) WHERE is_admin;
-- Members can hold a specialist division and Administration at once.
ALTER TABLE faction_member_divisions DROP CONSTRAINT IF EXISTS faction_member_divisions_pkey;
ALTER TABLE faction_member_divisions ADD PRIMARY KEY (faction, player_id, division_key);
INSERT INTO faction_divisions (faction, key, name, color, roles, required_qual, min_level, sort, is_admin) VALUES
    ('police', 'ADMIN', 'Administration', '#fcd34d', ARRAY['Commander', 'Deputy Commander', 'Administrator'], NULL, 0, 0, true),
    ('ems',    'ADMIN', 'Administration', '#fcd34d', ARRAY['Commander', 'Deputy Commander', 'Administrator'], NULL, 0, 0, true)
ON CONFLICT DO NOTHING;
COMMIT;
