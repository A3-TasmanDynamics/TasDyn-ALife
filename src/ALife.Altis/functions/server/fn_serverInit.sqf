/*
    ALife_fnc_serverInit (server)

    Runs once from initServer.sqf:
      - waits for the database (retrying), then tells clients the server is
        ready (ALife_serverReady / ALife_serverStatus are public, and late
        joiners receive them automatically)
      - saves a player's alive/position when they disconnect
      - marks a player dead in the database when they die
      - starts the periodic sync (ping + autosave)
      - logs FPS for the server manager's performance graph
*/

if (!isServer) exitWith {};

ALife_sessions = createHashMap; // uid -> loaded record (server only, never broadcast)

ALife_serverReady = false;
publicVariable "ALife_serverReady";
ALife_serverStatus = "Connecting to the database...";
publicVariable "ALife_serverStatus";

// "[ALife][FPS] <n>" is read by the server manager (serverprocess.go).
[] spawn {
    while { true } do {
        diag_log format ["[ALife][FPS] %1", diag_fps];
        sleep 2;
    };
};

addMissionEventHandler ["HandleDisconnect", {
    params ["_unit", "_id", "_uid", "_name"];
    diag_log format ["[ALife] %1 (%2) disconnected", _name, _uid];
    [_unit, _uid] call ALife_fnc_savePlayer;
    ALife_sessions deleteAt _uid;
    false // don't hand the body to AI
}];

addMissionEventHandler ["EntityKilled", {
    params ["_unit"];
    private _faction = _unit getVariable ["alife_faction", ""];
    private _uid = _unit getVariable ["alife_uid", ""];
    if (_faction == "" || { _uid == "" }) exitWith {};

    // Dead: next join starts from the spawn menu, not "last position".
    _unit setVariable ["alife_faction", nil];
    private _result = ["save", [_uid, _faction + "_alive", "false", "-"]] call ALife_fnc_extension;
    diag_log format ["[ALife] %1 died as %2 -- saved: %3", _uid, _faction, _result];

    private _record = ALife_sessions getOrDefault [_uid, createHashMap];
    _record set [_faction + "_alive", false];
}];

// Wait for the database, then open the doors.
[] spawn {
    private _attempt = 0;
    while { true } do {
        _attempt = _attempt + 1;
        private _ping = ["ping"] call ALife_fnc_extension;
        if (_ping == "OK") exitWith {
            ALife_serverStatus = "Ready.";
            publicVariable "ALife_serverStatus";
            ALife_serverReady = true;
            publicVariable "ALife_serverReady";
            diag_log format ["[ALife] database connected (attempt %1) -- server ready", _attempt];
        };
        ALife_serverStatus = format ["Waiting for the database: %1", _ping select [6]];
        publicVariable "ALife_serverStatus";
        diag_log format ["[ALife] database not ready (attempt %1): %2", _attempt, _ping];
        sleep 5;
    };

    [] call ALife_fnc_syncLoop;
};
