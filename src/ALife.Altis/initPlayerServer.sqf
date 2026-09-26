/*
    File: initPlayerServer.sqf

    Runs server-side once per player, on connect and on JIP. This is the
    join hook for docs/DATA_CONTRACT.md's "load" — see fn_load.sqf.
*/

params ["_player"];

if (!isServer) exitWith {};

// The functions library (CfgFunctions) isn't guaranteed to have finished
// compiling yet when initPlayerServer.sqf runs for the first/hosting
// player in a fast-starting session (singleplayer preview, locally-hosted
// MP) -- unlike a real dedicated server with a lobby wait, there's no
// guaranteed gap between mission start and the first player being "in."
// Confirmed via Eden Editor preview: ALife_fnc_load was still undefined at
// this exact line without this guard.
waitUntil { !isNil "ALife_fnc_load" };

private _record = [_player] call ALife_fnc_load;

if ((_record getOrDefault ["status", "ERROR"]) != "OK") exitWith {
    diag_log format ["[ALife] initPlayerServer: load failed for %1 (%2)", name _player, getPlayerUID _player];
    // TODO(#10): decide the actual failure policy before this ships — kick
    // with a clear message (safest — never let someone play on an unsaved
    // session), retry once, or let them in flagged as "unsaved" and retry
    // in the background. Not decided here on purpose; this is a product
    // call, not a technical one. Deliberately exitWith — a player with a
    // failed load does NOT get the spawn menu below.
};

// Server-side only, deliberately NOT public — a full record (cash, gear,
// position) broadcast to every client is exactly the kind of
// information-leakage ANTI_CHEAT.md's Vector 5 exists to avoid.
_player setVariable ["alife_record", _record];

// Targeted at this player specifically, not a broadcast -- same reasoning.
[] remoteExec ["ALife_fnc_spawnMenu", _player];
