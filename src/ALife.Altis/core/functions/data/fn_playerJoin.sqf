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

        Also pushes real progress milestones back to this specific client's
        loading screen (ALife_fnc_loadingScreen, core/functions/ui/) as the
        actual DB load happens -- initPlayerLocal.sqf owns the client-side
        milestones before this function is even called; this is the
        server-side half of the same real (not simulated) progress bar.

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

["setProgress", 60, "Loading character profile from database..."] remoteExec ["ALife_fnc_loadingScreen", _player];

waitUntil { !isNil "ALife_fnc_load" };

private _record = [_player] call ALife_fnc_load;

// Banned (docs/DATA_CONTRACT.md "load"): never reach the spawn menu. The
// server decides this, so a modified client can't skip it -- without the
// spawn menu there's no character to play.
if ((_record getOrDefault ["status", "ERROR"]) == "BANNED") exitWith {
    private _until = _record getOrDefault ["until", ""];
    diag_log format ["[ALife] playerJoin: refused banned player %1 (%2)", name _player, _uid];
    ["setProgress", 100, format ["You are banned from this server%1. Reason: %2. You can appeal on the website.",
        if (_until == "") then {" permanently"} else {" until " + _until},
        _record getOrDefault ["reason", "not given"]]] remoteExec ["ALife_fnc_loadingScreen", _player];
};

if ((_record getOrDefault ["status", "ERROR"]) != "OK") exitWith {
    diag_log format ["[ALife] playerJoin: load failed for %1 (%2)", name _player, _uid];
    // TODO(#10): decide the actual failure policy before this ships — kick
    // with a clear message (safest — never let someone play on an unsaved
    // session), retry once, or let them in flagged as "unsaved" and retry
    // in the background. Not decided here on purpose; this is a product
    // call, not a technical one. Deliberately exitWith — a player with a
    // failed load does NOT get the spawn menu below. At minimum the
    // loading screen now says so, rather than hanging silently forever on
    // "Loading character profile..." with no visible sign anything is
    // wrong (the underlying stuck-behind-the-loading-screen problem is
    // still open -- see src/ALife.Altis/README.md's Open Items).
    ["setProgress", 100, "Failed to load your character data -- please reconnect"] remoteExec ["ALife_fnc_loadingScreen", _player];
};

diag_log format ["[ALife] player connected: %1 (%2) -- load OK", name _player, _uid];

["setProgress", 90, "Preparing spawn point selection..."] remoteExec ["ALife_fnc_loadingScreen", _player];

// Server-side only, deliberately NOT public — a full record (cash, gear,
// position) broadcast to every client is exactly the kind of
// information-leakage ANTI_CHEAT.md's Vector 5 exists to avoid.
_player setVariable ["alife_record", _record];

// Targeted at this player specifically, not a broadcast -- same reasoning.
["open"] remoteExec ["ALife_fnc_spawnMenu", _player];
