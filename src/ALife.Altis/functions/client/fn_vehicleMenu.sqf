/*
    ALife_fnc_vehicleMenu (client)

    The radial menu for a vehicle you're looking at (Windows key,
    ALife_fnc_interact). Everyone:

        Unlock / Lock   - with the keys (its owner, or someone given a key)
        Trunk           - its storage (the server checks the lock)
        Repair          - with a Toolkit
        Push            - an empty vehicle or boat, away from you
        Give Keys       - the owner: to a player within 10 m
        Refuel          - with a Fuel Can
        Lockpick        - with a Lockpick, when it's locked

    Police also get Impound, Put in (the restrained player they're
    escorting) and Take out (restrained players inside). The server checks everything again
    (ALife_fnc_vehicleLock, ALife_fnc_containerOpen, ALife_fnc_useItem,
    ALife_fnc_vehicleAction).

    Modes:
        ["open", vehicle]
        ["keys", vehicle]   - who to give a key to
*/

params ["_mode", ["_vehicle", objNull]];

if (isNull _vehicle || { !alive _vehicle }) exitWith {};
private _uid = getPlayerUID player;
private _owner = (_vehicle getVariable ["alife_owner", ""]) == _uid;
private _hasKey = _owner || { _uid in (_vehicle getVariable ["alife_keys", []]) };
private _name = getText (configFile >> "CfgVehicles" >> typeOf _vehicle >> "displayName");
private _plate = _vehicle getVariable ["alife_plate", ""];
private _have = { private _i = ((missionNamespace getVariable ["ALife_phone", createHashMap]) getOrDefault ["items", []]) findIf { (_x select 0) == _this && { (_x select 1) > 0 } }; _i >= 0 };

switch (_mode) do {
    case "open": {
        private _options = [];
        if (_hasKey) then {
            _options pushBack [["Lock", "Unlock"] select (locked _vehicle >= 2), { [player, _this] remoteExecCall ["ALife_fnc_vehicleLock", 2]; }, _vehicle];
        };
        _options pushBack ["Trunk", { [player, _this] remoteExecCall ["ALife_fnc_containerOpen", 2]; }, _vehicle];
        if ((missionNamespace getVariable ["ALife_faction", "civ"]) == "cop") then {
            private _escorted = (attachedObjects player) select { _x isKindOf "CAManBase" && { _x getVariable ["alife_restrained", false] } };
            if (_escorted isNotEqualTo []) then {
                _options pushBack ["Put in", { [player, "putIn", _this] remoteExecCall ["ALife_fnc_personAction", 2]; }, _escorted select 0];
            };
            private _inside = (crew _vehicle) select { isPlayer _x && { _x getVariable ["alife_restrained", false] } };
            if (_inside isNotEqualTo []) then {
                _options pushBack ["Take out", { { [player, "takeOut", _x] remoteExecCall ["ALife_fnc_personAction", 2]; } forEach _this; }, _inside];
            };
        };
        _options pushBack ["Repair", { ["toolkit", _this] call ALife_fnc_useItem; }, _vehicle];
        if (crew _vehicle isEqualTo []) then {
            _options pushBack ["Push", { [player, "push", _this] remoteExecCall ["ALife_fnc_vehicleAction", 2]; }, _vehicle];
        };
        if (_owner) then {
            _options pushBack ["Give Keys", { ["keys", _this] call ALife_fnc_vehicleMenu; }, _vehicle];
        };
        if ("fuelcan" call _have) then {
            _options pushBack ["Refuel", { ["fuelcan", _this] call ALife_fnc_useItem; }, _vehicle];
        };
        if (!_hasKey && { locked _vehicle >= 2 } && { "lockpick" call _have }) then {
            _options pushBack ["Lockpick", { ["lockpick", _this] call ALife_fnc_useItem; }, _vehicle];
        };
        if ((missionNamespace getVariable ["ALife_faction", "civ"]) == "cop") then {
            _options pushBack ["Impound", { [player, "impound", _this] remoteExecCall ["ALife_fnc_vehicleAction", 2]; }, _vehicle];
        };
        ["open", _name, _options, _plate] call ALife_fnc_radial;
    };

    case "keys": {
        if (!_owner) exitWith {};
        private _players = allPlayers select { _x != player && { alive _x } && { !isObjectHidden _x } && { player distance _x <= 10 } };
        _players = [_players, [], { player distance _x }] call BIS_fnc_sortBy;
        private _options = (_players select [0, 7]) apply {
            [_x getVariable ["alife_name", name _x], { [player, "giveKey", _this select 0, _this select 1] remoteExecCall ["ALife_fnc_vehicleAction", 2]; }, [_vehicle, _x]]
        };
        if (_options isEqualTo []) exitWith { ["There's nobody within 10 m to give a key to.", "warning"] call ALife_fnc_toast; };
        _options pushBack ["Back", { ["open", _this] call ALife_fnc_vehicleMenu; }, _vehicle];
        ["open", "Give a key to", _options] call ALife_fnc_radial;
    };
};
