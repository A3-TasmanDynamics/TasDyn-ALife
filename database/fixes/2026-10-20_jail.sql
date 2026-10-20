-- Jail: time left to serve and the bail (see schema.sql players). Safe to run
-- more than once.

ALTER TABLE players ADD COLUMN IF NOT EXISTS civ_jail_seconds INTEGER NOT NULL DEFAULT 0 CHECK (civ_jail_seconds >= 0);
ALTER TABLE players ADD COLUMN IF NOT EXISTS civ_bail BIGINT NOT NULL DEFAULT 0 CHECK (civ_bail >= 0);
