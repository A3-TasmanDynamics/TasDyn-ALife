-- Personnel roster: badge, region, status, enrollment date, notes, and the
-- monthly roll call (RC / EA). Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-09-30_faction_roster.sql

BEGIN;
ALTER TABLE faction_log DROP CONSTRAINT IF EXISTS faction_log_kind_check;
ALTER TABLE faction_log ADD CONSTRAINT faction_log_kind_check CHECK (kind IN (
    'recruit', 'promote', 'demote', 'remove',
    'probation', 'training', 'discipline', 'discharge', 'blacklist', 'division', 'qual', 'rank_rules', 'settings',
    'roster', 'roll_call', 'drive'));

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

-- Existing members: enrollment date from their first recorded join, if any.
INSERT INTO faction_members (faction, player_id, enrolled_on)
SELECT f.faction, p.id,
       (SELECT min(changed_at)::date FROM rank_changes rc WHERE rc.player_id = p.id AND rc.field = f.col)
FROM players p CROSS JOIN (VALUES ('police', 'cop_level'), ('ems', 'medic_level')) AS f(faction, col)
WHERE (f.faction = 'police' AND p.cop_level > 0) OR (f.faction = 'ems' AND p.medic_level > 0)
ON CONFLICT DO NOTHING;
COMMIT;
