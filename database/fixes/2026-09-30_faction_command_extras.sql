-- Faction command: recruits & training, discipline, divisions & quals,
-- ranks & gear (layout plan "Police command"; GAMEPANEL_PARITY §6.1).
-- Brings an existing database up to database/schema.sql. Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-09-30_faction_command_extras.sql

BEGIN;

-- Rank rules: minimum days in rank before promotion, qualifications needed
-- to hold the rank, and the public description.
ALTER TABLE faction_rank_names ADD COLUMN IF NOT EXISTS min_days INTEGER NOT NULL DEFAULT 0 CHECK (min_days >= 0);
ALTER TABLE faction_rank_names ADD COLUMN IF NOT EXISTS required_quals TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE faction_rank_names ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';

-- The Command log now records every command action, not only rank changes.
ALTER TABLE faction_log DROP CONSTRAINT IF EXISTS faction_log_kind_check;
ALTER TABLE faction_log ADD CONSTRAINT faction_log_kind_check CHECK (kind IN (
    'recruit', 'promote', 'demote', 'remove',
    'probation', 'training', 'discipline', 'discharge', 'blacklist', 'division', 'qual', 'rank_rules', 'settings'));
ALTER TABLE faction_log ADD COLUMN IF NOT EXISTS detail TEXT NOT NULL DEFAULT '';

-- ---------------------------------------------------------------------------
-- Faction command extras (layout plan "Police command": Recruits &
-- training, Discipline, Divisions & quals, Ranks & gear). Everything here
-- is the faction's own record, run by its command; staff with
-- factions.audit can read it. History tables are append-only: corrections
-- are new rows, never edits.
-- ---------------------------------------------------------------------------

-- Per-faction numbers that command's tools use.
CREATE TABLE IF NOT EXISTS faction_settings (
    faction             TEXT PRIMARY KEY CHECK (faction IN ('police', 'ems')),
    probation_days      INTEGER NOT NULL DEFAULT 14 CHECK (probation_days BETWEEN 1 AND 90),
    points_expiry_days  INTEGER NOT NULL DEFAULT 90 CHECK (points_expiry_days BETWEEN 7 AND 730),
    mvw_days            INTEGER NOT NULL DEFAULT 7 CHECK (mvw_days BETWEEN 1 AND 90),
    blacklist_days      INTEGER NOT NULL DEFAULT 90 CHECK (blacklist_days BETWEEN 1 AND 3650)
);
INSERT INTO faction_settings (faction) VALUES ('police'), ('ems') ON CONFLICT DO NOTHING;

-- Qualifications, and who holds them. head_id is the head trainer.
CREATE TABLE IF NOT EXISTS faction_quals (
    faction  TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    key      TEXT NOT NULL CHECK (key ~ '^[A-Z0-9]{1,8}$'),
    name     TEXT NOT NULL,
    body     TEXT NOT NULL DEFAULT '',
    head_id  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    sort     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (faction, key)
);
CREATE TABLE IF NOT EXISTS faction_member_quals (
    faction     TEXT NOT NULL,
    player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    qual_key    TEXT NOT NULL,
    granted_by  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (faction, player_id, qual_key),
    FOREIGN KEY (faction, qual_key) REFERENCES faction_quals(faction, key) ON DELETE CASCADE
);

-- Specialist divisions. roles are listed most senior first; a member's
-- division role unlocks that division's gear. Everyone not in one is in
-- the faction's general duties.
CREATE TABLE IF NOT EXISTS faction_divisions (
    faction        TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    key            TEXT NOT NULL CHECK (key ~ '^[A-Z0-9]{1,8}$'),
    name           TEXT NOT NULL,
    color          TEXT NOT NULL DEFAULT '#93c5fd' CHECK (color ~ '^#[0-9a-fA-F]{6}$'),
    roles          TEXT[] NOT NULL CHECK (cardinality(roles) BETWEEN 1 AND 12),
    required_qual  TEXT,
    min_level      INTEGER NOT NULL DEFAULT 0,
    sort           INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (faction, key)
);
CREATE TABLE IF NOT EXISTS faction_member_divisions (
    faction       TEXT NOT NULL,
    player_id     BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    division_key  TEXT NOT NULL,
    role          TEXT NOT NULL,
    since         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (faction, player_id),
    FOREIGN KEY (faction, division_key) REFERENCES faction_divisions(faction, key) ON DELETE CASCADE
);

