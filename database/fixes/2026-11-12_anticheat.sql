-- Anti-cheat: the game server's new flags (ALife_fnc_anticheat) and the
-- gameplay event log it writes to (player_log). Safe to re-run.
--
--   'cheat_state'     : god mode / invisible / sped up, not staff
--   'gear'            : gear no shop or starting kit gives
--   'dupe'            : a sudden pile of weapons / attachments nobody bought
--   'invalid_request' : a request sent for someone else's character
--   (plus 'movement' and 'rate_limit', which already existed)
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-11-12_anticheat.sql

BEGIN;
ALTER TABLE anti_cheat_flags DROP CONSTRAINT IF EXISTS anti_cheat_flags_flag_type_check;
ALTER TABLE anti_cheat_flags ADD CONSTRAINT anti_cheat_flags_flag_type_check
    CHECK (flag_type IN ('honeypot', 'movement', 'idempotency_reject', 'rate_limit', 'steam_ban', 'combat_log',
                         'cheat_state', 'gear', 'dupe', 'invalid_request'));
-- Staff search the event log by player and time, and by kind.
CREATE INDEX IF NOT EXISTS idx_player_log_player_time ON player_log(player_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_player_log_event_time ON player_log(event_type, created_at DESC);
COMMIT;
