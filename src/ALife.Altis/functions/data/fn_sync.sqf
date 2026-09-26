/*
    File: fn_sync.sqf
    Author: Tasman Dynamics

    Description:
        Server-only periodic pulse -- spawned once from initServer.sqf, runs
        for the lifetime of the mission. Two jobs, both about not losing
        data to things HandleDisconnect can't cover:

        1. Keeps the DB connection alive. The C++ extension has no
           automatic reconnect of its own beyond what db.cpp's
           EnsureConnected does per-call (a network blip or a Postgres
           restart would otherwise silently fail every load/save for the
           rest of the server's uptime) -- this just calls "ping" on a
           schedule so a dropped connection gets noticed and logged rather
           than discovered the first time a player's load/save fails.

        2. Autosaves every actively-playing connected player's alive/
           position state (via ALife_fnc_savePlayerState, shared with
           HandleDisconnect) -- protects against an ungraceful server
           crash, which never fires the disconnect-time save at all.

        Reviewed an old local prototype's fn_autoSave.sqf/fn_serverPulse.sqf
        for the general "spawn one loop, sleep, act" shape -- not copied
        wholesale; this mission's own ALife_fnc_save (with its own
        allowlist/idempotency handling) does the actual persisting.

    Parameter(s):
        None

    Returns:
        Nothing (loops forever once spawned -- never call this directly,
        only [] spawn ALife_fnc_sync)
*/

if (!isServer) exitWith {};

private _intervalSeconds = 60;
private _wasConnected = true;

diag_log format ["[ALife] sync: started, %1s interval", _intervalSeconds];

while {true} do {
    sleep _intervalSeconds;

    private _isConnected = ("tasdyn_alife" callExtension ["ping", []]) == "OK";

    if (_isConnected != _wasConnected) then {
        diag_log format ["[ALife] sync: DB connection %1", ["lost", "restored"] select _isConnected];
        _wasConnected = _isConnected;
    };

    if (_isConnected) then {
        {
            if (isPlayer _x) then {
                [_x, getPlayerUID _x] call ALife_fnc_savePlayerState;
            };
        } forEach allPlayers;
    };
};
