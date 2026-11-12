-- Player Lookup's Connections tab (alt accounts, shared IPs). Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-11-12_connections.sql
--
-- 1. Name history: every name a player has used goes into player_aliases
--    (nothing wrote to it before). A trigger on players catches every way a
--    name changes; current names are backfilled.
-- 2. IP history: player_ips keeps each IP a player was seen on, and when.
--    web_sessions rows are deleted on logout, so the IP would otherwise be
--    lost. Filled by triggers on web_sessions (website logins) and
--    player_sessions (the game, if it ever records an IP); backfilled.

BEGIN;

CREATE OR REPLACE FUNCTION alife_record_alias() RETURNS trigger AS $$
BEGIN
    IF NEW.name IS NOT NULL AND btrim(NEW.name) <> '' THEN
        INSERT INTO player_aliases (player_id, name) VALUES (NEW.id, NEW.name)
        ON CONFLICT (player_id, name) DO UPDATE SET last_seen_at = now();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_players_alias ON players;
CREATE TRIGGER trg_players_alias AFTER INSERT OR UPDATE OF name ON players
    FOR EACH ROW EXECUTE FUNCTION alife_record_alias();

INSERT INTO player_aliases (player_id, name, first_seen_at, last_seen_at)
SELECT id, name, created_at, updated_at FROM players WHERE btrim(name) <> ''
ON CONFLICT (player_id, name) DO NOTHING;

CREATE TABLE IF NOT EXISTS player_ips (
    player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    ip_address  INET NOT NULL,
    source      TEXT NOT NULL CHECK (source IN ('website', 'game')),
    first_seen  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen   TIMESTAMPTZ NOT NULL DEFAULT now(),
    times_seen  INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (player_id, ip_address, source)
);
CREATE INDEX IF NOT EXISTS idx_player_ips_ip ON player_ips(ip_address);

CREATE OR REPLACE FUNCTION alife_record_ip() RETURNS trigger AS $$
BEGIN
    IF NEW.ip_address IS NOT NULL THEN
        INSERT INTO player_ips (player_id, ip_address, source)
        VALUES (NEW.player_id, NEW.ip_address, TG_ARGV[0])
        ON CONFLICT (player_id, ip_address, source)
        DO UPDATE SET last_seen = now(), times_seen = player_ips.times_seen + 1;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_web_sessions_ip ON web_sessions;
CREATE TRIGGER trg_web_sessions_ip AFTER INSERT ON web_sessions
    FOR EACH ROW EXECUTE FUNCTION alife_record_ip('website');
DROP TRIGGER IF EXISTS trg_player_sessions_ip ON player_sessions;
CREATE TRIGGER trg_player_sessions_ip AFTER INSERT OR UPDATE OF ip_address ON player_sessions
    FOR EACH ROW EXECUTE FUNCTION alife_record_ip('game');

INSERT INTO player_ips (player_id, ip_address, source, first_seen, last_seen, times_seen)
SELECT player_id, ip_address, 'website', min(created_at), max(last_seen_at), count(*)
FROM web_sessions WHERE ip_address IS NOT NULL GROUP BY player_id, ip_address
ON CONFLICT (player_id, ip_address, source) DO NOTHING;
INSERT INTO player_ips (player_id, ip_address, source, first_seen, last_seen, times_seen)
SELECT player_id, ip_address, 'game', min(connected_at), max(connected_at), count(*)
FROM player_sessions WHERE ip_address IS NOT NULL GROUP BY player_id, ip_address
ON CONFLICT (player_id, ip_address, source) DO NOTHING;

COMMIT;
