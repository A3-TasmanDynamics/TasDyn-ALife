-- TasDyn-ALife canonical schema.
-- This is the ONE source of truth for the DB shape — see database/README.md.
-- Column names/order here are part of the contract in docs/DATA_CONTRACT.md
-- for `players`, and docs/ADMIN_TOOLS.md §3 for staff/admin/arsenal tables.
-- Changing a column referenced by either doc is a breaking change to the C++
-- extension and/or SQF side — update both docs and both sides in the same PR.

BEGIN;

-- ---------------------------------------------------------------------------
-- Staff ranks (docs/ADMIN_TOOLS.md §3) — DB-driven, not hardcoded constants.
-- ---------------------------------------------------------------------------
CREATE TABLE staff_ranks (
    id            SERIAL PRIMARY KEY,
    key           TEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL,
    level         INTEGER NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO staff_ranks (key, display_name, level) VALUES
    ('trial_mod', 'Trial Moderator', 10),
    ('moderator', 'Moderator', 20),
    ('admin', 'Admin', 40),
    ('head_admin', 'Head Admin / Developer', 100);

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
    uid            TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'active'
                       CHECK (status IN ('active', 'banned', 'whitelisted_pending')),
    last_seen      TIMESTAMPTZ,     -- cache, synced from player_sessions by trigger

    civ_cash       BIGINT NOT NULL DEFAULT 0,
    civ_bank       BIGINT NOT NULL DEFAULT 0,
    civ_licence    JSONB NOT NULL DEFAULT '[]'::jsonb,   -- array of licence keys, e.g. ["driver","boat"]
    civ_gear       JSONB NOT NULL DEFAULT '{}'::jsonb,   -- full loadout + virtual items
    civ_alive      BOOLEAN NOT NULL DEFAULT true,
    civ_position   JSONB,           -- last known world position; NULL until first save
    civ_bounty     BIGINT NOT NULL DEFAULT 0,            -- cache of wanted_crimes, see below
    civ_playtime_seconds BIGINT NOT NULL DEFAULT 0,

    cop_level      INTEGER NOT NULL DEFAULT 0,
    cop_dept       TEXT,
    cop_cash       BIGINT NOT NULL DEFAULT 0,
    cop_bank       BIGINT NOT NULL DEFAULT 0,
    cop_licence    JSONB NOT NULL DEFAULT '[]'::jsonb,
    cop_gear       JSONB NOT NULL DEFAULT '{}'::jsonb,
    cop_alive      BOOLEAN NOT NULL DEFAULT true,
    cop_position   JSONB,
    cop_playtime_seconds BIGINT NOT NULL DEFAULT 0,

    medic_level    INTEGER NOT NULL DEFAULT 0,
    medic_dept     TEXT,
    medic_cash     BIGINT NOT NULL DEFAULT 0,
    medic_bank     BIGINT NOT NULL DEFAULT 0,
    medic_licence  JSONB NOT NULL DEFAULT '[]'::jsonb,
    medic_gear     JSONB NOT NULL DEFAULT '{}'::jsonb,
    medic_alive    BOOLEAN NOT NULL DEFAULT true,
    medic_position JSONB,
    medic_playtime_seconds BIGINT NOT NULL DEFAULT 0,

    staff_rank_id  INTEGER REFERENCES staff_ranks(id) ON DELETE SET NULL,

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
    damage           JSONB NOT NULL DEFAULT '{}'::jsonb,  -- per-hitpoint map, e.g. {"hitengine": 0.4, "hitfuel": 0.0}
                                                            -- not a single float -- see getAllHitPointsDamage
    status           TEXT NOT NULL DEFAULT 'garaged'
                         CHECK (status IN ('garaged', 'spawned', 'impounded', 'destroyed')),
    garage_id        BIGINT REFERENCES garages(id) ON DELETE SET NULL,
    position         JSONB,           -- last known world position, NULL if garaged
    inventory        JSONB NOT NULL DEFAULT '{}'::jsonb,  -- cargo
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
    storage          JSONB NOT NULL DEFAULT '{}'::jsonb,
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
CREATE TABLE staff_permission_overrides (
    id           SERIAL PRIMARY KEY,
    player_id    BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    command_key  TEXT NOT NULL,
    allow        BOOLEAN NOT NULL,
    granted_by   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, command_key)
);

-- Every admin action — see docs/ADMIN_TOOLS.md §9.
CREATE TABLE staff_log (
    id                BIGSERIAL PRIMARY KEY,
    staff_player_id   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    target_player_id  BIGINT REFERENCES players(id) ON DELETE SET NULL,  -- NULL: no target (e.g. announcement)
    action            TEXT NOT NULL,
    reason            TEXT,
    before_value      JSONB,
    after_value       JSONB,
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
    flag_type     TEXT NOT NULL CHECK (flag_type IN ('honeypot', 'movement', 'idempotency_reject', 'rate_limit')),
    confidence    TEXT NOT NULL CHECK (confidence IN ('high', 'medium')),
    details       JSONB,
    reviewed_by   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    reviewed_at   TIMESTAMPTZ,
    resolution    TEXT CHECK (resolution IN ('kick', 'ban', 'dismiss')),  -- NULL = unreviewed
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_anti_cheat_flags_player_id ON anti_cheat_flags(player_id);
CREATE INDEX idx_anti_cheat_flags_unreviewed ON anti_cheat_flags(created_at) WHERE reviewed_at IS NULL;

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

COMMIT;
