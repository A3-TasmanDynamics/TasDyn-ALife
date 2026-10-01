-- Project board, Trello-style: due dates, checklists, links between cards,
-- comments and activity. Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-10-01_dev_board_cards.sql

BEGIN;
-- Project board cards, Trello-style: due dates, checklists, links between
-- cards, comments and an activity history.
ALTER TABLE dev_tasks ADD COLUMN IF NOT EXISTS due_date DATE;

CREATE TABLE IF NOT EXISTS dev_task_checklists (
    id          BIGSERIAL PRIMARY KEY,
    task_id     BIGINT NOT NULL REFERENCES dev_tasks(id) ON DELETE CASCADE,
    title       TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 80),
    sort        DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_dev_task_checklists_task ON dev_task_checklists (task_id);

CREATE TABLE IF NOT EXISTS dev_task_checklist_items (
    id            BIGSERIAL PRIMARY KEY,
    checklist_id  BIGINT NOT NULL REFERENCES dev_task_checklists(id) ON DELETE CASCADE,
    body          TEXT NOT NULL CHECK (length(body) BETWEEN 1 AND 300),
    done          BOOLEAN NOT NULL DEFAULT false,
    done_by       BIGINT REFERENCES players(id) ON DELETE SET NULL,
    done_at       TIMESTAMPTZ,
    sort          DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_dev_task_checklist_items_list ON dev_task_checklist_items (checklist_id);

-- A link reads from task_id: "task_id blocks other_id", "relates to",
-- "duplicates". The other card shows the reverse ("blocked by"...).
CREATE TABLE IF NOT EXISTS dev_task_links (
    task_id     BIGINT NOT NULL REFERENCES dev_tasks(id) ON DELETE CASCADE,
    other_id    BIGINT NOT NULL REFERENCES dev_tasks(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('relates', 'blocks', 'duplicates')),
    created_by  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, other_id),
    CHECK (task_id <> other_id)
);
CREATE INDEX IF NOT EXISTS idx_dev_task_links_other ON dev_task_links (other_id);

CREATE TABLE IF NOT EXISTS dev_task_comments (
    id          BIGSERIAL PRIMARY KEY,
    task_id     BIGINT NOT NULL REFERENCES dev_tasks(id) ON DELETE CASCADE,
    author_id   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    body        TEXT NOT NULL CHECK (length(body) BETWEEN 1 AND 4000),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_dev_task_comments_task ON dev_task_comments (task_id);

CREATE TABLE IF NOT EXISTS dev_task_activity (
    id          BIGSERIAL PRIMARY KEY,
    task_id     BIGINT NOT NULL REFERENCES dev_tasks(id) ON DELETE CASCADE,
    actor_id    BIGINT REFERENCES players(id) ON DELETE SET NULL,
    what        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_dev_task_activity_task ON dev_task_activity (task_id);
COMMIT;
