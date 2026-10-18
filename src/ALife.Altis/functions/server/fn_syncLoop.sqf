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

    if (!isNil "ALife_containers") then {
        private _lifetime = getNumber (missionConfigFile >> "CfgALife" >> "bagLifetime");
        private _expired = [];
        {
            private _object = _y get "object";
            if (isNull _object || { (_y get "kind") == "ground" && { time - (_y get "created") > _lifetime } }) then { _expired pushBack _x; };
        } forEach ALife_containers;
        {
            private _object = (ALife_containers get _x) get "object";
            ALife_groundBags = (missionNamespace getVariable ["ALife_groundBags", []]) - [_object];
            if (!isNull _object && { (_x select [0, 4]) == "bag_" }) then { deleteVehicle _object; };
            ALife_containers deleteAt _x;
        } forEach _expired;
        if (_expired isNotEqualTo []) then {
            publicVariable "ALife_groundBags";
            diag_log format ["[ALife] removed %1 old bag(s)/trunk(s)", count _expired];
        };
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