-- Probation and training sheets. A recruit has at most one active
-- probation per faction; each training result is a new row and the
-- newest per item counts.
CREATE TABLE IF NOT EXISTS faction_training_items (
    faction       TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    key           TEXT NOT NULL,
    name          TEXT NOT NULL,
    retake_hours  INTEGER NOT NULL DEFAULT 0 CHECK (retake_hours >= 0),
    sort          INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (faction, key)
);
CREATE TABLE IF NOT EXISTS faction_probations (
    id          BIGSERIAL PRIMARY KEY,
    faction     TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    fto_id      BIGINT REFERENCES players(id) ON DELETE SET NULL,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ends_at     TIMESTAMPTZ NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'confirmed', 'ended')),
    note        TEXT NOT NULL DEFAULT '',
    decided_by  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    decided_at  TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_faction_probations_active ON faction_probations(faction, player_id) WHERE status = 'active';
CREATE TABLE IF NOT EXISTS faction_training_results (
    id            BIGSERIAL PRIMARY KEY,
    probation_id  BIGINT NOT NULL REFERENCES faction_probations(id) ON DELETE CASCADE,
    item_key      TEXT NOT NULL,
    result        TEXT NOT NULL CHECK (result IN ('pass', 'fail', 'none')),
    by_id         BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_faction_training_results ON faction_training_results(probation_id, item_key, id DESC);

-- Discipline. min_points = 0 means a marked verbal warning (MVW) is
-- allowed for the offence. Entries are never edited: a correction is a new
-- entry (kind 'correction', negative points, corrects_id set).
CREATE TABLE IF NOT EXISTS faction_offences (
    id          BIGSERIAL PRIMARY KEY,
    faction     TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    tier        TEXT NOT NULL CHECK (tier IN ('low', 'high')),
    name        TEXT NOT NULL,
    min_points  INTEGER NOT NULL CHECK (min_points >= 0),
    max_points  INTEGER NOT NULL,
    sort        INTEGER NOT NULL DEFAULT 0,
    CHECK (max_points >= GREATEST(min_points, 1)),
    UNIQUE (faction, name)
);
CREATE TABLE IF NOT EXISTS faction_discipline (
    id            BIGSERIAL PRIMARY KEY,
    faction       TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    player_id     BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    level         INTEGER NOT NULL,           -- their rank at the time
    offence       TEXT NOT NULL,              -- name at the time (the catalogue can change)
    kind          TEXT NOT NULL CHECK (kind IN ('points', 'mvw', 'mvw_conversion', 'correction')),
    points        INTEGER NOT NULL DEFAULT 0,
    notes         TEXT NOT NULL,
    action        TEXT NOT NULL DEFAULT '',   -- what command applied, e.g. "3-day suspension"
    corrects_id   BIGINT REFERENCES faction_discipline(id),
    by_id         BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_faction_discipline ON faction_discipline(faction, player_id, id DESC);
CREATE TABLE IF NOT EXISTS faction_suspensions (
    id             BIGSERIAL PRIMARY KEY,
    faction        TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    player_id      BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    discipline_id  BIGINT REFERENCES faction_discipline(id),
    starts_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    ends_at        TIMESTAMPTZ NOT NULL,
    by_id          BIGINT REFERENCES players(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_faction_suspensions ON faction_suspensions(faction, player_id, ends_at DESC);
CREATE TABLE IF NOT EXISTS faction_discharges (
    id          BIGSERIAL PRIMARY KEY,
    faction     TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    player_id   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    last_level  INTEGER NOT NULL,
    type        TEXT NOT NULL CHECK (type IN ('resigned', 'inactivity', 'contract_termination', 'honourable_discharge', 'probation_ended')),
    notes       TEXT NOT NULL,
    by_id       BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_faction_discharges ON faction_discharges(faction, id DESC);
-- A blacklisted player can't apply to, or be recruited into, the faction
-- until ends_at (NULL = permanent) or until it's lifted.
CREATE TABLE IF NOT EXISTS faction_blacklist (
    id           BIGSERIAL PRIMARY KEY,
    faction      TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    player_id    BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    last_level   INTEGER NOT NULL DEFAULT 0,
    reason       TEXT NOT NULL,
    starts_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    ends_at      TIMESTAMPTZ,
    by_id        BIGINT REFERENCES players(id) ON DELETE SET NULL,
    lifted_at    TIMESTAMPTZ,
    lifted_by    BIGINT REFERENCES players(id) ON DELETE SET NULL,
    lift_reason  TEXT
);
CREATE INDEX IF NOT EXISTS idx_faction_blacklist ON faction_blacklist(faction, player_id);

-- Starting configuration, from the layout plan. Command can change rank
-- rules on the site; the rest is edited here until it has an editor.
INSERT INTO faction_quals (faction, key, name, body, sort) VALUES
    ('police', 'BT',  'Basic Training',         'Core procedures, radio, traffic stops. Required for everyone.', 1),
    ('police', 'BFT', 'Basic Firearms',         'Rifle handling, rules of engagement, range standards.', 2),
    ('police', 'AT',  'Advanced Training',      'Bank and hostage response, armed vehicle stops.', 3),
    ('police', 'FTO', 'Field Training Officer', 'Can train and evaluate recruits.', 4),
    ('police', 'PIL', 'Pilot',                  'PolAir flight certification.', 5),
    ('police', 'TAC', 'Tactical',               'S.R.G. entry and breach certification.', 6),
    ('police', 'INV', 'Investigations',         'Detectives casework and warrants.', 7)
ON CONFLICT DO NOTHING;

INSERT INTO faction_divisions (faction, key, name, color, roles, required_qual, min_level, sort) VALUES
    ('police', 'POLAIR', 'PolAir',     '#c4b5fd', ARRAY['Commander', 'Second in Command', 'Trainer Pilot', 'Senior Pilot', 'Pilot', 'Junior Pilot', 'Trial Pilot'], 'PIL', 0, 1),
    ('police', 'SRG',    'S.R.G.',     '#fca5a5', ARRAY['Commander', 'Second in Command', 'Team Leader', 'Trainer', 'Operator', 'Trial'], 'TAC', 0, 2),
    ('police', 'DET',    'Detectives', '#86efac', ARRAY['Commander', 'Second in Command', 'Training Officer', 'Senior Officer', 'Officer', 'Trial Officer'], NULL, 0, 3)
ON CONFLICT DO NOTHING;

INSERT INTO faction_training_items (faction, key, name, retake_hours, sort) VALUES
    ('police', 'equipment',   'Equipment check',            0, 1),
    ('police', 'range',       'Shooting range',             0, 2),
    ('police', 'bank',        'Bank clearing',              0, 3),
    ('police', 'hostage',     'Hostage negotiation',        0, 4),
    ('police', 'fitness',     'Physical fitness',           0, 5),
    ('police', 'stop',        'Vehicle traffic stop',       0, 6),
    ('police', 'armed_stop',  'Armed vehicle traffic stop', 0, 7),
    ('police', 'ride_along',  'Ride-along',                 0, 8),
    ('police', 'theory',      'Theory exam',               24, 9),
    ('ems', 'equipment',   'Equipment check',     0, 1),
    ('ems', 'treatment',   'Treatment and revive', 0, 2),
    ('ems', 'driving',     'Emergency driving',   0, 3),
    ('ems', 'ride_along',  'Ride-along',          0, 4),
    ('ems', 'theory',      'Theory exam',        24, 5)
ON CONFLICT DO NOTHING;

INSERT INTO faction_offences (faction, tier, name, min_points, max_points, sort)
SELECT f, o.tier, o.name, o.min_points, o.max_points, o.sort
FROM (VALUES ('police'), ('ems')) AS fs(f)
CROSS JOIN (VALUES
    ('low',  'Incorrect name layout', 0, 5, 1, true),
    ('low',  'Wrong vehicle (ground or air)', 0, 5, 2, true),
    ('low',  'Wrong weapon, equipment or clothing equipped', 0, 5, 3, true),
    ('low',  'Improper use of an armed or armoured vehicle', 5, 15, 4, false),
    ('low',  'Failure to give name on request or on radio', 0, 15, 5, true),
    ('low',  'Not on radio or on the wrong frequency', 5, 10, 6, true),
    ('low',  'Unprofessional communication', 5, 10, 7, true),
    ('low',  'General unprofessionalism or misconduct', 5, 10, 8, true),
    ('low',  'Not following faction or department SOPs', 5, 10, 9, true),
    ('low',  'Insubordination', 5, 10, 10, true),
    ('low',  'Using weapons, equipment or vehicles without the training', 5, 20, 11, true),
    ('low',  'Misuse of non-lethal force', 15, 20, 12, false),
    ('low',  'Verbal abuse', 0, 5, 13, true),
    ('low',  'Not following rules of engagement', 5, 15, 14, false),
    ('high', 'Committing crimes as a civilian under an officer''s name', 10, 15, 15, true),
    ('high', 'Deliberate murder of a civilian', 20, 50, 16, true),
    ('high', 'Deliberate murder of an officer', 20, 50, 17, true),
    ('high', 'Selling faction weapons or equipment', 20, 50, 18, true),
    ('high', 'Illegal activity while on duty', 20, 50, 19, true),
    ('high', 'Possession of illegal items (e.g. rebel shop weapons)', 20, 50, 20, true),
    ('high', 'Corruption or affiliating with known criminals', 50, 50, 21, true),
    ('high', 'Abuse of power', 50, 50, 22, true),
    ('high', 'Distributing confidential material', 50, 50, 23, true),
    ('high', 'Deleting faction documents', 50, 50, 24, true)
) AS o(tier, name, min_points, max_points, sort, ems)
WHERE fs.f = 'police' OR o.ems
ON CONFLICT DO NOTHING;

COMMIT;
