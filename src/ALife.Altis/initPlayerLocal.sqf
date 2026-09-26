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

    Shows a brief "Welcome" screen while the load round-trip to the
    server completes (fn_playerJoin.sqf, remoteExec'd below), instead of
    leaving the player looking at whatever they happened to spawn next to
    with nothing visibly happening. Cleared once the spawn menu actually
    opens -- see fn_spawnMenu.sqf's "open" mode.
*/

if (!hasInterface) exitWith {};

cutText ["Welcome to TasDyn-ALife\nLoading your data...", "BLACK IN"];

waitUntil { !isNull player };

[player, getPlayerUID player, didJIP] remoteExec ["ALife_fnc_playerJoin", 2];
