-- Recruitment (docs/GAMEPANEL_PARITY.md §4, §6.2): staff applications with
-- interviews, and faction applications reviewed by faction command.
-- Safe to run more than once. schema.sql has the same definitions.
BEGIN;

CREATE TABLE IF NOT EXISTS staff_applications (
    id           BIGSERIAL PRIMARY KEY,
    player_id    BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    answers      JSONB NOT NULL,     -- question key -> answer (internal/applications)
    status       TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'interview', 'accepted', 'rejected', 'withdrawn')),
    reviewed_by  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    review_note  TEXT,               -- for other reviewers, never shown to the applicant
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at   TIMESTAMPTZ
);
-- One open application per player.
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_applications_open ON staff_applications(player_id)
    WHERE status IN ('pending', 'interview');

CREATE TABLE IF NOT EXISTS staff_interviews (
    id              BIGSERIAL PRIMARY KEY,
    application_id  BIGINT NOT NULL UNIQUE REFERENCES staff_applications(id) ON DELETE CASCADE,
    interviewer_id  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    answers         JSONB NOT NULL DEFAULT '{}',
    outcome         TEXT CHECK (outcome IN ('pass', 'fail')),  -- NULL = in progress
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS faction_applications (
    id             BIGSERIAL PRIMARY KEY,
    player_id      BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    faction        TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    answers        JSONB NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'withdrawn')),
    decided_by     BIGINT REFERENCES players(id) ON DELETE SET NULL,
    decision_note  TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_faction_applications_open ON faction_applications(player_id, faction)
    WHERE status = 'pending';

COMMIT;
