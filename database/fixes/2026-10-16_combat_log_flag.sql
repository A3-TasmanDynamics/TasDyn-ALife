-- Anti-cheat flags: allow 'combat_log' (disconnecting while incapacitated,
-- recorded by the game server). Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-10-16_combat_log_flag.sql

BEGIN;
ALTER TABLE anti_cheat_flags DROP CONSTRAINT IF EXISTS anti_cheat_flags_flag_type_check;
ALTER TABLE anti_cheat_flags ADD CONSTRAINT anti_cheat_flags_flag_type_check
    CHECK (flag_type IN ('honeypot', 'movement', 'idempotency_reject', 'rate_limit', 'steam_ban', 'combat_log'));
COMMIT;
