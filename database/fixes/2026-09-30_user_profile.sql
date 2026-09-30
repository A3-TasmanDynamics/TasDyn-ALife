-- User profile page: website display name and TeamSpeak link. Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-09-30_user_profile.sql

BEGIN;
ALTER TABLE players ADD COLUMN IF NOT EXISTS display_name TEXT;
ALTER TABLE players ADD COLUMN IF NOT EXISTS display_name_changed_at TIMESTAMPTZ;
ALTER TABLE players ADD COLUMN IF NOT EXISTS teamspeak_uid TEXT;
ALTER TABLE players ADD COLUMN IF NOT EXISTS teamspeak_linked_at TIMESTAMPTZ;
ALTER TABLE players DROP CONSTRAINT IF EXISTS players_display_name_check;
ALTER TABLE players ADD CONSTRAINT players_display_name_check CHECK (display_name IS NULL OR length(display_name) BETWEEN 3 AND 24);
ALTER TABLE players DROP CONSTRAINT IF EXISTS players_teamspeak_uid_key;
ALTER TABLE players ADD CONSTRAINT players_teamspeak_uid_key UNIQUE (teamspeak_uid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_players_display_name ON players (lower(display_name)) WHERE display_name IS NOT NULL;
COMMIT;
