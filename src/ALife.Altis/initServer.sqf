/*
    File: initServer.sqf

    Server-level (not per-player) initialization:
      - Registers the HandleDisconnect hook that persists alive/death state
        and position on quit — see the JSONB SHAPE RULE / players comment in
        database/schema.sql for why this exists: without it, disconnecting
        while dead/unconscious respawns fresh on reconnect instead of
        resuming dead, a known exploit in this genre (confirmed via
        reviewing Tonic's AsYetUntitled/Framework, which persists the same
        state for the same reason). The actual save logic lives in
        fn_savePlayerState.sqf, shared with fn_sync.sqf's autosave below —
        HandleDisconnect never fires on an ungraceful crash, so relying on
        it alone leaves exactly that gap.
      - Starts fn_sync.sqf's periodic pulse (DB connection keep-alive +
        autosave of every connected player) — see that file.
      - Starts a periodic diag_fps log line, read by src/server_manager's
        Performance tab (see serverprocess.go's tailLog) — Arma exposes
        diag_fps only to script running inside the sim, there's no external
        query for it, so the mission has to be the one to report it.
*/

// [ALife][FPS] <number> -- server_manager's tailLog greps the live RPT
// log for this exact tag (regex in serverprocess.go) to feed the
// Performance tab's FPS graph. Every 2s, matching samplePerformance's own
// sample interval -- logging faster wouldn't be read any sooner, only
// bloat the RPT file.
[] spawn {
    while {true} do {
        diag_log format ["[ALife][FPS] %1", diag_fps];
        sleep 2;
    };
};

// Same function-library-compile race as initPlayerServer.sqf's
// ALife_fnc_load guard -- initServer.sqf runs just as early, with no
// guaranteed gap before this line executes.
[] spawn {
    waitUntil { !isNil "ALife_fnc_sync" };
    [] call ALife_fnc_sync;
};

addMissionEventHandler ["HandleDisconnect", {
    params ["_unit", "_id", "_uid", "_name"];
    [_unit, _uid] call ALife_fnc_savePlayerState;
    false  // AI doesn't take over the body -- respawn handling is Phase 2
}];
