/*
    File: initPlayerLocal.sqf

    Runs on the CLIENT for every connecting player (join and JIP). This
    replaces initPlayerServer.sqf as the join hook -- that file never
    actually fired for a real dedicated-server connection in this project
    (confirmed with an unconditional diagnostic log that never printed
    across multiple real test sessions, despite disconnect/autosave
    logging elsewhere proving the session was genuinely happening).
    Reviewed Tonic's AsYetUntitled/Framework for comparison: it has no
    initPlayerServer.sqf at all -- its whole player-join flow starts from
    initPlayerLocal.sqf, matching Bohemia's own wiki guidance to avoid
    initPlayerServer.sqf.

    Shows the connection loading screen (dialog/loadingScreen.hpp) while
    the load round-trip to the server completes, with its progress bar
    driven by the ACTUAL join sequence -- not a simulated timer. This file
    owns the client-side milestones (player object ready, join request
    sent); fn_playerJoin.sqf remoteExecs the server-side ones (DB record
    loading, load finished) back to this same client. Closed once the
    spawn menu actually opens -- see fn_spawnMenu.sqf's "open" mode.
*/

if (!hasInterface) exitWith {};

["open"] call ALife_fnc_loadingScreen;
["setProgress", 10, "Connecting to server..."] call ALife_fnc_loadingScreen;

waitUntil { !isNull player };

["setProgress", 25, "Player instance ready..."] call ALife_fnc_loadingScreen;

[player, getPlayerUID player, didJIP] remoteExec ["ALife_fnc_playerJoin", 2];

["setProgress", 40, "Requesting character data..."] call ALife_fnc_loadingScreen;
