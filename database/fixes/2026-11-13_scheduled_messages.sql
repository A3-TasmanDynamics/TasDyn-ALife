-- Scheduled in-game announcements (/admin/schedule): one-off or repeating
-- messages, and restart countdowns (also queued by Server Control when a
-- restart is scheduled). The website's scheduler turns due rows into
-- game_commands announcements; the game server shows them.
CREATE TABLE IF NOT EXISTS scheduled_messages (
    id              BIGSERIAL PRIMARY KEY,
    kind            TEXT NOT NULL DEFAULT 'announce' CHECK (kind IN ('announce', 'restart')),
    text            TEXT NOT NULL CHECK (length(text) BETWEEN 1 AND 300),
    next_at         TIMESTAMPTZ NOT NULL,
    repeat_minutes  INTEGER CHECK (repeat_minutes IS NULL OR repeat_minutes BETWEEN 5 AND 10080),
    until_at        TIMESTAMPTZ,
    active          BOOLEAN NOT NULL DEFAULT true,
    created_by      BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_sent_at    TIMESTAMPTZ,
    sent_count      INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_scheduled_messages_due ON scheduled_messages(next_at) WHERE active;
