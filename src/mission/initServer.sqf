/*
    File: initServer.sqf

    Server-level (not per-player) initialization. Currently just registers
    the HandleDisconnect hook that persists alive/death state and position
    on quit — see the JSONB SHAPE RULE / players comment in
    database/schema.sql for why this exists: without it, disconnecting
    while dead/unconscious respawns fresh on reconnect instead of resuming
    dead, a known exploit in this genre (confirmed via reviewing Tonic's
    AsYetUntitled/Framework, which persists the same state for the same
    reason).
*/

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
