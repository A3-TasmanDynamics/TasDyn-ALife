/*
    ALife_fnc_requestUseItem (server, called by the client)

    A player finished using a virtual item (ALife_fnc_useItem). Checks it
    again -- spawned, alive, the item has a `use`, the vehicle is close and
    needs it -- uses one up (ALife_fnc_itemsChange) and applies it:

      consume / heal  their health (and stamina), on their machine
      repair          the vehicle, fully
      refuel          + `fuel` of a tank, where the vehicle is local
      lockpick        unlocked with `chance`; the lockpick is used up either way

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - item class (CfgItems)
        2: OBJECT - the vehicle (repair / refuel / lockpick), else objNull
*/

params [["_unit", objNull, [objNull]], ["_class", "", [""]], ["_target", objNull, [objNull]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
private _cfg = missionConfigFile >> "CfgItems" >> _class;
private _use = getText (_cfg >> "use");
private _name = getText (_cfg >> "displayName");

if ((_record getOrDefault ["status", ""]) != "OK" || { _faction == "" } || { !alive _unit } || { _unit getVariable ["alife_downed", false] }) exitWith {};
if (!isClass _cfg || { !(_use in ["consume", "heal", "repair", "refuel", "lockpick", "place"]) }) exitWith {};
// Placeables: the police only, and a cap per player (CfgALife >> Placeables).
if (_use == "place") then {
    if (_faction != "cop") exitWith { _use = ""; "Only the police can put those down." call _fnc_reply; };
    if (vehicle _unit != _unit) exitWith { _use = ""; "Get out of the vehicle first." call _fnc_reply; };
    if (isNil "ALife_placed") then { ALife_placed = []; };
    ALife_placed = ALife_placed select { !isNull _x };
    if (({ (_x getVariable ["alife_placed", []]) param [0, ""] == _uid } count ALife_placed) >= getNumber (missionConfigFile >> "CfgALife" >> "Placeables" >> "max")) exitWith {
        _use = ""; "You've put down as many as you can. Pick some up first." call _fnc_reply;
    };
};
if (_use == "") exitWith {};

private _onVehicle = _use in ["repair", "refuel", "lockpick"];
if (_onVehicle && { isNull _target || { !alive _target } || { !(_target isKindOf "AllVehicles") } || { _target isKindOf "Man" } || { _unit distance _target > 10 } }) exitWith {
    "Stand next to a vehicle to use that." call _fnc_reply;
};
private _refuse = switch (_use) do {
    case "refuel": { ["", "It's already full."] select (fuel _target > 0.99) };
    case "lockpick": { ["", "It isn't locked."] select (locked _target < 2) };
    default { "" };
};
if (_refuse != "") exitWith { _refuse call _fnc_reply; };

if (!([_uid, _faction, _class, -1] call ALife_fnc_itemsChange)) exitWith {
    format ["You don't have a %1.", _name] call _fnc_reply;
};

private _vehName = if (isNull _target) then { "" } else { getText (configFile >> "CfgVehicles" >> typeOf _target >> "displayName") };
private _message = "";
switch (_use) do {
    case "consume";
    case "heal": {
        ["heal", _unit, getNumber (_cfg >> "heal"), getNumber (_cfg >> "energy") == 1, getNumber (_cfg >> "food"), getNumber (_cfg >> "water")] remoteExecCall ["ALife_fnc_itemUsed", _owner];
        _message = ["You feel better.", format ["You had the %1.", _name]] select (_use == "consume");
    };
    case "repair": {
        _target setDamage 0;
        _message = format ["Repaired the %1.", _vehName];
    };
    case "refuel": {
        private _amount = getNumber (_cfg >> "fuel");
        if (local _target) then { _target setFuel ((fuel _target + _amount) min 1); } else { ["fuel", _target, _amount] remoteExecCall ["ALife_fnc_itemUsed", _target]; };
        _message = format ["Refuelled the %1.", _vehName];
    };
    case "place": {
        private _pos = _unit modelToWorld [0, 2, 0];
        private _object = createVehicle [getText (_cfg >> "object"), [_pos select 0, _pos select 1, 0], [], 0, "CAN_COLLIDE"];
        _object setDir getDir _unit;
        _object setPosATL [_pos select 0, _pos select 1, 0];
        _object allowDamage false;
        _object setVariable ["alife_placed", [_uid, _class], true]; // who, and what it goes back to
        if (isNil "ALife_placed") then { ALife_placed = []; };
        ALife_placed pushBack _object;
        if (getNumber (_cfg >> "spikes") == 1) then { [_object] call ALife_fnc_placeables; };
        _object spawn {
            sleep getNumber (missionConfigFile >> "CfgALife" >> "Placeables" >> "lifetime");
            if (!isNull _this) then { deleteVehicle _this; };
        };
        _message = format ["Put down the %1. Windows on it to pick it up.", _name];
    };
    case "lockpick": {
        if (random 1 < getNumber (_cfg >> "chance")) then {
            if (local _target) then { _target lock 0; } else { [_target, 0] remoteExecCall ["ALife_fnc_vehicleLockApply", _target]; };
            _message = format ["You picked the lock of the %1.", _vehName];
        } else {
            _message = "The lockpick broke.";
        };
    };
};

diag_log format ["[ALife] %1 used %2 (%3) on %4", _uid, _class, _use, [_target, typeOf _target] select (!isNull _target)];
[_message, ["success", "warning"] select (_message == "The lockpick broke.")] call _fnc_reply;
[_unit] call ALife_fnc_requestPhoneData;
