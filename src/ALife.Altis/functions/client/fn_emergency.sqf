/*
    ALife_fnc_emergency (client)

    Lights and sirens on police and EMS vehicles (any vehicle their shops
    sell -- CarShops with side "cop" / "medic"). The driver works them:
    Left Shift + L the light bar (off, then each flash pattern in turn, then
    off), Left Shift + F the siren (the crew hear the cabin version),
    Left Shift + G one "whoop" to pull someone over (CfgALife >> Emergency).
    The
    state is on the vehicle (alife_lights / alife_siren, public), and every
    player's machine shows the flashing lights and plays the siren for
    vehicles near them -- so it's right for players who join later too.

    The siren goes off when the driver gets out; the lights stay on (a
    parked patrol car), and go off when the vehicle is destroyed.

    Modes:
        ["init"]            - from initPlayerLocal.sqf: keys + the watcher
        ["lights", vehicle] - next pattern / off (the driver)
        ["siren", vehicle]  - toggle (the driver)
        ["whoop", vehicle]  - one whoop, heard by everyone near (the driver)
*/

params [["_mode", "", [""]], ["_veh", objNull, [objNull]]];

if (!hasInterface) exitWith {};
private _cfg = missionConfigFile >> "CfgALife" >> "Emergency";

switch (_mode) do {
    case "init": {
        // Which classes are police / EMS vehicles: their shops' rows.
        ALife_emergencyTypes = createHashMap;
        {
            private _side = getText (_x >> "side");
            if (_side in ["cop", "medic"]) then {
                { ALife_emergencyTypes set [toLower (_x select 0), _side]; } forEach getArray (_x >> "vehicles");
            };
        } forEach ("true" configClasses (missionConfigFile >> "CarShops"));

        waitUntil { !isNull (findDisplay 46) };
        (findDisplay 46) displayAddEventHandler ["KeyDown", {
            params ["", "_key", "", "_ctrl", "_alt"];
            private _cfg = missionConfigFile >> "CfgALife" >> "Emergency";
            // Left Shift (initPlayerLocal.sqf tracks it; Arma's _shift is either one).
            if (_ctrl || _alt || { !(missionNamespace getVariable ["ALife_leftShift", false]) }
                || { !(_key in [getNumber (_cfg >> "lightsKey"), getNumber (_cfg >> "sirenKey"), getNumber (_cfg >> "whoopKey")]) }) exitWith { false };
            private _veh = vehicle player;
            if (_veh == player || { driver _veh != player } || { !alive player }) exitWith { false };
            private _type = ALife_emergencyTypes getOrDefault [toLower typeOf _veh, ""];
            if (_type == "" || { _type != (missionNamespace getVariable ["ALife_faction", "civ"]) }) exitWith { false };
            // Once per press (Arma repeats KeyDown while a key is held).
            if (isNil "ALife_emergencyKeys") then { ALife_emergencyKeys = createHashMap; };
            private _repeat = diag_tickTime - (ALife_emergencyKeys getOrDefault [_key, -10]) < 0.6;
            ALife_emergencyKeys set [_key, diag_tickTime];
            if (!_repeat) then {
                private _mode = switch (_key) do {
                    case (getNumber (_cfg >> "lightsKey")): { "lights" };
                    case (getNumber (_cfg >> "whoopKey")): { "whoop" };
                    default { "siren" };
                };
                [_mode, _veh] call ALife_fnc_emergency;
            };
            true
        }];
        (findDisplay 46) displayAddEventHandler ["KeyUp", {
            if (!isNil "ALife_emergencyKeys") then { ALife_emergencyKeys deleteAt (_this select 1); };
            false
        }];

        // The watcher: start the lights / siren of vehicles near you.
        [] spawn {
            private _cfg = missionConfigFile >> "CfgALife" >> "Emergency";
            while { true } do {
                {
                    private _v = _x;
                    if ((_v getVariable ["alife_lights", false]) && { !(_v getVariable ["alife_lightsShown", false]) } && { player distance _v < getNumber (_cfg >> "lightRange") }) then {
                        _v setVariable ["alife_lightsShown", true];
                        _v spawn ALife_fnc_emergencyLights;
                    };
                    if ((_v getVariable ["alife_siren", false]) && { !(_v getVariable ["alife_sirenPlaying", false]) } && { player distance _v < getNumber (_cfg >> "sirenRange") }) then {
                        _v setVariable ["alife_sirenPlaying", true];
                        _v spawn ALife_fnc_emergencySiren;
                    };
                } forEach (vehicles select { (_x getVariable ["alife_lights", false]) || { _x getVariable ["alife_siren", false] } });
                uiSleep 1;
            };
        };
    };

    case "lights": {
        // Off -> pattern 1 -> pattern 2 ... -> off. The pattern is public, so
        // everyone sees the same one; it changes on the next flash.
        private _patterns = getArray (_cfg >> "patterns");
        private _next = if (_veh getVariable ["alife_lights", false]) then { (_veh getVariable ["alife_lightPattern", 1]) + 1 } else { 1 };
        if (_next > count _patterns) exitWith {
            _veh setVariable ["alife_lights", false, true];
            ["Lights off.", "info", "", 2] call ALife_fnc_toast;
        };
        _veh setVariable ["alife_lightPattern", _next, true];
        _veh setVariable ["alife_lights", true, true];
        [format ["Lights: %1.", _patterns select (_next - 1)], "info", "", 2] call ALife_fnc_toast;
    };

    case "whoop": {
        // At most one every 1.5 s; heard by everyone near (playSound3D is global).
        if (diag_tickTime - (missionNamespace getVariable ["ALife_lastWhoop", -10]) < 1.5) exitWith {};
        ALife_lastWhoop = diag_tickTime;
        private _type = ALife_emergencyTypes getOrDefault [toLower typeOf _veh, "cop"];
        playSound3D [getText (_cfg >> _type >> "whoop"), _veh, false, getPosASL _veh, 3, 1, getNumber (_cfg >> "sirenRange")];
    };

    case "siren": {
        private _on = !(_veh getVariable ["alife_siren", false]);
        _veh setVariable ["alife_siren", _on, true];
        [["Siren off.", "Siren on."] select _on, "info", "", 2] call ALife_fnc_toast;
        // Off when you get out (or the vehicle's gone).
        if (_on) then {
            _veh spawn {
                waitUntil { uiSleep 0.5; isNull _this || { !alive _this } || { vehicle player != _this } || { !(_this getVariable ["alife_siren", false]) } };
                if (!isNull _this && { _this getVariable ["alife_siren", false] }) then { _this setVariable ["alife_siren", false, true]; };
            };
        };
    };
};
