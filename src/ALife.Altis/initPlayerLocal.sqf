// A player has joined: black screen, load the character, spawn menu.
// See functions\client\fn_startSession.sqf.

if (!hasInterface) exitWith {};

waitUntil { !isNil "ALife_fnc_startSession" };
[] spawn ALife_fnc_startSession;

// Phone key (CfgALife >> phoneKey, Y by default), the interaction key
// (CfgALife >> interactKeys, Windows): open a bag / trunk you're looking at,
// or your own inventory to drop things; and the lock key (lockKey, U):
// lock / unlock your vehicle.
[] spawn {
    waitUntil { !isNull (findDisplay 46) };
    (findDisplay 46) displayAddEventHandler ["KeyDown", {
        params ["", "_key", "_shift", "_ctrl", "_alt"];
        if (_shift || _ctrl || _alt) exitWith { false };
        if (_key in getArray (missionConfigFile >> "CfgALife" >> "interactKeys")) exitWith { call ALife_fnc_interact };
        if (_key == getNumber (missionConfigFile >> "CfgALife" >> "lockKey")) exitWith {
            if (!alive player || { dialog }) exitWith { false };
            private _uid = getPlayerUID player;
            private _veh = [cursorObject, vehicle player] select (vehicle player != player);
            if (isNull _veh || { (_veh getVariable ["alife_owner", ""]) != _uid } || { player distance _veh > 10 }) then {
                private _mine = (nearestObjects [player, ["LandVehicle", "Air", "Ship"], 10]) select { (_x getVariable ["alife_owner", ""]) == _uid };
                _veh = _mine param [0, objNull];
            };
            if (isNull _veh) exitWith { false };
            [player, _veh] remoteExecCall ["ALife_fnc_vehicleLock", 2];
            true
        };
        if (_key != getNumber (missionConfigFile >> "CfgALife" >> "phoneKey")) exitWith { false };
        if (!(missionNamespace getVariable ["ALife_spawned", false]) || { !alive player } || { dialog }) exitWith { false };
        ["open"] call ALife_fnc_phone;
        true
    }];
};
