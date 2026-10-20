/*
    ALife_fnc_garage (server, called by the client: the garage window)

    The garage at a vehicle shop (any CfgShops "vehicles" placement for the
    player's faction). Lists the player's vehicles for that faction, takes
    one out at the shop's spawn point (impounded ones for a release fee,
    from the bank -- CfgGarage), and stores an owned vehicle parked nearby.
    Vehicles come back with the fuel, damage, trunk and skin they had.

    Staff (the admin menu's "Their vehicles", game.vehicles or a server
    owner) can see every vehicle a player owns, in any of their garages,
    pull one out next to themselves (impounded or not, no fee) and send one
    back to its owner's garage (from the world or the impound), with its
    state saved. Logged to the staff log.

    Arguments:
        0: OBJECT - the player's unit (staff for the admin modes)
        1: STRING - "list", "retrieve" or "store"; "adminList", "adminPull", "adminStore"
        2: STRING - CfgShops placement (a vehicle shop); admin modes: the owner's uid
        3: STRING - retrieve / adminPull / adminStore: vehicles.id
*/

params [["_unit", objNull, [objNull]], ["_mode", "", [""]], ["_placement", "", [""]], ["_id", "", [""]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
if (isNil "ALife_vehicleObjects") then { ALife_vehicleObjects = createHashMap; };
private _name = { private _n = getText (configFile >> "CfgVehicles" >> _this >> "displayName"); [_this, _n] select (_n != "") };

// Staff: a player's vehicles in all their garages.
if (_mode in ["adminList", "adminPull", "adminStore"]) exitWith {
    if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit }) exitWith {};
    private _isOwner = _uid in getArray (missionConfigFile >> "CfgALife" >> "owners");
    if (!_isOwner && { !("game.vehicles" in ((_record getOrDefault ["staff_perms", []]) select { _x isEqualType "" })) }) exitWith {
        diag_log format ["[ALife] garage: %1 tried %2 without game.vehicles", _uid, _mode];
    };
    private _theirUid = _placement;
    if (count _theirUid != 17) exitWith {};
    private _fnc_log = {
        params ["_action", "_reason", ["_after", ""]];
        ["admin", ["log", _uid, _theirUid, _action, _reason, _after]] call ALife_fnc_extension;
    };
    private _fnc_all = {
        private _rows = [];
        {
            private _scope = _x;
            private _raw = ["vehicle", ["list", _theirUid, _scope]] call ALife_fnc_extension;
            if (_raw select [0, 6] != "ERROR:") then {
                {
                    _x params ["_vid", "_class", "_plate", "_status"];
                    private _object = ALife_vehicleObjects getOrDefault [_vid, objNull];
                    if (_status == "spawned" && { isNull _object || { !alive _object } }) then { _status = "garaged"; };
                    _rows pushBack [_vid, _class, _plate, _status, 0, _scope];
                } forEach parseSimpleArray _raw;
            };
        } forEach ["civ", "cop", "medic"];
        _rows
    };
    switch (_mode) do {
        case "adminList": { ["list", call _fnc_all] remoteExecCall ["ALife_fnc_garageMenu", _owner]; };
        case "adminPull": {
            private _object = ALife_vehicleObjects getOrDefault [_id, objNull];
            if (!isNull _object && { alive _object }) exitWith { "That vehicle is already out. Send it to the garage first." call _fnc_reply; };
            private _raw = ["vehicle", ["get", _id]] call ALife_fnc_extension;
            if (_raw select [0, 6] == "ERROR:") exitWith { "Couldn't find that vehicle." call _fnc_reply; };
            (parseSimpleArray _raw) params ["", "_ownerUid", "", "_class", "_plate", "_status", ["_fuel", 1], ["_damage", []], ["_trunk", []], ["_skin", ""]];
            if (_ownerUid != _theirUid || { _status == "destroyed" }) exitWith {};
            private _pos = _unit modelToWorld [0, 8, 0];
            if ((nearestObjects [_pos, ["LandVehicle", "Air", "Ship"], 5]) isNotEqualTo []) exitWith { "Something's in the way in front of you." call _fnc_reply; };
            private _r = ["vehicle", ["status", _id, "spawned"]] call ALife_fnc_extension;
            if (_r != "OK") exitWith { "Couldn't pull it out. Try again." call _fnc_reply; };
            [_class, [[_pos select 0, _pos select 1, 0], getDir _unit], _theirUid, _id, _plate, _skin, _fuel, _damage, _trunk] call ALife_fnc_vehicleSpawn;
            ["game.vehicle_pull", format ["%1 %2 (was %3)", _class call _name, _plate, _status], format ['{"vehicle": %1}', _id]] call _fnc_log;
            [format ["Pulled out their %1 (%2).", _class call _name, _plate], "success"] call _fnc_reply;
            ["list", call _fnc_all] remoteExecCall ["ALife_fnc_garageMenu", _owner];
        };
        case "adminStore": {
            private _object = ALife_vehicleObjects getOrDefault [_id, objNull];
            private _state = [];
            if (!isNull _object && { alive _object }) then {
                if ((crew _object) isNotEqualTo []) exitWith { _state = "busy"; };
                _state = [_object] call ALife_fnc_vehicleState;
            } else {
                // Already off the map (impounded / left over): keep what was saved.
                private _raw = ["vehicle", ["get", _id]] call ALife_fnc_extension;
                if (_raw select [0, 6] == "ERROR:") exitWith {};
                (parseSimpleArray _raw) params ["", "_ownerUid", "", "", "", "_status", ["_fuel", 1], ["_damage", []], ["_trunk", []]];
                if (_ownerUid == _theirUid && { _status != "destroyed" }) then { _state = [_fuel, _damage, _trunk]; };
            };
            if (_state isEqualTo "busy") exitWith { "Someone's in it." call _fnc_reply; };
            if (_state isEqualTo []) exitWith { "Couldn't find that vehicle." call _fnc_reply; };
            _state params ["_fuel", "_damage", "_trunk"];
            private _r = ["vehicle", ["store", _id, "garaged", str _fuel, str _damage, str _trunk]] call ALife_fnc_extension;
            if (_r != "OK") exitWith { "Couldn't store it. Try again." call _fnc_reply; };
            if (!isNull _object) then {
                private _container = _object getVariable ["alife_container", ""];
                if (_container != "") then { ALife_containers deleteAt _container; };
                deleteVehicle _object;
            };
            ALife_vehicleObjects deleteAt _id;
            ["game.vehicle_store", format ["vehicle %1", _id], format ['{"vehicle": %1}', _id]] call _fnc_log;
            ["Sent it back to their garage.", "success"] call _fnc_reply;
            ["list", call _fnc_all] remoteExecCall ["ALife_fnc_garageMenu", _owner];
        };
    };
};

