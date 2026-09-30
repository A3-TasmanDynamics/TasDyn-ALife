-- Faction drive (folders, documents, uploaded files, versions). Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-09-30_faction_drive.sql

BEGIN;
ALTER TABLE faction_log DROP CONSTRAINT IF EXISTS faction_log_kind_check;
ALTER TABLE faction_log ADD CONSTRAINT faction_log_kind_check CHECK (kind IN (
    'recruit', 'promote', 'demote', 'remove',
    'probation', 'training', 'discipline', 'discharge', 'blacklist', 'division', 'qual', 'rank_rules', 'settings',
    'roster', 'roll_call', 'drive'));

-- Faction drive: folders, documents written in the built-in editor (SOPs,
-- training documents, policies) and uploaded files. division_key scopes an
-- item to a division; visibility 'members' is readable by the faction's
-- members (a division item: that division's members), 'command' by the
-- command panel only. Documents keep a version per save. Deleting archives.
CREATE TABLE IF NOT EXISTS faction_drive_folders (
    id            BIGSERIAL PRIMARY KEY,
    faction       TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    parent_id     BIGINT REFERENCES faction_drive_folders(id) ON DELETE CASCADE,
    name          TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    division_key  TEXT,
    visibility    TEXT NOT NULL DEFAULT 'members' CHECK (visibility IN ('members', 'command')),
    created_by    BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at   TIMESTAMPTZ,
    FOREIGN KEY (faction, division_key) REFERENCES faction_divisions(faction, key) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_faction_drive_folders ON faction_drive_folders(faction, parent_id);
CREATE TABLE IF NOT EXISTS faction_drive_items (
    id            BIGSERIAL PRIMARY KEY,
    faction       TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    folder_id     BIGINT REFERENCES faction_drive_folders(id) ON DELETE SET NULL,
    kind          TEXT NOT NULL CHECK (kind IN ('doc', 'file')),
    title         TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 120),
    category      TEXT NOT NULL DEFAULT 'other' CHECK (category IN ('sop', 'training', 'policy', 'other')),
    division_key  TEXT,
    visibility    TEXT NOT NULL DEFAULT 'members' CHECK (visibility IN ('members', 'command')),
    body_html     TEXT NOT NULL DEFAULT '',          -- documents (sanitised)
    file_name     TEXT NOT NULL DEFAULT '',          -- files
    file_type     TEXT NOT NULL DEFAULT '',
    file_size     INTEGER NOT NULL DEFAULT 0,
    file_data     BYTEA,
    created_by    BIGINT REFERENCES players(id) ON DELETE SET NULL,
    updated_by    BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at   TIMESTAMPTZ,
    FOREIGN KEY (faction, division_key) REFERENCES faction_divisions(faction, key) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_faction_drive_items ON faction_drive_items(faction, folder_id) WHERE archived_at IS NULL;
CREATE TABLE IF NOT EXISTS faction_drive_versions (
    id          BIGSERIAL PRIMARY KEY,
    item_id     BIGINT NOT NULL REFERENCES faction_drive_items(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    body_html   TEXT NOT NULL,
    edited_by   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_faction_drive_versions ON faction_drive_versions(item_id, id DESC);
COMMIT;
