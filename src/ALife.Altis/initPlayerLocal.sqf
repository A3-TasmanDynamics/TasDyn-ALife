// A player has joined: black screen, load the character, spawn menu.
// See functions\client\fn_startSession.sqf.

if (!hasInterface) exitWith {};

waitUntil { !isNil "ALife_fnc_startSession" };
[] spawn ALife_fnc_startSession;

// Phone key (CfgALife >> phoneKey, Y by default) and the interaction key
// (CfgALife >> interactKeys, Windows): open a bag / trunk you're looking at,
// or your own inventory to drop things.
[] spawn {
    waitUntil { !isNull (findDisplay 46) };
    (findDisplay 46) displayAddEventHandler ["KeyDown", {
        params ["", "_key", "_shift", "_ctrl", "_alt"];
        if (_shift || _ctrl || _alt) exitWith { false };
        if (_key in getArray (missionConfigFile >> "CfgALife" >> "interactKeys")) exitWith { call ALife_fnc_interact };
        if (_key != getNumber (missionConfigFile >> "CfgALife" >> "phoneKey")) exitWith { false };
        if (!(missionNamespace getVariable ["ALife_spawned", false]) || { !alive player } || { dialog }) exitWith { false };
        ["open"] call ALife_fnc_phone;
        true
    }];
};
