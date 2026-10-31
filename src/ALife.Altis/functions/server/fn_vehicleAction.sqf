/*
    ALife_fnc_vehicleAction (server, called by the client: the vehicle menu)

    Vehicle interactions from ALife_fnc_vehicleMenu, checked here:

        "push"     - an empty vehicle within 10 m, pushed away from the
                     player (where it's local: ALife_fnc_vehicleApply)
        "giveKey"  - the owner gives a key to a player within Keys >> giveRange
                     of them, wherever the vehicle is (also from the phone's
                     Keys app);
                     key holders can lock / unlock it and open the trunk
                     when it's locked (alife_keys, a list of Steam UIDs)
        "impound"  - police: an empty vehicle within 10 m is removed and
                     marked impounded in the vehicles table, with its fuel,
                     damage and trunk; the owner gets it back from a garage
                     for a fee (ALife_fnc_garage)

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - action
        2: OBJECT - the vehicle
        3: OBJECT - giveKey: who to give it to
*/

params [["_unit", objNull, [objNull]], ["_action", "", [""]], ["_vehicle", objNull, [objNull]], ["_target", objNull, [objNull]]];

if (!isServer || { isNull _unit } || { isNull _vehicle }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;
private _faction = _unit getVariable ["alife_faction", ""];
private _name = getText (configFile >> "CfgVehicles" >> typeOf _vehicle >> "displayName");

if (!alive _unit || { _faction == "" } || { !alive _vehicle }) exitWith {};
if (_action != "giveKey" && { _unit distance _vehicle > 10 }) exitWith { "Get closer to the vehicle." call _fnc_reply; };

switch (_action) do {
    case "push": {
        if (crew _vehicle isNotEqualTo []) exitWith { "Someone's in it." call _fnc_reply; };
        if (speed _vehicle > 5) exitWith {};
        private _dir = getPosASL _vehicle vectorDiff getPosASL _unit;
        _dir set [2, 0];
        if (vectorMagnitude _dir < 0.1) then { _dir = vectorDir _unit; _dir set [2, 0]; };
        private _boat = _vehicle isKindOf "Ship";
        private _velocity = (vectorNormalized _dir) vectorMultiply ([2.5, 4] select _boat);
        _velocity set [2, [0.4, 0.6] select _boat];
        if (local _vehicle) then { _vehicle setVelocity _velocity; } else { ["push", _vehicle, _velocity] remoteExecCall ["ALife_fnc_vehicleApply", _vehicle]; };
    };

    case "giveKey": {
        if ((_vehicle getVariable ["alife_owner", ""]) != _uid) exitWith { "Only the owner can give keys." call _fnc_reply; };
        private _giveRange = getNumber (missionConfigFile >> "CfgALife" >> "Keys" >> "giveRange");
        if (isNull _target || { !isPlayer _target } || { !alive _target } || { _target == _unit } || { _unit distance _target > _giveRange }) exitWith {
            format ["They need to be within %1 m of you.", _giveRange] call _fnc_reply;
        };
        private _keys = +(_vehicle getVariable ["alife_keys", []]);
        private _theirs = getPlayerUID _target;
        if (_theirs in _keys) exitWith { format ["%1 already has a key.", _target getVariable ["alife_name", name _target]] call _fnc_reply; };
        _keys pushBack _theirs;
        _vehicle setVariable ["alife_keys", _keys, true];
        diag_log format ["[ALife] %1 gave a key of %2 (%3) to %4", _uid, typeOf _vehicle, _vehicle getVariable ["alife_plate", ""], _theirs];
        [format ["Gave %1 a key to the %2.", _target getVariable ["alife_name", name _target], _name], "success"] call _fnc_reply;
        [format ["%1 gave you a key to their %2.", _unit getVariable ["alife_name", name _unit], _name], "success"] remoteExecCall ["ALife_fnc_notify", owner _target];
    };

    case "impound": {
        if (_faction != "cop") exitWith {};
        if (crew _vehicle isNotEqualTo []) exitWith { "Get everyone out of it first." call _fnc_reply; };
        private _id = _vehicle getVariable ["alife_vehicleId", ""];
        private _plate = _vehicle getVariable ["alife_plate", ""];
        private _ownerUid = _vehicle getVariable ["alife_owner", ""];
        if (_id != "") then {
            // Kept as it is (fuel, damage, trunk) until the owner pays to get it out of a garage.
            ([_vehicle] call ALife_fnc_vehicleState) params ["_fuel", "_damage", "_trunk"];
            private _result = ["vehicle", ["store", _id, "impounded", str _fuel, str _damage, str _trunk]] call ALife_fnc_extension;
            if (_result != "OK") then { diag_log format ["[ALife] impound of vehicle %1 not recorded: %2", _id, _result]; };
            ["vehicle", ["log", _id, _uid, "impound", ""]] call ALife_fnc_extension;
            if (!isNil "ALife_vehicleObjects") then { ALife_vehicleObjects deleteAt _id; };
        };
        private _container = _vehicle getVariable ["alife_container", ""];
        if (_container != "") then { ALife_containers deleteAt _container; };
        deleteVehicle _vehicle;
        diag_log format ["[ALife] %1 impounded %2 (id %3, plate %4, owner %5)", _uid, _name, _id, _plate, _ownerUid];
        [format ["Impounded the %1%2.", _name, ["", " (" + _plate + ")"] select (_plate != "")], "success"] call _fnc_reply;
        private _i = allPlayers findIf { getPlayerUID _x == _ownerUid };
        private _ownerUnit = if (_i < 0) then { objNull } else { allPlayers select _i };
        if (!isNull _ownerUnit && { _ownerUnit != _unit }) then {
            [format ["Your %1 has been impounded by the police.", _name], "warning"] remoteExecCall ["ALife_fnc_notify", owner _ownerUnit];
        };
    };
};
