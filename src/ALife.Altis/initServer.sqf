/*
    File: initServer.sqf

    Server-level (not per-player) initialization: registers the
    HandleDisconnect hook that persists alive/death state and position on
    quit — see the JSONB SHAPE RULE / players comment in database/schema.sql
    for why this exists: without it, disconnecting while dead/unconscious
    respawns fresh on reconnect instead of resuming dead, a known exploit
    in this genre (confirmed via reviewing Tonic's AsYetUntitled/Framework,
    which persists the same state for the same reason). Also starts a
    periodic diag_fps log line, read by src/server_manager's Performance
    tab (see serverprocess.go's tailLog) -- Arma exposes diag_fps only to
    script running inside the sim, there's no external query for it, so
    the mission has to be the one to report it.
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

addMissionEventHandler ["HandleDisconnect", {
    params ["_unit", "_id", "_uid", "_name"];

    // TODO(Phase 2): _unit needs a tracked "which faction is this player
    // currently playing" variable, set when they spawn — that system
    // doesn't exist yet (faction selection is Phase 2 gameplay work), so
    // there's nothing to read here until it does. Exits cleanly rather
    // than guessing.
    private _activeFaction = _unit getVariable ["alife_activeFaction", ""];
    if (_activeFaction == "") exitWith { false };

    private _pos = getPosATL _unit;
    private _positionValue = str [["x", _pos select 0], ["y", _pos select 1], ["z", _pos select 2]];
    private _token = (str serverTime) + "-" + _uid;

    [_uid, _activeFaction + "_alive", str (alive _unit), _token] call ALife_fnc_save;
    [_uid, _activeFaction + "_position", _positionValue, _token] call ALife_fnc_save;

    false  // AI doesn't take over the body -- respawn handling is Phase 2
}];