private _faction = _unit getVariable ["alife_faction", ""];
private _info = [_placement] call ALife_fnc_shopInfo;
if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" }) exitWith {};
if (_info isEqualTo [] || { (_info select 0) != "vehicles" } || { getText ((_info select 2) >> "side") != _faction }) exitWith {};
_info params ["", "_place"];
if (!([_unit, _place] call ALife_fnc_atPlace)) exitWith { "You're not at the garage." call _fnc_reply; };
private _cfg = missionConfigFile >> "CfgGarage";
// The release fee: a share of what the vehicle costs in the shops.
private _fnc_fee = {
    private _price = 0;
    {
        { if ((_x select 0) == _this) exitWith { _price = _x select 2; }; } forEach getArray (_x >> "vehicles");
        if (_price > 0) exitWith {};
    } forEach ("true" configClasses (missionConfigFile >> "CarShops"));
    (round (_price * getNumber (_cfg >> "impoundRate"))) max getNumber (_cfg >> "impoundMin")
};
private _fnc_list = {
    private _raw = ["vehicle", ["list", _uid, _faction]] call ALife_fnc_extension;
    if (_raw select [0, 6] == "ERROR:") exitWith { diag_log format ["[ALife] garage list for %1 failed: %2", _uid, _raw]; [] };
    (parseSimpleArray _raw) apply {
        _x params ["_vid", "_class", "_plate", "_status"];
        // "Out" only while it's really in the world.
        private _object = ALife_vehicleObjects getOrDefault [_vid, objNull];
        if (_status == "spawned" && { isNull _object || { !alive _object } }) then { _status = "garaged"; };
        [_vid, _class, _plate, _status, [0, _class call _fnc_fee] select (_status == "impounded")]
    }
};

