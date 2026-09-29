-- Faction rank overrides from Player Lookup become Management-only
-- (players.edit_police / players.edit_medic): day-to-day faction rank
-- changes belong to faction command. Removes the two keys from every rank
-- below Head Admin (level 100) and from per-player overrides (they're now
-- NoOverride keys). Also adds the faction command columns and faction_log.
-- Safe to run more than once.
BEGIN;

DELETE FROM rank_permissions rp
USING staff_ranks sr
WHERE rp.rank_id = sr.id AND sr.level < 100
  AND rp.command_key IN ('players.edit_police', 'players.edit_medic');

DELETE FROM staff_permission_overrides
WHERE command_key IN ('players.edit_police', 'players.edit_medic');

ALTER TABLE faction_rank_names ADD COLUMN IF NOT EXISTS short_name TEXT;
ALTER TABLE faction_rank_names ADD COLUMN IF NOT EXISTS slots INTEGER CHECK (slots > 0);
ALTER TABLE faction_rank_names ADD COLUMN IF NOT EXISTS promote_up_to INTEGER NOT NULL DEFAULT 0
    CHECK (promote_up_to >= 0 AND promote_up_to < level);

CREATE TABLE IF NOT EXISTS faction_log (
    id           BIGSERIAL PRIMARY KEY,
    faction      TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    actor_id     BIGINT REFERENCES players(id) ON DELETE SET NULL,
    target_id    BIGINT REFERENCES players(id) ON DELETE SET NULL,
    kind         TEXT NOT NULL CHECK (kind IN ('recruit', 'promote', 'demote', 'remove')),
    from_level   INTEGER NOT NULL,
    to_level     INTEGER NOT NULL,
    reason       TEXT NOT NULL,
    via          TEXT NOT NULL CHECK (via IN ('command', 'staff_override')),
    own_faction  BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_faction_log_faction ON faction_log(faction, id DESC);
CREATE INDEX IF NOT EXISTS idx_faction_log_target ON faction_log(target_id, id DESC);

COMMIT;
