-- The garage brings a vehicle back in the colour / livery it was bought in:
-- vehicles.skin (see schema.sql). Safe to run more than once.

ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS skin TEXT NOT NULL DEFAULT '';
