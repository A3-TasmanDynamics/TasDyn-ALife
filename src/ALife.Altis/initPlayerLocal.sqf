// A player has joined: black screen, load the character, spawn menu.
// See functions\client\fn_startSession.sqf.

if (!hasInterface) exitWith {};

waitUntil { !isNil "ALife_fnc_startSession" };
[] spawn ALife_fnc_startSession;
[] spawn ALife_fnc_groupMarkers; // your police / EMS / gang on the map and GPS
[] spawn ALife_fnc_survival;     // food, water and the health / food / water HUD
[] spawn ALife_fnc_playerTags;   // names (and police ranks) over players you can see

// Phone key (CfgALife >> phoneKey, Y by default), the interaction key
// (CfgALife >> interactKeys, Windows): open a bag / trunk you're looking at,
// or your own inventory to drop things; and the lock key (lockKey, U):
// lock / unlock your vehicle; police: Left Shift + R (restrainKey) to
// restrain / release the player you're looking at; anyone: Left Shift + B
// (surrenderKey) hands on head, Left Shift + H (holsterKey) weapon away /
// out. While restrained, the phone, Windows key, lock key and Arma's
// inventory don't work.
[] spawn {
    waitUntil { !isNull (findDisplay 46) };
    (findDisplay 46) displayAddEventHandler ["KeyDown", {
        params ["", "_key", "_shift", "_ctrl", "_alt"];
        private _cfg = missionConfigFile >> "CfgALife";
        if ((player getVariable ["alife_restrained", false]) && { alive player }
            && { _key in (getArray (_cfg >> "interactKeys") + [getNumber (_cfg >> "lockKey"), getNumber (_cfg >> "phoneKey")] + actionKeys "Gear") }) exitWith { true };
        if (_key == 42) then { ALife_leftShift = true; }; // Left Shift (Arma's _shift is either one)
        // Arma repeats KeyDown while a key is held: the Left Shift toggles
        // (restrain, surrender, holster) act once per press, not per repeat.
        // A repeat = the same key again within 0.6 s with no KeyUp between
        // (repeats come every few hundredths; a missed KeyUp heals itself).
        if (isNil "ALife_keysHeld") then { ALife_keysHeld = createHashMap; };
        private _repeat = diag_tickTime - (ALife_keysHeld getOrDefault [_key, -10]) < 0.6;
        ALife_keysHeld set [_key, diag_tickTime];
        private _toggles = [getNumber (_cfg >> "restrainKey"), getNumber (_cfg >> "surrenderKey"), getNumber (_cfg >> "holsterKey")];
        if (_repeat && { missionNamespace getVariable ["ALife_leftShift", false] } && { _key in _toggles }) exitWith { true };
        if ((missionNamespace getVariable ["ALife_leftShift", false]) && { !_ctrl } && { !_alt } && { _key == getNumber (_cfg >> "restrainKey") }) exitWith {
            private _target = cursorObject;
            if ((missionNamespace getVariable ["ALife_faction", "civ"]) != "cop" || { !alive player } || { dialog }
                || { isNull _target } || { !isPlayer _target } || { !alive _target } || { player distance _target > 4 }) exitWith { false };
            [player, ["restrain", "unrestrain"] select (_target getVariable ["alife_restrained", false]), _target] remoteExecCall ["ALife_fnc_personAction", 2];
            true
        };
        if ((missionNamespace getVariable ["ALife_leftShift", false]) && { !_ctrl } && { !_alt } && { _key == getNumber (_cfg >> "surrenderKey") }) exitWith { call ALife_fnc_surrender };
        if ((missionNamespace getVariable ["ALife_leftShift", false]) && { !_ctrl } && { !_alt } && { _key == getNumber (_cfg >> "holsterKey") }) exitWith { call ALife_fnc_holster };
        if (_shift || _ctrl || _alt) exitWith { false };
        if (_key in getArray (missionConfigFile >> "CfgALife" >> "interactKeys")) exitWith { call ALife_fnc_interact };
        if (_key == getNumber (missionConfigFile >> "CfgALife" >> "lockKey")) exitWith {
            if (!alive player || { dialog }) exitWith { false };
            private _uid = getPlayerUID player;
            private _veh = [cursorObject, vehicle player] select (vehicle player != player);
            // Yours, or one you were given a key to.
            private _keyed = { (_this getVariable ["alife_owner", ""]) == _uid || { _uid in (_this getVariable ["alife_keys", []]) } };
            if (isNull _veh || { !(_veh call _keyed) } || { player distance _veh > 10 }) then {
                private _mine = (nearestObjects [player, ["LandVehicle", "Air", "Ship"], 10]) select { _x call _keyed };
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
    (findDisplay 46) displayAddEventHandler ["KeyUp", {
        params ["", "_key"];
        if (_key == 42) then { ALife_leftShift = false; };
        if (!isNil "ALife_keysHeld") then { ALife_keysHeld deleteAt _key; };
        false
    }];
};
