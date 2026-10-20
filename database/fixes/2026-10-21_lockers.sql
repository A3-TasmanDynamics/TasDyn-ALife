-- Personal lockers for police / EMS (the in-game locker objects,
-- ALife_fnc_locker). Additive; safe to run more than once.

-- Personal lockers (police / EMS): what an officer keeps at the station,
-- per faction. gear = Arma gear, as the server reads it off the locker crate:
-- [["items", [[class, n], ...]], ["mags", [[class, ammo], ...]],
--  ["weapons", [weapon arrays]], ["containers", [[class, items, mags, weapons], ...]]];
-- items = virtual items [[class, count], ...] (config/CfgItems.hpp).
CREATE TABLE IF NOT EXISTS faction_lockers (
    player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    faction     TEXT NOT NULL CHECK (faction IN ('cop', 'medic')),
    gear        JSONB NOT NULL DEFAULT '[]'::jsonb,
    items       JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (player_id, faction)
);
