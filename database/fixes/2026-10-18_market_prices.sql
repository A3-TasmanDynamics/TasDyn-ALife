-- Dynamic market: the current price of each virtual item, kept across
-- server restarts. An item with no row is at its CfgMarket base price, so a
-- wipe or economy reset is just emptying this table (DELETE FROM
-- market_prices, or the extension's "market reset"). Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-10-18_market_prices.sql

BEGIN;
CREATE TABLE IF NOT EXISTS market_prices (
    item        TEXT PRIMARY KEY CHECK (item ~ '^[A-Za-z0-9_]{1,64}$'),  -- CfgMarket class
    price       NUMERIC(12, 2) NOT NULL CHECK (price > 0),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMIT;
