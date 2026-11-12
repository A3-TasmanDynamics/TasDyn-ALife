-- Staff watchlist: players staff want to keep an eye on. The admin panel
-- tags them everywhere, lists them on /admin/watchlist, and the Discord bot
-- posts to #staff-alerts when one joins or moves money. One row a player.
CREATE TABLE IF NOT EXISTS player_watchlist (
    player_id  BIGINT PRIMARY KEY REFERENCES players(id) ON DELETE CASCADE,
    reason     TEXT NOT NULL CHECK (length(reason) BETWEEN 1 AND 500),
    added_by   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    flag_id    BIGINT REFERENCES anti_cheat_flags(id) ON DELETE SET NULL,  -- set when added by reviewing a flag as "watch"
    added_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
