-- Moderation (docs/GAMEPANEL_PARITY.md §3, OPERATIONS.md §2): cases,
-- punishment points, bans linked to cases, ban appeals, anti-cheat review.
-- Safe to run more than once. schema.sql has the same definitions for new
-- installs.
BEGIN;

-- A case groups everything about one incident: who was involved, an
-- append-only timeline, points and bans.
CREATE TABLE IF NOT EXISTS staff_cases (
    id             BIGSERIAL PRIMARY KEY,
    case_type      TEXT NOT NULL CHECK (case_type IN ('exploit', 'cheating', 'rdm_vdm', 'conduct', 'chat',
                                                      'compensation', 'ban_appeal', 'other')),
    status         TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    summary        TEXT NOT NULL CHECK (length(summary) BETWEEN 3 AND 200),
    outcome        TEXT,               -- short result shown in the list, e.g. "30-day ban"
    lead_staff_id  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    flag_id        BIGINT REFERENCES anti_cheat_flags(id) ON DELETE SET NULL,  -- opened from a flag
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at      TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_staff_cases_status ON staff_cases(status, id DESC);

-- Everyone involved: subjects/related players and assisting staff. The lead
-- is staff_cases.lead_staff_id.
CREATE TABLE IF NOT EXISTS staff_case_participants (
    id         BIGSERIAL PRIMARY KEY,
    case_id    BIGINT NOT NULL REFERENCES staff_cases(id) ON DELETE CASCADE,
    player_id  BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('subject', 'related', 'reporter', 'witness', 'assisting_staff')),
    added_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (case_id, player_id, role)
);
CREATE INDEX IF NOT EXISTS idx_case_participants_player ON staff_case_participants(player_id);

-- The timeline. Never edited or deleted: a correction is a new entry that
-- points at the one it corrects (corrects_entry_id), so the original stays.
CREATE TABLE IF NOT EXISTS staff_case_entries (
    id                 BIGSERIAL PRIMARY KEY,
    case_id            BIGINT NOT NULL REFERENCES staff_cases(id) ON DELETE CASCADE,
    author_id          BIGINT REFERENCES players(id) ON DELETE SET NULL,
    kind               TEXT NOT NULL DEFAULT 'note'
                           CHECK (kind IN ('note', 'correction', 'points', 'points_revoked', 'ban', 'unban',
                                           'status', 'participant', 'flag', 'appeal')),
    body               TEXT NOT NULL,
    corrects_entry_id  BIGINT REFERENCES staff_case_entries(id) ON DELETE SET NULL,
    action_ref         TEXT,          -- e.g. 'banlist:482', informational
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_case_entries_case ON staff_case_entries(case_id, id);

-- Guidance for staff, not enforced automatically. Revoking is a case entry
-- plus revoked_at, never a delete.
CREATE TABLE IF NOT EXISTS punishment_points (
    id          BIGSERIAL PRIMARY KEY,
    case_id     BIGINT NOT NULL REFERENCES staff_cases(id) ON DELETE CASCADE,
    player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    points      INTEGER NOT NULL CHECK (points BETWEEN 1 AND 100),
    rules       TEXT NOT NULL,
    comment     TEXT,
    issued_by   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    expires_at  TIMESTAMPTZ,          -- stops counting after this
    revoked_at  TIMESTAMPTZ,
    revoked_by  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_points_player ON punishment_points(player_id);

-- Bans come from a case. Lifting sets expires_at to the lift time (that's
-- what the game checks) and records who and why.
ALTER TABLE banlist ADD COLUMN IF NOT EXISTS case_id BIGINT REFERENCES staff_cases(id) ON DELETE SET NULL;
ALTER TABLE banlist ADD COLUMN IF NOT EXISTS player_id BIGINT REFERENCES players(id) ON DELETE SET NULL;
ALTER TABLE banlist ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'game';
ALTER TABLE banlist ADD COLUMN IF NOT EXISTS note TEXT;        -- staff-only, never shown to the player
ALTER TABLE banlist ADD COLUMN IF NOT EXISTS lifted_at TIMESTAMPTZ;
ALTER TABLE banlist ADD COLUMN IF NOT EXISTS lifted_by BIGINT REFERENCES players(id) ON DELETE SET NULL;
ALTER TABLE banlist ADD COLUMN IF NOT EXISTS lift_reason TEXT;
DO $$ BEGIN
    ALTER TABLE banlist ADD CONSTRAINT banlist_scope_check CHECK (scope IN ('game', 'game_discord'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- A banned player's appeal, from their dashboard. One open appeal per ban.
CREATE TABLE IF NOT EXISTS ban_appeals (
    id           BIGSERIAL PRIMARY KEY,
    ban_id       INTEGER NOT NULL REFERENCES banlist(id) ON DELETE CASCADE,
    player_id    BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    body         TEXT NOT NULL CHECK (length(body) BETWEEN 20 AND 4000),
    status       TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'accepted', 'rejected')),
    decided_by   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    decision     TEXT,
    decided_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ban_appeals_open ON ban_appeals(ban_id) WHERE status = 'open';

-- Anti-cheat review: a note, and 'watch' / 'case' outcomes.
ALTER TABLE anti_cheat_flags ADD COLUMN IF NOT EXISTS resolution_note TEXT;
ALTER TABLE anti_cheat_flags DROP CONSTRAINT IF EXISTS anti_cheat_flags_resolution_check;
ALTER TABLE anti_cheat_flags ADD CONSTRAINT anti_cheat_flags_resolution_check
    CHECK (resolution IN ('kick', 'ban', 'dismiss', 'watch', 'case'));

COMMIT;
