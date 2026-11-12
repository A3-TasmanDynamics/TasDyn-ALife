-- Website -> game commands: staff on the admin panel message, warn, kick
-- or announce to players in game. The website queues a row (after its own
-- permission check, and logs it to staff_log); the game server claims
-- pending rows every few seconds through the extension ("gamecmd"), runs
-- them and records the outcome. Unclaimed after 2 minutes = expired.
CREATE TABLE IF NOT EXISTS game_commands (
    id                BIGSERIAL PRIMARY KEY,
    action            TEXT NOT NULL CHECK (action IN ('message', 'warn', 'kick', 'announce')),
    target_player_id  BIGINT REFERENCES players(id) ON DELETE CASCADE,  -- NULL for announce
    text              TEXT NOT NULL CHECK (length(text) BETWEEN 1 AND 300),
    issued_by         BIGINT REFERENCES players(id) ON DELETE SET NULL,
    status            TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'claimed', 'done', 'failed', 'expired')),
    result            TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    claimed_at        TIMESTAMPTZ,
    done_at           TIMESTAMPTZ,
    CHECK ((action = 'announce') = (target_player_id IS NULL))
);
CREATE INDEX IF NOT EXISTS idx_game_commands_pending ON game_commands(id) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_game_commands_created ON game_commands(created_at DESC);
