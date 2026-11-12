-- The police evidence vault (ALife_fnc_evidence): what police seize from the
-- ground (gear, virtual items, cash) is kept here until it's robbed or
-- destroyed. One row. Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-11-12_evidence_vault.sql

CREATE TABLE IF NOT EXISTS evidence_vault (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    gear        JSONB NOT NULL DEFAULT '[]'::jsonb,   -- locker gear format (items, mags, weapons, containers)
    items       JSONB NOT NULL DEFAULT '[]'::jsonb,   -- virtual items [[class, count], ...]
    cash        BIGINT NOT NULL DEFAULT 0 CHECK (cash >= 0),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
