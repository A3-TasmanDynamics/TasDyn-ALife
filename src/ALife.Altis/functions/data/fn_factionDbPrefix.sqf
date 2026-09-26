/*
    File: fn_factionDbPrefix.sqf
    Author: Tasman Dynamics

    Description:
        Maps a faction identifier ("civilian"/"police"/"medic" -- the
        identifier used throughout spawn/faction-selection and the bank
        subsystem, see database/schema.sql's bank_accounts.faction CHECK
        constraint) to the abbreviated prefix docs/DATA_CONTRACT.md actually
        specifies for the players table's per-faction fields (civ_cash,
        cop_alive, medic_position, ...). Two different, both-intentional
        naming conventions meeting at exactly one boundary -- every
        <faction>_alive / <faction>_position field name handed to
        ALife_fnc_load / ALife_fnc_save must go through this, so the
        mismatch can't silently drift back in (fn_spawnPlayer.sqf and
        initServer.sqf's HandleDisconnect both used to build "civilian_alive"
        / "police_position" directly -- never a real players column, so
        police/civilian death-position persistence silently no-opped).

    Parameter(s):
        0: STRING - "civilian" / "police" / "medic"

    Returns:
        STRING - "civ" / "cop" / "medic", or "" if unrecognized
*/

params ["_faction"];

switch (_faction) do {
    case "civilian": { "civ" };
    case "police": { "cop" };
    case "medic": { "medic" };
    default { "" };
};
