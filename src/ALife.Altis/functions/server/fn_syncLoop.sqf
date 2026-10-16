/*
    ALife_fnc_syncLoop (server)

    Every 60 seconds: pings the database (the extension reconnects on its
    own if Postgres restarted) and autosaves every spawned player, so a
    server crash loses at most a minute.
*/

if (!isServer) exitWith {};

private _wasConnected = true;

while { true } do {
    sleep 60;

    private _ping = ["ping"] call ALife_fnc_extension;
    private _connected = _ping == "OK";

    if (_connected != _wasConnected) then {
        diag_log format ["[ALife] database connection %1: %2", ["lost", "restored"] select _connected, _ping];
        ALife_serverStatus = ["Waiting for the database: " + (_ping select [6]), "Ready."] select _connected;
        publicVariable "ALife_serverStatus";
        ALife_serverReady = _connected;
        publicVariable "ALife_serverReady";
        _wasConnected = _connected;
    };

    if (_connected) then {
        private _saved = 0;
        {
            if (alive _x && { [_x] call ALife_fnc_savePlayer }) then { _saved = _saved + 1; };
        } forEach allPlayers;
        if (_saved > 0) then {
            diag_log format ["[ALife] autosaved %1 player(s)", _saved];
        };
    };
};