switch (_mode) do {
    case "list": {
        ["list", call _fnc_list] remoteExecCall ["ALife_fnc_garageMenu", _owner];
    };

    case "retrieve": {
        private _row = (call _fnc_list) select { (_x select 0) == _id };
        if (_row isEqualTo []) exitWith { "That vehicle isn't in your garage." call _fnc_reply; };
        (_row select 0) params ["", "_class", "_plate", "_status", "_fee"];
        if (_status == "spawned") exitWith { format ["Your %1 is already out.", _class call _name] call _fnc_reply; };

        // The shop's spawn point nearest the player, and it must be clear.
        private _spawns = getArray (_place >> "vehicleSpawns") apply { [_unit distance2D getMarkerPos _x, _x] };
        _spawns sort true;
        if (_spawns isEqualTo []) exitWith { "This garage has nowhere to put vehicles." call _fnc_reply; };
        private _marker = (_spawns select 0) select 1;
        if ((nearestObjects [getMarkerPos _marker, ["LandVehicle", "Air", "Ship"], 7]) isNotEqualTo []) exitWith {
            "The spawn point is blocked. Move the vehicle that's on it and try again." call _fnc_reply;
        };

        private _raw = ["vehicle", ["get", _id]] call ALife_fnc_extension;
        if (_raw select [0, 6] == "ERROR:") exitWith { diag_log format ["[ALife] garage get %1 failed: %2", _id, _raw]; "Couldn't get that vehicle out. Try again." call _fnc_reply; };
        (parseSimpleArray _raw) params ["", "_ownerUid", "", "", "", "", ["_fuel", 1], ["_damage", []], ["_trunk", []], ["_skin", ""]];
        if (_ownerUid != _uid) exitWith {};

        if (_status == "impounded" && { _fee > 0 }) then {
            private _paid = [_uid, _faction, _record, _fee, "bank", format ["Impound release: %1 %2", _class call _name, _plate]] call ALife_fnc_shopCharge;
            if (_paid != "OK") then { _fee = -1; };
        };
        if (_fee < 0) exitWith { format ["The impound release fee is $%1, from your bank. You don't have enough.", [(_row select 0) select 4] call BIS_fnc_numberText] call _fnc_reply; };

        private _r = ["vehicle", ["status", _id, "spawned"]] call ALife_fnc_extension;
        if (_r != "OK") exitWith { diag_log format ["[ALife] garage status %1 failed: %2", _id, _r]; "Couldn't get that vehicle out. Try again." call _fnc_reply; };
        if (_status == "impounded") then { ["vehicle", ["log", _id, _uid, "return", str _fee]] call ALife_fnc_extension; };
        [_class, _marker, _uid, _id, _plate, _skin, _fuel, _damage, _trunk] call ALife_fnc_vehicleSpawn;
        diag_log format ["[ALife] %1 took %2 (id %3, plate %4) out of the garage at %5%6", _uid, _class, _id, _plate, _placement, ["", format [" (impound fee $%1)", _fee]] select (_status == "impounded")];
        [format ["Your %1 is at the spawn point, locked: press U near it to unlock.%2", _class call _name,
            ["", format [" Paid $%1 to get it out of the impound.", [_fee] call BIS_fnc_numberText]] select (_status == "impounded")], "success"] call _fnc_reply;
        ["list", call _fnc_list] remoteExecCall ["ALife_fnc_garageMenu", _owner];
        [_unit] call ALife_fnc_requestPhoneData;
    };

    case "store": {
        // The nearest vehicle of theirs within storeRange, with nobody in it.
        private _range = getNumber (_cfg >> "storeRange");
        private _mine = (nearestObjects [_unit, ["LandVehicle", "Air", "Ship"], _range]) select {
            alive _x && { (_x getVariable ["alife_owner", ""]) == _uid } && { (_x getVariable ["alife_vehicleId", ""]) != "" }
        };
        if (_mine isEqualTo []) exitWith { format ["Park one of your vehicles within %1 m first.", _range] call _fnc_reply; };
        private _vehicle = _mine select 0;
        if ((crew _vehicle) isNotEqualTo []) exitWith { "Get everyone out of it first." call _fnc_reply; };
        private _vid = _vehicle getVariable ["alife_vehicleId", ""];
        ([_vehicle] call ALife_fnc_vehicleState) params ["_fuel", "_damage", "_trunk"];
        private _r = ["vehicle", ["store", _vid, "garaged", str _fuel, str _damage, str _trunk]] call ALife_fnc_extension;
        if (_r != "OK") exitWith { diag_log format ["[ALife] garage store %1 failed: %2", _vid, _r]; "Couldn't store it. Try again." call _fnc_reply; };
        ALife_vehicleObjects deleteAt _vid;
        private _container = _vehicle getVariable ["alife_container", ""];
        if (_container != "") then { ALife_containers deleteAt _container; };
        private _vname = (typeOf _vehicle) call _name;
        diag_log format ["[ALife] %1 stored %2 (id %3) at %4", _uid, typeOf _vehicle, _vid, _placement];
        deleteVehicle _vehicle;
        [format ["Stored your %1. Its trunk stays in it.", _vname], "success"] call _fnc_reply;
        ["list", call _fnc_list] remoteExecCall ["ALife_fnc_garageMenu", _owner];
    };
};
