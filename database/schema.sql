-- TasDyn-ALife canonical schema.
-- This is the ONE source of truth for the DB shape — see database/README.md.
-- Column names/order here are part of the contract in docs/DATA_CONTRACT.md
-- for `players`, and docs/ADMIN_TOOLS.md §3 for staff/admin/arsenal tables.
-- Changing a column referenced by either doc is a breaking change to the C++
-- extension and/or SQF side — update both docs and both sides in the same PR.
--
-- JSONB SHAPE RULE: any JSONB column whose value can cross the callExtension
-- boundary (gear, position, inventory, storage, damage, ...) must contain
-- only arrays, strings, numbers, and booleans -- NEVER a bare JSON object
-- (`{...}`). SQF's `parseSimpleArray` -- the documented, intended way to
-- turn callExtension's string output back into data (see
-- docs/DATA_CONTRACT.md) -- has no object/map literal in its grammar at
-- all. Represent key-value data as an array of [key, value] pairs instead
-- of an object: `[["primaryWeapon","arifle_MX_F"], ...]`, never
-- `{"primaryWeapon": "arifle_MX_F"}`. Written this way, Postgres's raw
-- JSONB text is already valid `parseSimpleArray` input with zero
-- transformation -- that's the whole point of the constraint, not an
-- arbitrary style preference.

BEGIN;

