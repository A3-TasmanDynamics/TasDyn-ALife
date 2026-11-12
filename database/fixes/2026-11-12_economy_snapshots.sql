-- Economy history for the admin panel's Economy & Crime charts: the website
-- records one snapshot an hour (internal/economy RunSnapshots). Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-11-12_economy_snapshots.sql

CREATE TABLE IF NOT EXISTS economy_snapshots (
    taken_at    TIMESTAMPTZ PRIMARY KEY DEFAULT now(),
    civ_cash    BIGINT NOT NULL,
    cop_cash    BIGINT NOT NULL,
    medic_cash  BIGINT NOT NULL,
    civ_bank    BIGINT NOT NULL,
    cop_bank    BIGINT NOT NULL,
    medic_bank  BIGINT NOT NULL,
    bounties    BIGINT NOT NULL,              -- outstanding wanted bounties
    market      JSONB NOT NULL DEFAULT '{}'   -- {item: buy price} for items off their base price
);
