-- Notifications with acknowledgement (docs/GAMEPANEL_PARITY.md §7.3).
-- Safe to run more than once. schema.sql has the same definitions.
BEGIN;

-- An essential notice sent to many people at once (a policy or rule
-- change), so staff can see who has acknowledged it.
CREATE TABLE IF NOT EXISTS staff_notices (
    id          BIGSERIAL PRIMARY KEY,
    title       TEXT NOT NULL CHECK (length(title) BETWEEN 3 AND 120),
    body        TEXT NOT NULL,
    link        TEXT CHECK (link IS NULL OR (link LIKE '/%' AND link NOT LIKE '//%')),
    audience    TEXT NOT NULL DEFAULT 'staff' CHECK (audience IN ('staff', 'everyone')),
    created_by  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notifications (
    id               BIGSERIAL PRIMARY KEY,
    player_id        BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    title            TEXT NOT NULL,
    body             TEXT NOT NULL DEFAULT '',
    -- Same-site paths only, so a notification can never send anyone off-site.
    link             TEXT CHECK (link IS NULL OR (link LIKE '/%' AND link NOT LIKE '//%')),
    essential        BOOLEAN NOT NULL DEFAULT false,
    notice_id        BIGINT REFERENCES staff_notices(id) ON DELETE CASCADE,
    read_at          TIMESTAMPTZ,
    acknowledged_at  TIMESTAMPTZ,   -- essential only: an explicit "I've read this"
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notifications_player ON notifications(player_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(player_id) WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_unacked ON notifications(player_id) WHERE essential AND acknowledged_at IS NULL;

COMMIT;
