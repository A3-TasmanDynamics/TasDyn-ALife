-- Which faction each game session was played as (Admin -> Online now). The
-- extension's "session" "start" records it from the mission. Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-11-12_session_faction.sql

ALTER TABLE player_sessions ADD COLUMN IF NOT EXISTS faction TEXT
    CHECK (faction IS NULL OR faction IN ('civ', 'cop', 'medic'));
CREATE INDEX IF NOT EXISTS idx_player_sessions_open ON player_sessions(player_id) WHERE disconnected_at IS NULL;
