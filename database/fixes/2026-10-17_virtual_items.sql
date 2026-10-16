-- Virtual items (medkit, ...) per faction: players.<faction>_items, saved
-- by the game server as [[class,count],...]. Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-10-17_virtual_items.sql

BEGIN;
ALTER TABLE players ADD COLUMN IF NOT EXISTS civ_items   JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE players ADD COLUMN IF NOT EXISTS cop_items   JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE players ADD COLUMN IF NOT EXISTS medic_items JSONB NOT NULL DEFAULT '[]'::jsonb;
COMMIT;
