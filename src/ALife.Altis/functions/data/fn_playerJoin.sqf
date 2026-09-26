/*
    File: fn_playerJoin.sqf
    Author: Tasman Dynamics

    Description:
        Server-side player-join handling -- loads the record and opens the
        spawn menu. Triggered by initPlayerLocal.sqf's remoteExec rather
        than the engine's own initPlayerServer.sqf, which never actually
        fired for a real dedicated-server connection in this project --
        confirmed with an unconditional diagnostic log (no dependency on
        params/isServer/anything else) that never printed across multiple
        real test sessions, despite disconnect/autosave logging elsewhere
        proving the session was genuinely happening.

        Reviewed Tonic's AsYetUntitled/Framework for comparison: it has no
        initPlayerServer.sqf at all -- its entire player-join flow starts
        from initPlayerLocal.sqf instead, matching Bohemia's own wiki
        guidance to avoid initPlayerServer.sqf. This is that same "client
        requests, server decides" shape applied to join itself, not just
        gameplay actions -- consistent with everywhere else in this
        mission.

        Server-only -- allowlisted in CfgRemoteExec.hpp as
        ALife_fnc_playerJoin.

    Parameter(s):
        0: OBJECT - the player's unit
        1: STRING - uid
        2: BOOLEAN - didJIP

    Returns:
        Nothing
*/

params ["_player", "_uid", "_didJIP"];

if (!isServer) exitWith {};

diag_log format ["[ALife] player connecting: %1 (%2)%3",
    name _player, _uid, if (_didJIP) then {" [JIP]"} else {""}];

waitUntil { !isNil "ALife_fnc_load" };

private _record = [_player] call ALife_fnc_load;

if ((_record getOrDefault ["status", "ERROR"]) != "OK") exitWith {
    diag_log format ["[ALife] playerJoin: load failed for %1 (%2)", name _player, _uid];
    // TODO(#10): decide the actual failure policy before this ships — kick
    // with a clear message (safest — never let someone play on an unsaved
    // session), retry once, or let them in flagged as "unsaved" and retry
    // in the background. Not decided here on purpose; this is a product
    // call, not a technical one. Deliberately exitWith — a player with a
    // failed load does NOT get the spawn menu below.
};

diag_log format ["[ALife] player connected: %1 (%2) -- load OK", name _player, _uid];

// Server-side only, deliberately NOT public — a full record (cash, gear,
// position) broadcast to every client is exactly the kind of
// information-leakage ANTI_CHEAT.md's Vector 5 exists to avoid.
_player setVariable ["alife_record", _record];

// Targeted at this player specifically, not a broadcast -- same reasoning.
["open"] remoteExec ["ALife_fnc_spawnMenu", _player];
