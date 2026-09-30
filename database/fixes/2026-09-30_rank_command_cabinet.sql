-- Command and Cabinet ranks are now ticked per rank on Ranks & gear,
-- instead of every rank that can promote counting as command.
-- Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-09-30_rank_command_cabinet.sql
--
-- Nothing is ticked afterwards: tick the command and cabinet ranks on
-- /command/<faction>/ranks (as Head Admin, or the faction's top rank for
-- ranks below theirs). Until then nobody has the command panel.

BEGIN;
ALTER TABLE faction_rank_names ADD COLUMN IF NOT EXISTS is_command BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE faction_rank_names ADD COLUMN IF NOT EXISTS is_cabinet BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE faction_rank_names DROP CONSTRAINT IF EXISTS faction_rank_names_cabinet_check;
ALTER TABLE faction_rank_names ADD CONSTRAINT faction_rank_names_cabinet_check CHECK (NOT is_cabinet OR is_command);
COMMIT;
