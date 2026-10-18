/*
    ALife_fnc_startSession (client, scheduled)

    Runs on join (initPlayerLocal.sqf) and after every respawn
    (onPlayerRespawn.sqf): freezes and hides you behind the loading screen,
    waits for the server's database, then asks the server for your
    character. The answer arrives in ALife_fnc_sessionReceived.
*/

if (!hasInterface) exitWith {};

waitUntil { !isNull player && { !isNull (findDisplay 46) } };

ALife_spawned = false;
player allowDamage false;
player enableSimulation false;
ALife_sessionReply = nil;

["show", "Connecting to ALife..."] call ALife_fnc_loading;

// The server publishes these once its database is connected.
waitUntil {
    if (!(missionNamespace getVariable ["ALife_serverReady", false])) then {
        ["show", missionNamespace getVariable ["ALife_serverStatus", "Waiting for the server..."]] call ALife_fnc_loading;
        sleep 1;
    };
    missionNamespace getVariable ["ALife_serverReady", false]
};

for "_attempt" from 1 to 3 do {
    ["show", "Loading your character..."] call ALife_fnc_loading;
    [player] remoteExec ["ALife_fnc_requestSession", 2];

    private _deadline = diag_tickTime + 45;
    waitUntil { sleep 0.2; !isNil "ALife_sessionReply" || { diag_tickTime > _deadline } };
    if (!isNil "ALife_sessionReply") exitWith {};

    diag_log format ["[ALife] no session answer from the server (attempt %1/3)", _attempt];
};

if (isNil "ALife_sessionReply") then {
    ["error", "The server didn't answer.<br/>Please reconnect."] call ALife_fnc_loading;
};
