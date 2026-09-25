/*
    File: initPlayerServer.sqf

    Runs server-side once per player, on connect and on JIP. This is the
    join hook for docs/DATA_CONTRACT.md's "load" — see fn_load.sqf.
*/

params ["_player"];

if (!isServer) exitWith {};

private _record = [_player] call ALife_fnc_load;

if ((_record getOrDefault ["status", "ERROR"]) != "OK") then {
    diag_log format ["[ALife] initPlayerServer: load failed for %1 (%2)", name _player, getPlayerUID _player];
    // TODO(#10): decide the actual failure policy before this ships — kick
    // with a clear message (safest — never let someone play on an unsaved
    // session), retry once, or let them in flagged as "unsaved" and retry
    // in the background. Not decided here on purpose; this is a product
    // call, not a technical one.
};

// Server-side only, deliberately NOT public — a full record (cash, gear,
// position) broadcast to every client is exactly the kind of
// information-leakage ANTI_CHEAT.md's Vector 5 exists to avoid. Whatever
// subset the client's own UI actually needs should go back via a targeted
// remoteExec to just this player, not a publicVariable. Not built yet —
// there's no client-side UI to send it to until Phase 2.
_player setVariable ["alife_record", _record];