-- ---------------------------------------------------------------------------
-- Staff ranks (docs/ADMIN_TOOLS.md §3) — DB-driven, not hardcoded constants.
-- ---------------------------------------------------------------------------
CREATE TABLE staff_ranks (
    id                     SERIAL PRIMARY KEY,
    key                    TEXT NOT NULL UNIQUE,
    display_name           TEXT NOT NULL,
    level                  INTEGER NOT NULL,
    -- Web panel access defaults for this rank -- see docs/WEBSITE.md §4. Per-
    -- player exceptions reuse staff_permission_overrides below
    -- (command_key = 'panel.admin' / 'panel.support'), not a second override
    -- table -- Admin and Support are deliberately separate grants, not
    -- implied by rank level alone, since a support volunteer shouldn't gain
    -- ban tools just by being trusted enough to triage tickets.
    default_admin_panel    BOOLEAN NOT NULL DEFAULT false,
    default_support_panel  BOOLEAN NOT NULL DEFAULT false,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO staff_ranks (key, display_name, level, default_admin_panel, default_support_panel) VALUES
    ('trial_mod', 'Trial Moderator', 10, false, true),
    ('moderator', 'Moderator', 20, false, true),
    ('admin', 'Admin', 40, true, true),
    ('head_admin', 'Head Admin / Developer', 100, true, true);

-- ---------------------------------------------------------------------------
-- Players.
--
-- Each of civilian/police/medic is a SEPARATE progression on the same
-- account — a player can't have two civ characters, so *_level/_dept/
-- _licence/_gear/_cash are functionally dependent on player_id alone and
-- belong here rather than in a separate characters table (see
-- docs/DATA_CONTRACT.md for the full reasoning).
--
-- *_bank is a CACHE, not the source of truth — bank_accounts.balance is
-- authoritative, kept in sync with *_bank by trigger below, never by
-- application code remembering to update both.
--
-- No `active_faction` column: which faction a player is playing is chosen
-- at spawn each session (runtime SQF state) and passed explicitly with
-- every save/load call — it doesn't need to survive a restart.
--
-- *_alive/*_position ARE persisted, though, per faction -- reviewed
-- AsYetUntitled/Framework (Tonic's Altis Life, a widely-deployed 7-year-old
-- base) while designing this, and its `civ_alive`/`civ_position` columns
-- exist for a specific, real reason: without persisting death state, a
-- player who disconnects while dead/unconscious respawns fresh on
-- reconnect instead of resuming dead at the same spot -- a well-known
-- disconnect-to-escape-arrest/death exploit in this exact genre. Not
-- optional given that precedent.
--
-- civ_bounty is a CACHE (see wanted_crimes below), same pattern as *_bank.
-- ---------------------------------------------------------------------------
CREATE TABLE players (
    id             BIGSERIAL PRIMARY KEY,
    uid            TEXT NOT NULL UNIQUE CHECK (uid ~ '^[0-9]{17}$'),  -- Steam64 ID, digits only
    name           TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'active'
                       CHECK (status IN ('active', 'banned', 'whitelisted_pending')),
    last_seen      TIMESTAMPTZ,     -- cache, synced from player_sessions by trigger

    -- Linked Discord account (docs/WEBSITE.md §3/§9) -- nullable, a player
    -- can use the website without ever linking Discord. Linked either from
    -- the member portal (OAuth2) or via a `/link <code>` Discord command
    -- redeeming a code the portal generated -- see discord_link_codes
    -- below. discord_username is a display cache, not identity; discord_id
    -- is the durable key.
    discord_id       TEXT UNIQUE,
    discord_username TEXT,
    -- Player dashboard leaderboards are opt-in (layout plan): nobody is
    -- listed until they tick "Show me on leaderboards".
    leaderboard_opt_in BOOLEAN NOT NULL DEFAULT false,

    civ_cash       BIGINT NOT NULL DEFAULT 0,
    civ_bank       BIGINT NOT NULL DEFAULT 0,
    civ_licence    JSONB NOT NULL DEFAULT '[]'::jsonb,   -- array of licence keys, e.g. ["driver","boat"]
    civ_gear       JSONB NOT NULL DEFAULT '[]'::jsonb,   -- [[key,value],...] pairs, not an object -- see JSONB SHAPE RULE above
    civ_alive      BOOLEAN NOT NULL DEFAULT true,
    civ_position   JSONB NOT NULL DEFAULT '[]'::jsonb,   -- [] until the first death is ever recorded
    civ_bounty     BIGINT NOT NULL DEFAULT 0,            -- cache of wanted_crimes, see below
    civ_playtime_seconds BIGINT NOT NULL DEFAULT 0,

    cop_level      INTEGER NOT NULL DEFAULT 0,
    cop_dept       TEXT,
    cop_cash       BIGINT NOT NULL DEFAULT 0,
    cop_bank       BIGINT NOT NULL DEFAULT 0,
    cop_licence    JSONB NOT NULL DEFAULT '[]'::jsonb,
    cop_gear       JSONB NOT NULL DEFAULT '[]'::jsonb,
    cop_alive      BOOLEAN NOT NULL DEFAULT true,
    cop_position   JSONB NOT NULL DEFAULT '[]'::jsonb,
    cop_playtime_seconds BIGINT NOT NULL DEFAULT 0,

    medic_level    INTEGER NOT NULL DEFAULT 0,
    medic_dept     TEXT,
    medic_cash     BIGINT NOT NULL DEFAULT 0,
    medic_bank     BIGINT NOT NULL DEFAULT 0,
    medic_licence  JSONB NOT NULL DEFAULT '[]'::jsonb,
    medic_gear     JSONB NOT NULL DEFAULT '[]'::jsonb,
    medic_alive    BOOLEAN NOT NULL DEFAULT true,
    medic_position JSONB NOT NULL DEFAULT '[]'::jsonb,
    medic_playtime_seconds BIGINT NOT NULL DEFAULT 0,

    staff_rank_id  INTEGER REFERENCES staff_ranks(id) ON DELETE SET NULL,
    -- Whether staff_rank_id's access should actually apply right now --
    -- docs/OPERATIONS.md §3. A suspended/LOA staff member keeps their rank
    -- (so nothing else has to change when they come back) but panel access
    -- resolution (internal/auth/session.go's resolvePanelAccess) requires
    -- 'active' in addition to the rank's own grants.
    staff_status         TEXT NOT NULL DEFAULT 'active'
                             CHECK (staff_status IN ('active', 'suspended', 'loa')),
    staff_status_reason  TEXT,
    staff_status_until   TIMESTAMPTZ,  -- LOA/suspension end date; NULL = indefinite
    staff_team           TEXT,         -- e.g. 'Moderation', 'Support'; NULL = unassigned (GAMEPANEL_PARITY §2.2)
    staff_region         TEXT,         -- e.g. 'AU-East', 'NZ'

    -- BattlEye GUID, derived from uid (src/website/internal/steam/guid.go).
    -- Set on website signup or by the website's backfill for game-created rows.
    be_guid              TEXT UNIQUE,

    -- Steam Web API cache (docs/INTEGRATIONS.md §2.1) -- display/review data
    -- only, never identity. NULL = never fetched, or no API key configured.
    steam_name             TEXT,
    steam_avatar_url       TEXT,
    steam_created_at       TIMESTAMPTZ,   -- NULL also when the profile is private
    steam_vac_bans         INTEGER,
    steam_game_bans        INTEGER,
    steam_days_since_ban   INTEGER,
    steam_community_banned BOOLEAN,
    steam_refreshed_at     TIMESTAMPTZ,

    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_players_staff_rank_id ON players(staff_rank_id);

-- General gameplay event log (job payouts, deaths, revives, licence
-- purchases, faction switches, ...). Distinct from staff_log (admin
-- actions) and kick_log (kicks specifically, including non-staff ones).
CREATE TABLE player_log (
    id          BIGSERIAL PRIMARY KEY,
    player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL,
    details     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_player_log_player_id ON player_log(player_id);

-- Name history, not a single overwritten column -- lets staff spot a UID
-- cycling through names (alt-account/evasion pattern) instead of only ever
-- seeing whatever name happens to be current. One row per distinct name
-- ever used; last_used_at bumps on repeat sightings rather than inserting
-- a duplicate row.
CREATE TABLE player_aliases (
    id            BIGSERIAL PRIMARY KEY,
    player_id     BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, name)
);

-- Connect/disconnect tracking — backs ANTI_CHEAT.md's first-time player
-- screening (a UID's first session gets tighter thresholds) and general
-- moderation (IP visible to staff investigating a report).
CREATE TABLE player_sessions (
    id               BIGSERIAL PRIMARY KEY,
    player_id        BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    ip_address       INET,
    connected_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    disconnected_at  TIMESTAMPTZ
);

CREATE INDEX idx_player_sessions_player_id ON player_sessions(player_id);

-- ---------------------------------------------------------------------------
-- Wanted list (Police faction). players.civ_bounty is a CACHE of the sum of
-- outstanding (cleared_at IS NULL) bounty_amount here, kept in sync by
-- trigger -- same "ledger is authoritative, players.* is a cache" pattern
-- as bank_accounts/bank_transactions above, not a separate design.
-- ---------------------------------------------------------------------------
CREATE TABLE wanted_crimes (
    id             BIGSERIAL PRIMARY KEY,
    player_id      BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    crime_key      TEXT NOT NULL,
    bounty_amount  BIGINT NOT NULL,
    issued_by      BIGINT REFERENCES players(id) ON DELETE SET NULL,  -- NULL = system-issued
    cleared_at     TIMESTAMPTZ,     -- NULL = still outstanding
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wanted_crimes_player_id ON wanted_crimes(player_id);
CREATE INDEX idx_wanted_crimes_outstanding ON wanted_crimes(player_id) WHERE cleared_at IS NULL;

-- ---------------------------------------------------------------------------
-- Generic idempotency ledger for `save` calls on delta (not absolute-set)
-- fields -- currently just *_cash. Physical cash has the same
-- concurrent-double-apply risk bank_accounts was split into its own
-- ledger to avoid (see docs/DATA_CONTRACT.md), so `save` treats it as a
-- signed delta, not a new total, and checks/inserts a token here in the
-- same transaction as the balance update. Every other `save` field is
-- absolute-set and idempotent by nature (re-applying the same name or
-- gear twice is harmless), so it doesn't need a token check here.
-- ---------------------------------------------------------------------------
CREATE TABLE applied_request_tokens (
    id          BIGSERIAL PRIMARY KEY,
    player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    token       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, token)
);

-- ---------------------------------------------------------------------------
-- Banking — bank_accounts.balance is authoritative; players.*_bank is kept
-- in sync by trigger (see bottom of this file). One account per player per
-- faction, matching "each faction is a separate entity."
-- ---------------------------------------------------------------------------
CREATE TABLE bank_accounts (
    id          BIGSERIAL PRIMARY KEY,
    player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    faction     TEXT NOT NULL CHECK (faction IN ('civilian', 'police', 'medic')),
    balance     BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, faction)
);

CREATE TABLE bank_transactions (
    id                  BIGSERIAL PRIMARY KEY,
    account_id          BIGINT NOT NULL REFERENCES bank_accounts(id) ON DELETE CASCADE,
    type                TEXT NOT NULL
                            CHECK (type IN ('deposit', 'withdrawal', 'transfer_in', 'transfer_out',
                                             'payout', 'purchase', 'admin_adjustment')),
    amount              BIGINT NOT NULL,
    balance_after       BIGINT NOT NULL,
    related_account_id  BIGINT REFERENCES bank_accounts(id) ON DELETE SET NULL,  -- for transfers
    memo                TEXT,
    request_token       TEXT,          -- idempotency, see ANTI_CHEAT.md Layer 2
    created_by          BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_bank_transactions_account_id ON bank_transactions(account_id);
-- Idempotency enforcement: a token can only be used once per account, but
-- admin_adjustment entries may have no token at all (partial index).
CREATE UNIQUE INDEX idx_bank_transactions_token
    ON bank_transactions(account_id, request_token)
    WHERE request_token IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Vehicles.
-- ---------------------------------------------------------------------------
CREATE TABLE garages (
    id             BIGSERIAL PRIMARY KEY,
    name           TEXT NOT NULL,
    position       JSONB NOT NULL,     -- world coordinates
    faction_scope  TEXT CHECK (faction_scope IN ('civilian', 'police', 'medic')),  -- NULL = open to all
    capacity       INTEGER,            -- NULL = unlimited
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE gangs (
    id                 BIGSERIAL PRIMARY KEY,
    name               TEXT NOT NULL UNIQUE,
    tag                TEXT NOT NULL UNIQUE,
    leader_player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE RESTRICT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Not in the original list — a gang needs members beyond just its leader.
CREATE TABLE gang_members (
    id         BIGSERIAL PRIMARY KEY,
    gang_id    BIGINT NOT NULL REFERENCES gangs(id) ON DELETE CASCADE,
    player_id  BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    rank       TEXT NOT NULL DEFAULT 'member',
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id)   -- a player belongs to at most one gang at a time
);

CREATE TABLE vehicles (
    id               BIGSERIAL PRIMARY KEY,
    owner_player_id  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    owner_gang_id    BIGINT REFERENCES gangs(id) ON DELETE SET NULL,
    classname        TEXT NOT NULL,
    plate            TEXT UNIQUE,
    faction_scope    TEXT CHECK (faction_scope IN ('civilian', 'police', 'medic')),
    fuel             REAL NOT NULL DEFAULT 1.0,
    damage           JSONB NOT NULL DEFAULT '[]'::jsonb,  -- [[hitpointName,damage],...] pairs, e.g. [["hitengine",0.4]]
                                                            -- not a single float -- see getAllHitPointsDamage
    status           TEXT NOT NULL DEFAULT 'garaged'
                         CHECK (status IN ('garaged', 'spawned', 'impounded', 'destroyed')),
    garage_id        BIGINT REFERENCES garages(id) ON DELETE SET NULL,
    position         JSONB,           -- last known world position, NULL if garaged
    inventory        JSONB NOT NULL DEFAULT '[]'::jsonb,  -- [[key,value],...] pairs -- cargo
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (owner_player_id IS NOT NULL OR owner_gang_id IS NOT NULL)
);

CREATE INDEX idx_vehicles_owner_player_id ON vehicles(owner_player_id);
CREATE INDEX idx_vehicles_owner_gang_id ON vehicles(owner_gang_id);

CREATE TABLE vehicle_logs (
    id          BIGSERIAL PRIMARY KEY,
    vehicle_id  BIGINT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    player_id   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    action      TEXT NOT NULL CHECK (action IN ('purchase', 'rent', 'sell', 'chop', 'impound', 'return')),
    amount      BIGINT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_vehicle_logs_vehicle_id ON vehicle_logs(vehicle_id);

-- ---------------------------------------------------------------------------
-- Houses.
-- ---------------------------------------------------------------------------
CREATE TABLE houses (
    id               BIGSERIAL PRIMARY KEY,
    house_key        TEXT NOT NULL UNIQUE,   -- matches a predefined map location/building id
    owner_player_id  BIGINT REFERENCES players(id) ON DELETE SET NULL,  -- NULL = unowned
    price            BIGINT NOT NULL,
    storage          JSONB NOT NULL DEFAULT '[]'::jsonb,  -- [[key,value],...] pairs
    locked           BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_houses_owner_player_id ON houses(owner_player_id);

-- ---------------------------------------------------------------------------
-- Gang finances + activity log.
-- ---------------------------------------------------------------------------
CREATE TABLE gang_accounts (
    id          BIGSERIAL PRIMARY KEY,
    gang_id     BIGINT NOT NULL UNIQUE REFERENCES gangs(id) ON DELETE CASCADE,
    balance     BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE gang_transactions (
    id               BIGSERIAL PRIMARY KEY,
    gang_account_id  BIGINT NOT NULL REFERENCES gang_accounts(id) ON DELETE CASCADE,
    type             TEXT NOT NULL CHECK (type IN ('deposit', 'withdrawal', 'admin_adjustment')),
    amount           BIGINT NOT NULL,
    balance_after    BIGINT NOT NULL,
    actor_player_id  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    memo             TEXT,
    request_token    TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_gang_transactions_gang_account_id ON gang_transactions(gang_account_id);
CREATE UNIQUE INDEX idx_gang_transactions_token
    ON gang_transactions(gang_account_id, request_token)
    WHERE request_token IS NOT NULL;

CREATE TABLE gang_log (
    id                BIGSERIAL PRIMARY KEY,
    gang_id           BIGINT NOT NULL REFERENCES gangs(id) ON DELETE CASCADE,
    actor_player_id   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    action            TEXT NOT NULL,   -- e.g. 'member_added','member_removed','rank_changed'
    details           JSONB,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_gang_log_gang_id ON gang_log(gang_id);

-- ---------------------------------------------------------------------------
-- Staff/moderation (docs/ADMIN_TOOLS.md §7-9).
-- ---------------------------------------------------------------------------
-- Which permission keys each staff rank has (docs/GAMEPANEL_PARITY.md §2.1),
-- edited on /admin/roles. Keys must exist in the website's catalogue
-- (src/website/internal/auth/permissions.go); the seed below gives each
-- built-in rank every key whose SeedLevel is at or below its level, and is
-- checked against the catalogue by internal/auth's tests.
CREATE TABLE rank_permissions (
    rank_id      INTEGER NOT NULL REFERENCES staff_ranks(id) ON DELETE CASCADE,
    command_key  TEXT NOT NULL,
    PRIMARY KEY (rank_id, command_key)
);

INSERT INTO rank_permissions (rank_id, command_key)
SELECT sr.id, k.command_key
FROM staff_ranks sr
JOIN (VALUES
    ('staff.view', 10),
    ('staff.edit', 100),
    ('staff.loa', 40),
    ('staff.suspend', 100),
    ('staff.remove', 100),
    ('roles.manage', 100),
    ('staff.notes', 40),
    ('cases.view', 10),
    ('cases.lead', 10),
    ('cases.close', 20),
    ('bans.issue', 20),
    ('bans.permanent', 40),
    ('bans.revoke', 40),
    ('bans.appeal_review', 40),
    ('anticheat.review', 20),
    ('players.view', 10),
    ('players.vehicles', 20),
    ('players.edit_police', 100),
    ('players.edit_medic', 100),
    ('players.compensate', 40),
    ('players.compensate_large', 100),
    ('applications.view', 20),
    ('applications.decide', 40),
    ('factions.audit', 20),
    ('factions.configure', 100),
    ('records.review', 40),
    ('server.logs', 40),
    ('server.control', 100),
    ('database.query', 100),
    ('announce.post', 40),
    ('rules.edit', 40),
    ('bot.admin', 100)
) AS k(command_key, seed_level) ON sr.level >= k.seed_level;

-- Display names for police/EMS levels (GAMEPANEL_PARITY §6.3), shown
-- wherever a raw cop_level / medic_level number used to appear.
CREATE TABLE faction_rank_names (
    faction  TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    level    INTEGER NOT NULL CHECK (level > 0),
    name     TEXT NOT NULL,
    -- Faction command (docs/GAMEPANEL_PARITY.md §6.1, layout plan "Police
    -- command"): short name for tight spaces, how many officers the rank
    -- is meant to hold (NULL = no limit), and the highest level someone
    -- at this rank may set others to (0 = can't change ranks). Whether a
    -- rank is command is its own tick, is_command, below. promote_up_to
    -- must stay below the rank's own level.
    short_name     TEXT,
    slots          INTEGER CHECK (slots > 0),
    promote_up_to  INTEGER NOT NULL DEFAULT 0 CHECK (promote_up_to >= 0 AND promote_up_to < level),
    -- Rank rules (Ranks & gear): minimum days in rank before promotion,
    -- qualifications needed to hold it (faction_quals keys), and the
    -- description shown on the public faction page.
    min_days        INTEGER NOT NULL DEFAULT 0 CHECK (min_days >= 0),
    required_quals  TEXT[] NOT NULL DEFAULT '{}',
    description     TEXT NOT NULL DEFAULT '',
    -- Ticked on Ranks & gear. is_command gives the rank the command panel
    -- (shown as CMD); is_cabinet (CAB) is the faction's senior leadership,
    -- who can lift blacklists early and blacklist permanently. Cabinet
    -- ranks must also be command ranks.
    is_command      BOOLEAN NOT NULL DEFAULT false,
    is_cabinet      BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT faction_rank_names_cabinet_check CHECK (NOT is_cabinet OR is_command),
    PRIMARY KEY (faction, level)
);

-- Every faction roster change made on the website: by faction command on
-- the command panel, or by Management as a staff override from Player
-- Lookup. own_faction flags an override by a staff member who is in that
-- faction themselves (conflict of interest: allowed, but visible). The
-- level change itself is also captured by the players trigger in
-- rank_changes/staff_log; this table is the faction's own record, shown
-- as the Command log. Never edited or deleted.
CREATE TABLE faction_log (
    id           BIGSERIAL PRIMARY KEY,
    faction      TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    actor_id     BIGINT REFERENCES players(id) ON DELETE SET NULL,
    target_id    BIGINT REFERENCES players(id) ON DELETE SET NULL,
    -- Rank changes, plus every other command action (detail describes those).
    kind         TEXT NOT NULL CHECK (kind IN (
                     'recruit', 'promote', 'demote', 'remove',
                     'probation', 'training', 'discipline', 'discharge', 'blacklist', 'division', 'qual', 'rank_rules', 'settings',
                     'roster', 'roll_call')),
    detail       TEXT NOT NULL DEFAULT '',
    from_level   INTEGER NOT NULL,
    to_level     INTEGER NOT NULL,
    reason       TEXT NOT NULL,
    via          TEXT NOT NULL CHECK (via IN ('command', 'staff_override')),
    own_faction  BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_faction_log_faction ON faction_log(faction, id DESC);
CREATE INDEX idx_faction_log_target ON faction_log(target_id, id DESC);

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
    -- The Administration division: its members administrate and maintain
    -- the command panel whatever their rank. Only cabinet, Management and
    -- the division's Commander (roles[1]) appoint to it. One per faction.
    is_admin       BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (faction, key)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_faction_divisions_admin ON faction_divisions(faction) WHERE is_admin;
CREATE TABLE IF NOT EXISTS faction_member_divisions (
    faction       TEXT NOT NULL,
    player_id     BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    division_key  TEXT NOT NULL,
    role          TEXT NOT NULL,
    since         TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One specialist division per member (enforced in internal/factions),
    -- plus optionally the Administration division.
    PRIMARY KEY (faction, player_id, division_key),
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

-- Division applications: a faction member applies to join a division.
-- Reviewed by Administration, the division's command (its top two roles,
-- or the Commander alone in a two-role division), cabinet and Management;
-- the Administration division is reviewed only by cabinet, Management and
-- its Commander. Accepting posts them at the division's entry role.
CREATE TABLE IF NOT EXISTS faction_division_applications (
    id             BIGSERIAL PRIMARY KEY,
    faction        TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    division_key   TEXT NOT NULL,
    player_id      BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    why            TEXT NOT NULL,
    availability   TEXT NOT NULL DEFAULT '',
    experience     TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'withdrawn')),
    decided_by     BIGINT REFERENCES players(id) ON DELETE SET NULL,
    decision_note  TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at     TIMESTAMPTZ,
    FOREIGN KEY (faction, division_key) REFERENCES faction_divisions(faction, key) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_faction_division_apps_open ON faction_division_applications(faction, division_key, player_id) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_faction_division_apps ON faction_division_applications(faction, division_key, id DESC);

-- Personnel roster (Roster page): per-member details command keeps, and
-- the monthly roll call. Rows are created the first time someone is
-- recruited or edited.
CREATE TABLE IF NOT EXISTS faction_members (
    faction      TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    player_id    BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    badge        TEXT NOT NULL DEFAULT '' CHECK (length(badge) <= 12),
    region       TEXT NOT NULL DEFAULT '' CHECK (region IN ('', 'AU', 'NZ', 'Asia', 'EU', 'NA', 'SA', 'Other')),
    status       TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'semi_active', 'loa', 'reserve')),
    enrolled_on  DATE,
    notes        TEXT NOT NULL DEFAULT '' CHECK (length(notes) <= 300),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (faction, player_id)
);
-- Monthly roll call: present (RC) or excused absence (EA), per month.
CREATE TABLE IF NOT EXISTS faction_roll_call (
    faction    TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    player_id  BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    month      DATE NOT NULL CHECK (extract(day FROM month) = 1),
    mark       TEXT NOT NULL CHECK (mark IN ('present', 'excused')),
    by_id      BIGINT REFERENCES players(id) ON DELETE SET NULL,
    marked_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (faction, player_id, month)
);

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

INSERT INTO faction_divisions (faction, key, name, color, roles, required_qual, min_level, sort, is_admin) VALUES
    ('police', 'ADMIN', 'Administration', '#fcd34d', ARRAY['Commander', 'Deputy Commander', 'Administrator'], NULL, 0, 0, true),
    ('ems',    'ADMIN', 'Administration', '#fcd34d', ARRAY['Commander', 'Deputy Commander', 'Administrator'], NULL, 0, 0, true)
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

CREATE TABLE staff_permission_overrides (
    id           SERIAL PRIMARY KEY,
    player_id    BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    command_key  TEXT NOT NULL,
    allow        BOOLEAN NOT NULL,
    granted_by   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, command_key)
);

-- Append-only notes about a staff member (GAMEPANEL_PARITY §2.2) --
-- never edited or deleted, so the history can't be quietly rewritten.
CREATE TABLE staff_notes (
    id          BIGSERIAL PRIMARY KEY,
    player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    author_id   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('note', 'promotion')),
    body        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_staff_notes_player_id ON staff_notes(player_id, created_at DESC);

-- Every admin action — see docs/ADMIN_TOOLS.md §9.
CREATE TABLE staff_log (
    id                BIGSERIAL PRIMARY KEY,
    staff_player_id   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    target_player_id  BIGINT REFERENCES players(id) ON DELETE SET NULL,  -- NULL: no target (e.g. announcement)
    action            TEXT NOT NULL,
    reason            TEXT,
    before_value      JSONB,
    after_value       JSONB,
    -- Where the action came from (docs/INTEGRATIONS.md §3.1). 'manual' =
    -- a direct database change with no app attribution -- flagged, not hidden.
    source            TEXT NOT NULL DEFAULT 'website'
                          CHECK (source IN ('website', 'discord', 'game', 'manual')),
    -- Set once internal/audit's poster has sent this row to #staff-log.
    discord_posted_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_staff_log_target_player_id ON staff_log(target_player_id);

-- Kept separate from staff_log: not every kick is staff-issued (vote-kick,
-- BattlEye, anti-cheat auto-kick all land here with kicked_by = NULL).
CREATE TABLE kick_log (
    id                BIGSERIAL PRIMARY KEY,
    target_player_id  BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    kicked_by         BIGINT REFERENCES players(id) ON DELETE SET NULL,  -- NULL = system/BattlEye/anti-cheat
    kick_type         TEXT NOT NULL CHECK (kick_type IN ('manual', 'vote', 'anticheat', 'battleye')),
    reason            TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_kick_log_target_player_id ON kick_log(target_player_id);

-- This project's own persistent banlist (docs/ADMIN_TOOLS.md §8) — not a
-- literal multi-org shared/cloud ban service.
CREATE TABLE banlist (
    id          SERIAL PRIMARY KEY,
    uid         TEXT NOT NULL,
    reason      TEXT NOT NULL,
    banned_by   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    expires_at  TIMESTAMPTZ,           -- NULL = permanent
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_banlist_uid ON banlist(uid);

CREATE TABLE whitelist (
    id          SERIAL PRIMARY KEY,
    uid         TEXT NOT NULL UNIQUE,
    added_by    BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Flagged anti-cheat events — backs the Admin-tier flag review panel,
-- docs/ADMIN_TOOLS.md §7, and docs/ANTI_CHEAT.md Layer 4's graduated response.
CREATE TABLE anti_cheat_flags (
    id            BIGSERIAL PRIMARY KEY,
    player_id     BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    -- 'steam_ban': a new VAC/game ban appeared on the player's Steam account (internal/steam).
    flag_type     TEXT NOT NULL CHECK (flag_type IN ('honeypot', 'movement', 'idempotency_reject', 'rate_limit', 'steam_ban')),
    confidence    TEXT NOT NULL CHECK (confidence IN ('high', 'medium')),
    details       JSONB,
    reviewed_by   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    reviewed_at   TIMESTAMPTZ,
    -- NULL = unreviewed. 'watch' = keep an eye on them; 'case' = handled in a case.
    resolution    TEXT CONSTRAINT anti_cheat_flags_resolution_check CHECK (resolution IN ('kick', 'ban', 'dismiss', 'watch', 'case')),
    resolution_note TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_anti_cheat_flags_player_id ON anti_cheat_flags(player_id);
CREATE INDEX idx_anti_cheat_flags_unreviewed ON anti_cheat_flags(created_at) WHERE reviewed_at IS NULL;

-- ---------------------------------------------------------------------------
-- Moderation: cases, punishment points, bans from cases, appeals
-- (docs/GAMEPANEL_PARITY.md §3, OPERATIONS.md §2; internal/cases).
-- ---------------------------------------------------------------------------

-- A case groups everything about one incident: who was involved, an
-- append-only timeline, points and bans.
CREATE TABLE staff_cases (
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
CREATE INDEX idx_staff_cases_status ON staff_cases(status, id DESC);

-- Everyone involved: subjects/related players and assisting staff. The lead
-- is staff_cases.lead_staff_id.
CREATE TABLE staff_case_participants (
    id         BIGSERIAL PRIMARY KEY,
    case_id    BIGINT NOT NULL REFERENCES staff_cases(id) ON DELETE CASCADE,
    player_id  BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('subject', 'related', 'reporter', 'witness', 'assisting_staff')),
    added_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (case_id, player_id, role)
);
CREATE INDEX idx_case_participants_player ON staff_case_participants(player_id);

-- The timeline. Never edited or deleted: a correction is a new entry that
-- points at the one it corrects (corrects_entry_id), so the original stays.
CREATE TABLE staff_case_entries (
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
CREATE INDEX idx_case_entries_case ON staff_case_entries(case_id, id);

-- Guidance for staff, not enforced automatically. Revoking is a case entry
-- plus revoked_at, never a delete.
CREATE TABLE punishment_points (
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
CREATE INDEX idx_points_player ON punishment_points(player_id);

-- Bans come from a case. Lifting sets expires_at to the lift time (that's
-- what the game checks) and records who and why.
ALTER TABLE banlist ADD COLUMN case_id BIGINT REFERENCES staff_cases(id) ON DELETE SET NULL;
ALTER TABLE banlist ADD COLUMN player_id BIGINT REFERENCES players(id) ON DELETE SET NULL;
ALTER TABLE banlist ADD COLUMN scope TEXT NOT NULL DEFAULT 'game';
ALTER TABLE banlist ADD COLUMN note TEXT;        -- staff-only, never shown to the player
ALTER TABLE banlist ADD COLUMN lifted_at TIMESTAMPTZ;
ALTER TABLE banlist ADD COLUMN lifted_by BIGINT REFERENCES players(id) ON DELETE SET NULL;
ALTER TABLE banlist ADD COLUMN lift_reason TEXT;
ALTER TABLE banlist ADD CONSTRAINT banlist_scope_check CHECK (scope IN ('game', 'game_discord'));

-- A banned player's appeal, from their dashboard. One open appeal per ban.
CREATE TABLE ban_appeals (
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
CREATE UNIQUE INDEX idx_ban_appeals_open ON ban_appeals(ban_id) WHERE status = 'open';

-- ---------------------------------------------------------------------------
-- Recruitment: staff applications and interviews, faction applications
-- (docs/GAMEPANEL_PARITY.md §4, §6.2; internal/applications).
-- ---------------------------------------------------------------------------

CREATE TABLE staff_applications (
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
CREATE UNIQUE INDEX idx_staff_applications_open ON staff_applications(player_id)
    WHERE status IN ('pending', 'interview');

CREATE TABLE staff_interviews (
    id              BIGSERIAL PRIMARY KEY,
    application_id  BIGINT NOT NULL UNIQUE REFERENCES staff_applications(id) ON DELETE CASCADE,
    interviewer_id  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    answers         JSONB NOT NULL DEFAULT '{}',
    outcome         TEXT CHECK (outcome IN ('pass', 'fail')),  -- NULL = in progress
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE faction_applications (
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
CREATE UNIQUE INDEX idx_faction_applications_open ON faction_applications(player_id, faction)
    WHERE status = 'pending';


-- ---------------------------------------------------------------------------
-- Arsenal editing (docs/ADMIN_TOOLS.md §5).
-- ---------------------------------------------------------------------------
CREATE TABLE arsenal_item_pools (
    id              SERIAL PRIMARY KEY,
    faction_key     TEXT NOT NULL,     -- faction key, or 'staff' for the admin test pool
    item_classname  TEXT NOT NULL,
    category        TEXT NOT NULL
                        CHECK (category IN ('weapon', 'attachment', 'uniform', 'vest', 'backpack', 'item')),
    UNIQUE (faction_key, item_classname)
);

CREATE TABLE arsenal_loadout_presets (
    id             SERIAL PRIMARY KEY,
    name           TEXT NOT NULL,
    faction_key    TEXT,               -- nullable: not every preset is faction-scoped
    staff_rank_id  INTEGER REFERENCES staff_ranks(id) ON DELETE SET NULL,
    items          JSONB NOT NULL,     -- array of classnames
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Website (docs/WEBSITE.md) — public site, member portal, Admin Panel,
-- Support Panel. Deliberately NOT a parallel identity/money system: logins
-- resolve to the same `players` row (uid = Steam64 ID), and writes from the
-- member portal (transfers, gang management) go through the exact same
-- bank_accounts/bank_transactions/gangs/gang_members tables the game
-- already treats as authoritative -- see docs/WEBSITE.md §5-6.
-- ---------------------------------------------------------------------------

-- DB-backed sessions, not JWT -- deleting a row logs a session out
-- immediately (a ban or rank change can delete it in the same transaction),
-- which a signed, self-contained token can't do without its own blocklist
-- table anyway. token_hash stores SHA-256 of the cookie value; the raw
-- token itself is never persisted, same reasoning as a password-reset token.
CREATE TABLE web_sessions (
    id                     BIGSERIAL PRIMARY KEY,
    player_id              BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    token_hash             TEXT NOT NULL UNIQUE,
    admin_panel_access     BOOLEAN NOT NULL DEFAULT false,  -- resolved at login, docs/WEBSITE.md §4
    support_panel_access   BOOLEAN NOT NULL DEFAULT false,
    ip_address             INET,
    user_agent             TEXT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at             TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_web_sessions_player_id ON web_sessions(player_id);

-- ---------------------------------------------------------------------------
-- Ticket categories (docs/WEBSITE.md §8) -- a self-referential lookup table
-- (parent_id NULL = top-level category; non-NULL = a subcategory of that
-- row), not a hardcoded CHECK-constraint list, since a support team
-- realistically wants to add/rename categories over time without a code
-- deploy -- the same reasoning staff_ranks/arsenal_item_pools were made
-- DB-driven rather than hardcoded constants.
--
-- `key` is globally unique BY CONSTRUCTION (a subcategory's key is prefixed
-- with its parent's, e.g. gameplay_bug) rather than scoped per-parent via
-- UNIQUE(parent_id, key) -- that constraint wouldn't actually stop two
-- top-level rows from colliding, since Postgres treats every NULL as
-- distinct for uniqueness purposes (parent_id IS NULL for all top-level
-- rows never conflicts with itself).
-- ---------------------------------------------------------------------------
CREATE TABLE ticket_categories (
    id          SERIAL PRIMARY KEY,
    key         TEXT NOT NULL UNIQUE,
    label       TEXT NOT NULL,
    parent_id   INTEGER REFERENCES ticket_categories(id) ON DELETE CASCADE,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ticket_categories_parent_id ON ticket_categories(parent_id);

INSERT INTO ticket_categories (key, label, parent_id, sort_order) VALUES
    ('gameplay', 'Gameplay', NULL, 1),
    ('discord', 'Discord', NULL, 2),
    ('panel', 'Panel', NULL, 3),
    ('teamspeak', 'TeamSpeak', NULL, 4),
    ('other', 'Other', NULL, 5);

INSERT INTO ticket_categories (key, label, parent_id, sort_order)
    SELECT 'gameplay_bug', 'Bug Report', id, 1 FROM ticket_categories WHERE key = 'gameplay'
    UNION ALL SELECT 'gameplay_player_report', 'Player Report', id, 2 FROM ticket_categories WHERE key = 'gameplay'
    UNION ALL SELECT 'gameplay_ban_appeal', 'Ban Appeal', id, 3 FROM ticket_categories WHERE key = 'gameplay'
    UNION ALL SELECT 'gameplay_whitelist', 'Whitelist Application', id, 4 FROM ticket_categories WHERE key = 'gameplay'
    UNION ALL SELECT 'gameplay_economy', 'Economy / Item Issue', id, 5 FROM ticket_categories WHERE key = 'gameplay'
    UNION ALL SELECT 'gameplay_vehicle_property', 'Vehicle / Property Issue', id, 6 FROM ticket_categories WHERE key = 'gameplay';

INSERT INTO ticket_categories (key, label, parent_id, sort_order)
    SELECT 'discord_access', 'Access Issue', id, 1 FROM ticket_categories WHERE key = 'discord'
    UNION ALL SELECT 'discord_bot', 'Bot Issue', id, 2 FROM ticket_categories WHERE key = 'discord'
    UNION ALL SELECT 'discord_report_member', 'Report a Member', id, 3 FROM ticket_categories WHERE key = 'discord'
    UNION ALL SELECT 'discord_role_request', 'Role Request', id, 4 FROM ticket_categories WHERE key = 'discord';

INSERT INTO ticket_categories (key, label, parent_id, sort_order)
    SELECT 'panel_login', 'Login / Account Issue', id, 1 FROM ticket_categories WHERE key = 'panel'
    UNION ALL SELECT 'panel_bug', 'Bug Report', id, 2 FROM ticket_categories WHERE key = 'panel'
    UNION ALL SELECT 'panel_feature_request', 'Feature Request', id, 3 FROM ticket_categories WHERE key = 'panel'
    UNION ALL SELECT 'panel_discord_link', 'Discord Linking Issue', id, 4 FROM ticket_categories WHERE key = 'panel';

INSERT INTO ticket_categories (key, label, parent_id, sort_order)
    SELECT 'teamspeak_access', 'Access Issue', id, 1 FROM ticket_categories WHERE key = 'teamspeak'
    UNION ALL SELECT 'teamspeak_report_member', 'Report a Member', id, 2 FROM ticket_categories WHERE key = 'teamspeak'
    UNION ALL SELECT 'teamspeak_technical', 'Technical Issue', id, 3 FROM ticket_categories WHERE key = 'teamspeak';

-- A ticket is one row viewed from two access levels (owner in the member
-- portal, staff in the Support Panel) -- not two separate objects.
CREATE TABLE support_tickets (
    id                  BIGSERIAL PRIMARY KEY,
    player_id           BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    subject             TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'open'
                            CHECK (status IN ('open', 'pending', 'closed')),
    -- Submitter's own sense of urgency at creation time -- staff can
    -- re-triage (raise/lower) once they've actually looked at it; this is
    -- the initial signal, not a locked-in SLA commitment.
    priority            TEXT NOT NULL DEFAULT 'normal'
                            CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    -- category_id must reference a TOP-LEVEL row (parent_id IS NULL);
    -- subcategory_id, if set, must be a child of category_id. Enforced in
    -- application code (internal/handlers/tickets.go), not a DB
    -- constraint here -- a CHECK can't reference another row, and a
    -- trigger felt like more machinery than this needed given it's the
    -- write path's job either way.
    category_id         INTEGER NOT NULL REFERENCES ticket_categories(id),
    subcategory_id      INTEGER REFERENCES ticket_categories(id),
    assigned_staff_id   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    discord_thread_id   TEXT,              -- Discord thread this ticket mirrors to, docs/WEBSITE.md §9
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at           TIMESTAMPTZ
);

CREATE INDEX idx_support_tickets_player_id ON support_tickets(player_id);
CREATE INDEX idx_support_tickets_open ON support_tickets(status) WHERE status != 'closed';
CREATE INDEX idx_support_tickets_unassigned ON support_tickets(created_at) WHERE assigned_staff_id IS NULL AND status != 'closed';

CREATE TABLE support_ticket_messages (
    id                   BIGSERIAL PRIMARY KEY,
    ticket_id            BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    author_player_id     BIGINT REFERENCES players(id) ON DELETE SET NULL,  -- NULL = system message
    body                 TEXT NOT NULL,
    source               TEXT NOT NULL DEFAULT 'web' CHECK (source IN ('web', 'discord')),
    -- Staff-only note, never shown to the ticket's owner and never mirrored
    -- to Discord -- the classic "internal comment" every real support
    -- portal has, for staff to coordinate on a ticket without that
    -- coordination being part of the player-facing conversation. Can only
    -- be true on a message a staff member wrote (enforced in application
    -- code, not a DB constraint here, since that needs the ticket's
    -- support-panel-access check, not just a column relationship).
    internal             BOOLEAN NOT NULL DEFAULT false,
    discord_message_id   TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_support_ticket_messages_ticket_id ON support_ticket_messages(ticket_id);

-- One-time codes for linking a Discord account from the Discord side (a
-- `/link <code>` slash command), the mirror-image of the member portal's
-- OAuth2 "Connect Discord" flow -- either proves the same thing (this
-- Discord user and this player account belong to the same person), just
-- starting from the opposite end. The website generates the code (proving
-- portal-login/Steam identity); redeeming it in Discord proves Discord
-- identity; matching the two links the account. Single-use and short-lived
-- (expires_at, checked alongside used_at IS NULL on redemption) so a leaked
-- code has a narrow window and can't be replayed.
CREATE TABLE discord_link_codes (
    code        TEXT PRIMARY KEY,
    player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ
);

CREATE INDEX idx_discord_link_codes_player_id ON discord_link_codes(player_id);

-- Devlog posts -- public dev-blog entries, authored by staff through the
-- Admin Panel. `body` is plain text, not HTML/markdown: html/template
-- auto-escapes it on render, and the template applies white-space: pre-wrap
-- for paragraph breaks -- no markdown-to-HTML pipeline and no risk of a
-- staff-authored post accidentally (or maliciously, if an account is ever
-- compromised) injecting markup into a public page. `published_at IS NULL`
-- is a draft, not shown on any public route -- the column supports it, but
-- there's no draft/edit UI yet (DevlogCreate always publishes immediately),
-- so nothing in the app sets it NULL today; a draft with no way to ever
-- publish or change it later would just be a dead end, not a real feature.
-- `slug` is what the public URL uses (/devlog/<slug>), kept separate from
-- `id` so a URL never has to change if a post's id would (it won't, but
-- decoupling costs nothing).
CREATE TABLE devlog_posts (
    id             BIGSERIAL PRIMARY KEY,
    slug           TEXT NOT NULL UNIQUE,
    title          TEXT NOT NULL,
    body           TEXT NOT NULL,
    author_player_id BIGINT REFERENCES players(id) ON DELETE SET NULL,
    published_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_devlog_posts_published_at ON devlog_posts(published_at DESC) WHERE published_at IS NOT NULL;

-- Public status page history (src/website/internal/status). One row per
-- component per minute; pruned after 90 days by the checker itself.
CREATE TABLE status_checks (
    id          BIGSERIAL PRIMARY KEY,
    component   TEXT NOT NULL,          -- 'game', 'website', 'database', 'discord'
    ok          BOOLEAN NOT NULL,
    latency_ms  INTEGER,
    detail      TEXT,                   -- e.g. '7 / 64 players', or why it failed
    checked_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_status_checks_component_time ON status_checks(component, checked_at DESC);
CREATE INDEX idx_status_checks_time ON status_checks(checked_at);

-- ---------------------------------------------------------------------------
-- Triggers: keep players.*_bank in sync with bank_accounts.balance
-- (authoritative) automatically — a DB-enforced guarantee, not something
-- application code has to remember to do in two places.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION sync_bank_account_balance() RETURNS TRIGGER AS $$
BEGIN
    UPDATE bank_accounts SET balance = NEW.balance_after WHERE id = NEW.account_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_bank_transactions_sync_balance
    AFTER INSERT ON bank_transactions
    FOR EACH ROW EXECUTE FUNCTION sync_bank_account_balance();

CREATE OR REPLACE FUNCTION sync_players_bank_cache() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.faction = 'civilian' THEN
        UPDATE players SET civ_bank = NEW.balance WHERE id = NEW.player_id;
    ELSIF NEW.faction = 'police' THEN
        UPDATE players SET cop_bank = NEW.balance WHERE id = NEW.player_id;
    ELSIF NEW.faction = 'medic' THEN
        UPDATE players SET medic_bank = NEW.balance WHERE id = NEW.player_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_bank_accounts_sync_players_cache
    AFTER UPDATE OF balance ON bank_accounts
    FOR EACH ROW EXECUTE FUNCTION sync_players_bank_cache();

-- players.civ_bounty cache, synced from wanted_crimes the same way.
CREATE OR REPLACE FUNCTION sync_civ_bounty_cache() RETURNS TRIGGER AS $$
DECLARE
    v_player_id BIGINT := COALESCE(NEW.player_id, OLD.player_id);
BEGIN
    UPDATE players
    SET civ_bounty = (
        SELECT COALESCE(SUM(bounty_amount), 0)
        FROM wanted_crimes
        WHERE player_id = v_player_id AND cleared_at IS NULL
    )
    WHERE id = v_player_id;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_wanted_crimes_sync_bounty
    AFTER INSERT OR UPDATE OF cleared_at OR DELETE ON wanted_crimes
    FOR EACH ROW EXECUTE FUNCTION sync_civ_bounty_cache();

-- players.last_seen cache, synced from player_sessions on connect/disconnect.
CREATE OR REPLACE FUNCTION sync_last_seen_cache() RETURNS TRIGGER AS $$
BEGIN
    UPDATE players
    SET last_seen = COALESCE(NEW.disconnected_at, NEW.connected_at)
    WHERE id = NEW.player_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_player_sessions_sync_last_seen
    AFTER INSERT OR UPDATE OF disconnected_at ON player_sessions
    FOR EACH ROW EXECUTE FUNCTION sync_last_seen_cache();

-- Discord outbox (docs/DISCORD_BOT.md §8): messages a person must receive,
-- queued in the same transaction as the action; internal/discord delivers
-- them with retries. gave_up_at = permanently undeliverable (DMs closed,
-- unknown channel) or still failing after 24h.
CREATE TABLE discord_outbox (
    id            BIGSERIAL PRIMARY KEY,
    kind          TEXT NOT NULL CHECK (kind IN ('dm', 'channel_post')),
    target        TEXT NOT NULL,            -- user ID (dm) or channel ID
    payload       JSONB NOT NULL,
    dedupe_key    TEXT UNIQUE,
    attempts      INTEGER NOT NULL DEFAULT 0,
    next_attempt  TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at       TIMESTAMPTZ,
    gave_up_at    TIMESTAMPTZ,
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_discord_outbox_due ON discord_outbox(next_attempt) WHERE sent_at IS NULL AND gave_up_at IS NULL;

-- Server rules (public /rules, edited on /admin/rules; internal/rules).
-- The rulebook is one plain-text document ("# Section" headings, numbered
-- rules); every save is a new version and the latest is live. changed =
-- rule numbers that differ from the previous version, highlighted publicly.
CREATE TABLE rule_versions (
    id           BIGSERIAL PRIMARY KEY,
    body         TEXT NOT NULL,
    change_note  TEXT,
    changed      TEXT[] NOT NULL DEFAULT '{}',
    created_by   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Notifications with acknowledgement (docs/GAMEPANEL_PARITY.md §7.3;
-- internal/notify).
-- ---------------------------------------------------------------------------
-- An essential notice sent to many people at once (a policy or rule
-- change), so staff can see who has acknowledged it.
CREATE TABLE staff_notices (
    id          BIGSERIAL PRIMARY KEY,
    title       TEXT NOT NULL CHECK (length(title) BETWEEN 3 AND 120),
    body        TEXT NOT NULL,
    link        TEXT CHECK (link IS NULL OR (link LIKE '/%' AND link NOT LIKE '//%')),
    audience    TEXT NOT NULL DEFAULT 'staff' CHECK (audience IN ('staff', 'everyone')),
    created_by  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notifications (
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
CREATE INDEX idx_notifications_player ON notifications(player_id, id DESC);
CREATE INDEX idx_notifications_unread ON notifications(player_id) WHERE read_at IS NULL;
CREATE INDEX idx_notifications_unacked ON notifications(player_id) WHERE essential AND acknowledged_at IS NULL;

-- Discord bot layout (docs/DISCORD_BOT.md §3): channels and toggles set on
-- /admin/discord, plus "state.*" rows where the bot remembers its own
-- long-lived messages (welcome, live status) as "channelID/messageID".
CREATE TABLE discord_settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_by  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Change capture: every rank/role change is logged once, here, whichever
-- tool made it (website, Discord bot, game server, or a manual psql edit).
-- docs/INTEGRATIONS.md §3.
-- ---------------------------------------------------------------------------

CREATE TABLE rank_changes (
    id           BIGSERIAL PRIMARY KEY,
    player_id    BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    field        TEXT NOT NULL,          -- 'staff_rank_id', 'staff_status', 'staff_team', 'cop_level', 'medic_level', 'discord_id'
    old_value    TEXT,
    new_value    TEXT,
    source       TEXT NOT NULL CHECK (source IN ('website', 'discord', 'game', 'manual')),
    actor_id     BIGINT REFERENCES players(id) ON DELETE SET NULL,
    reason       TEXT,
    changed_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_rank_changes_player_id ON rank_changes(player_id, changed_at DESC);

-- Attribution comes from transaction-local settings the app sets before
-- writing (internal/audit.SetActor). The C++ extension identifies itself
-- via application_name instead. Anything else is a manual change.
CREATE OR REPLACE FUNCTION tasdyn_change_source() RETURNS TEXT AS $$
    SELECT COALESCE(
        NULLIF(current_setting('tasdyn.source', true), ''),
        CASE WHEN current_setting('application_name', true) = 'tasdyn-extension' THEN 'game' ELSE 'manual' END)
$$ LANGUAGE sql STABLE;

-- Human-readable label for a changed value, stored alongside the raw value
-- so the log still reads correctly after a rank is renamed or deleted.
CREATE OR REPLACE FUNCTION tasdyn_change_label(p_field TEXT, p_value TEXT) RETURNS TEXT AS $$
    SELECT CASE
        WHEN p_value IS NULL THEN 'none'
        WHEN p_field = 'staff_rank_id' THEN
            COALESCE((SELECT display_name FROM staff_ranks WHERE id = p_value::integer), 'rank #' || p_value)
        ELSE p_value
    END
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION log_player_changes() RETURNS TRIGGER AS $$
DECLARE
    v_source TEXT   := tasdyn_change_source();
    v_actor  BIGINT := NULLIF(current_setting('tasdyn.actor_id', true), '')::bigint;
    v_reason TEXT   := NULLIF(current_setting('tasdyn.reason', true), '');
    v_field  TEXT;
    v_old    TEXT;
    v_new    TEXT;
BEGIN
    FOR v_field, v_old, v_new IN
        SELECT * FROM (VALUES
            ('staff_rank_id', OLD.staff_rank_id::text, NEW.staff_rank_id::text),
            ('staff_status',  OLD.staff_status,        NEW.staff_status),
            ('staff_team',    OLD.staff_team,          NEW.staff_team),
            ('cop_level',     OLD.cop_level::text,     NEW.cop_level::text),
            ('medic_level',   OLD.medic_level::text,   NEW.medic_level::text),
            ('discord_id',    OLD.discord_id,          NEW.discord_id)
        ) AS v(field, old_value, new_value)
    LOOP
        CONTINUE WHEN v_old IS NOT DISTINCT FROM v_new;

        INSERT INTO rank_changes (player_id, field, old_value, new_value, source, actor_id, reason)
        VALUES (NEW.id, v_field, v_old, v_new, v_source, v_actor, v_reason);

        -- Linking/unlinking Discord is recorded for sync, but it's the
        -- player's own action, not a staff one -- no staff_log row.
        IF v_field <> 'discord_id' THEN
            INSERT INTO staff_log (staff_player_id, target_player_id, action, reason, before_value, after_value, source)
            VALUES (v_actor, NEW.id, 'rank_change:' || v_field, v_reason,
                    jsonb_build_object('value', v_old, 'label', tasdyn_change_label(v_field, v_old)),
                    jsonb_build_object('value', v_new, 'label', tasdyn_change_label(v_field, v_new)),
                    v_source);
        END IF;

        -- Wakes the website's role-sync engine for this player.
        PERFORM pg_notify('rank_changed', NEW.id::text);
    END LOOP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_players_log_changes
    AFTER UPDATE OF staff_rank_id, staff_status, staff_team, cop_level, medic_level, discord_id ON players
    FOR EACH ROW EXECUTE FUNCTION log_player_changes();

-- Role sync (docs/INTEGRATIONS.md §2.3): which platform group each
-- entitlement grants, and a log of every add/remove the sync makes.
-- Groups NOT listed here are never touched by sync.
CREATE TABLE platform_group_map (
    id           SERIAL PRIMARY KEY,
    platform     TEXT NOT NULL CHECK (platform IN ('discord', 'teamspeak')),
    entitlement  TEXT NOT NULL,     -- e.g. 'linked', 'staff_rank:admin', 'faction_rank:police:3'
    group_id     TEXT NOT NULL,     -- Discord role snowflake / TeamSpeak server group id
    UNIQUE (platform, entitlement, group_id)
);

CREATE TABLE sync_log (
    id              BIGSERIAL PRIMARY KEY,
    player_id       BIGINT REFERENCES players(id) ON DELETE SET NULL,
    platform        TEXT NOT NULL CHECK (platform IN ('discord', 'teamspeak')),
    action          TEXT NOT NULL CHECK (action IN ('add', 'remove', 'ban', 'unban', 'drift_reverted')),
    group_id        TEXT,
    rank_change_id  BIGINT REFERENCES rank_changes(id) ON DELETE SET NULL,  -- NULL = periodic/manual pass
    ok              BOOLEAN NOT NULL,
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sync_log_created ON sync_log(created_at DESC);
CREATE INDEX idx_sync_log_failures ON sync_log(created_at DESC) WHERE NOT ok;
-- Wakes internal/audit's poster so new staff_log rows reach #staff-log
-- promptly; it also polls, so a missed notification only delays a post.
CREATE OR REPLACE FUNCTION notify_staff_log() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('staff_log', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_staff_log_notify
    AFTER INSERT ON staff_log
    FOR EACH ROW EXECUTE FUNCTION notify_staff_log();

COMMIT;
