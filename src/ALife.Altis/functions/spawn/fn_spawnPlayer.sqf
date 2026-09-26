/*
    File: fn_spawnPlayer.sqf
    Author: Tasman Dynamics

    Description:
        Authoritative spawn handling — "Client Requests, Server Decides"
        applied to spawning specifically. The client's chosen spawn point is
        a REQUEST, not a fact: if the player's stored <faction>_alive is
        false, their request is ignored outright and they're placed back at
        <faction>_position instead. This is the entire reason those columns
        exist (see database/schema.sql's JSONB SHAPE RULE comment) — a
        player who disconnected mid-death/arrest doesn't get to pick a
        fresh spawn on reconnect and walk away from it.

        Reuses the record initPlayerServer.sqf already loaded and stored on
        the unit (alife_record) rather than re-querying the DB — that data
        is already known-fresh for this session.

        Faction is "civ"/"cop"/"medic" throughout the spawn menu and this
        function -- matching docs/DATA_CONTRACT.md's players.<faction>_*
        field prefix directly, so there's no translation step between "what
        the player picked" and "what column that reads/writes."

        Gear/loadout equipping is deliberately NOT done here yet — civ_gear/
        cop_gear/medic_gear don't have a documented item-key contract yet
        (docs/DATA_CONTRACT.md only says "full loadout" in general terms).
        Write that contract before wiring gear application, same rule as
        everywhere else in this project.

        Server-only — allowlisted in CfgRemoteExec.hpp as ALife_fnc_spawnPlayer.

    Parameter(s):
        0: OBJECT - the player's unit
        1: STRING - uid
        2: STRING - faction ("civ" / "cop" / "medic")
        3: STRING - requested spawn point key (a CfgSpawnPoints class name,
                    a REQUEST, see above — not a marker name directly,
                    since config/spawn_config.hpp resolves that itself)

    Returns:
        Nothing
*/

params ["_unit", "_uid", "_faction", "_requestedSpawnKey"];

if (!isServer) exitWith {};

if !(_faction in ["civ", "cop", "medic"]) exitWith {
    diag_log format ["[ALife] spawnPlayer rejected -- invalid faction: %1", _faction];
};

private _record = _unit getVariable ["alife_record", createHashMapFromArray [["status", "ERROR"]]];

if ((_record getOrDefault ["status", "ERROR"]) != "OK") exitWith {
    diag_log format ["[ALife] spawnPlayer: no valid loaded record for uid %1", _uid];
};

private _wasAlive = _record getOrDefault [_faction + "_alive", true];

diag_log format ["[ALife] spawnPlayer: uid=%1 faction=%2 wasAlive=%3 requestedSpawn=%4",
    _uid, _faction, _wasAlive, _requestedSpawnKey];

if (!_wasAlive) then {
    // Ignore the requested marker entirely -- resume at the stored
    // position instead. Not a full "resume unconscious" simulation (that
    // needs a revive system that doesn't exist yet); this at minimum
    // means their death/arrest still happened somewhere, not nowhere.
    private _resumePos = [_record getOrDefault [_faction + "_position", []]] call ALife_fnc_parseStoredPosition;

    if (!isNil "_resumePos") then {
        _unit setPosATL _resumePos;
        diag_log format ["[ALife] spawnPlayer: uid=%1 resumed at stored %2_position", _uid, _faction];
    } else {
        // Nothing valid stored (fresh account, or -- shouldn't happen, but
        // never crash over it -- malformed data) -- fall back to this
        // faction's first configured spawn point rather than leaving the
        // player wherever they happened to load in.
        private _fallback = [_faction] call ALife_fnc_getSpawnPoints;
        if (count _fallback > 0) then {
            _unit setPosATL (_fallback select 0 select 2);
        };
        diag_log format ["[ALife] spawnPlayer: uid=%1 has no valid stored %2_position, used fallback spawn",
            _uid, _faction];
    };
    _unit setDamage 1;
} else {
    // Fresh spawn: re-derive the valid spawn point set server-side rather
    // than trusting the client's chosen key/position at all -- the client
    // only ever sends a key, never a position, so there's nothing to trust
    // there even in principle.
    private _validSpawnPoints = [_faction] call ALife_fnc_getSpawnPoints;
    private _matchIndex = _validSpawnPoints findIf { (_x select 0) == _requestedSpawnKey };

    if (_matchIndex == -1) exitWith {
        diag_log format ["[ALife] spawnPlayer rejected -- %1 is not a valid %2 spawn point for uid %3",
            _requestedSpawnKey, _faction, _uid];
    };

    // TODO: once gangs.gang_members exists in the loaded record, a
    // non-empty gang field (_validSpawnPoints select _matchIndex select 3)
    // needs to be checked against the player's actual gang membership here
    // -- currently unenforceable, see config/spawn_config.hpp.

    _unit setPosATL (_validSpawnPoints select _matchIndex select 2);
    diag_log format ["[ALife] spawnPlayer: uid=%1 spawned fresh as %2 at %3",
        _uid, _faction, _requestedSpawnKey];
};

_unit setVariable ["alife_activeFaction", _faction];
