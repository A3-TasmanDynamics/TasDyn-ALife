-- Vehicles can be sold back at a garage (ALife_fnc_garage "sell"): status
-- 'sold', kept for its history (vehicle_logs 'sell'; the money is a bank
-- 'payout'). Safe to re-run.
--
--   psql -U alife_admin -d alife_db -f database/fixes/2026-11-12_vehicle_sold.sql

BEGIN;
ALTER TABLE vehicles DROP CONSTRAINT IF EXISTS vehicles_status_check;
ALTER TABLE vehicles ADD CONSTRAINT vehicles_status_check
    CHECK (status IN ('garaged', 'spawned', 'impounded', 'destroyed', 'sold'));
COMMIT;
