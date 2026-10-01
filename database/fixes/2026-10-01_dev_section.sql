-- Development section: the dev.tools permission, the Lead Developer /
-- Developer / Trial Developer ranks and the project board table. Only adds
-- what's missing; safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-10-01_dev_section.sql

BEGIN;
-- Management (level 100) gets dev.tools; tick it for other ranks on
-- Roles & Permissions.
INSERT INTO rank_permissions (rank_id, command_key)
SELECT id, 'dev.tools' FROM staff_ranks WHERE level >= 100
ON CONFLICT DO NOTHING;

-- Development ranks (Admin → Development): project and tooling access
-- without moderation powers, so they get explicit grants rather than the
-- level-based seed above.
-- Each goes at its preferred level, or the nearest free level below it
-- when another rank already uses that one.
DO $$
DECLARE
    r record;
    lvl int;
BEGIN
    FOR r IN SELECT * FROM (VALUES ('lead_developer', 'Lead Developer', 48), ('developer', 'Developer', 45),
                                   ('trial_developer', 'Trial Developer', 35)) AS v(key, name, want) LOOP
        CONTINUE WHEN EXISTS (SELECT 1 FROM staff_ranks WHERE key = r.key);
        lvl := r.want;
        WHILE lvl > 1 AND EXISTS (SELECT 1 FROM staff_ranks WHERE level = lvl) LOOP
            lvl := lvl - 1;
        END LOOP;
        INSERT INTO staff_ranks (key, display_name, level, default_admin_panel, default_support_panel)
        VALUES (r.key, r.name, lvl, true, false);
    END LOOP;
END $$;

INSERT INTO rank_permissions (rank_id, command_key)
SELECT sr.id, g.command_key
FROM staff_ranks sr
JOIN (VALUES
    ('lead_developer', 'dev.tools'), ('lead_developer', 'staff.view'), ('lead_developer', 'server.logs'), ('lead_developer', 'database.query'),
    ('developer', 'dev.tools'), ('developer', 'staff.view'), ('developer', 'server.logs'),
    ('trial_developer', 'dev.tools'), ('trial_developer', 'staff.view')
) AS g(rank_key, command_key) ON sr.key = g.rank_key
ON CONFLICT DO NOTHING;

-- Admin → Development → Project board: the development to-do list.
CREATE TABLE IF NOT EXISTS dev_tasks (
    id           BIGSERIAL PRIMARY KEY,
    title        TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 140),
    body         TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'doing', 'review', 'done')),
    priority     TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    labels       TEXT[] NOT NULL DEFAULT '{}',
    assignee_id  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_by   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    sort         DOUBLE PRECISION NOT NULL DEFAULT 0,   -- order within its column
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    done_at      TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_dev_tasks_status_sort ON dev_tasks (status, sort);
COMMIT;
