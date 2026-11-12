-- Saved replies & templates: canned ticket replies, case-entry templates and
-- player-note templates the whole staff team can insert with one click
-- (/admin/replies). Placeholders like {player} are filled in when inserted.
CREATE TABLE IF NOT EXISTS saved_replies (
    id          BIGSERIAL PRIMARY KEY,
    scope       TEXT NOT NULL CHECK (scope IN ('ticket', 'case', 'note')),
    title       TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 80),
    body        TEXT NOT NULL CHECK (length(body) BETWEEN 1 AND 4000),
    created_by  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    updated_by  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (scope, title)
);

-- Managing them (add / edit / delete): Admin and up by default.
INSERT INTO rank_permissions (rank_id, command_key)
SELECT sr.id, 'replies.manage' FROM staff_ranks sr
WHERE sr.key IN ('admin', 'head_admin')
ON CONFLICT DO NOTHING;
