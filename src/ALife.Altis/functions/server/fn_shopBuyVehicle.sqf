/*
    ALife_fnc_shopBuyVehicle (server, called by the client)

    Buys a vehicle from a vehicle shop (ALife_fnc_vehicleShop). Checked
    here: faction, that the player is at the shop, that the shop sells it
    (CarShops row), licence / rank (shop and row), the colour, a clear spawn
    point, and payment. Then it's spawned at the shop's vehicleSpawns marker
    nearest the player, in the chosen colour, locked, owned by the player
    (alife_owner) and recorded in the vehicles table with a plate.

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - shop placement (CfgShops) of a vehicle shop
        2: STRING - classname
        3: STRING - colour: a TextureSources class of the vehicle, "" = default
        4: STRING - "cash" or "bank"
*/

params [["_unit", objNull, [objNull]], ["_placement", "", [""]], ["_class", "", [""]], ["_colour", "", [""]], ["_method", "", [""]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };

private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
private _info = [_placement] call ALife_fnc_shopInfo;

if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" }) exitWith {};
if (_info isEqualTo [] || { (_info select 0) != "vehicles" } || { !(_method in ["cash", "bank"]) }) exitWith {};
_info params ["", "_place", "_shop"];

private _rows = getArray (_shop >> "vehicles");
private _index = _rows findIf { (_x select 0) == _class };
if (_index < 0 || { getText (_shop >> "side") != _faction }) exitWith { "You can't buy that here." call _fnc_reply; };
private _radius = getNumber (_place >> "radius");
if ((getArray (_place >> "markers") findIf { _unit distance2D getMarkerPos _x <= _radius }) < 0) exitWith {
    "You're too far from the shop." call _fnc_reply;
};

(_rows select _index) params ["", "_name", "_price", "", ["_licence", ""], ["_level", 0]];
private _licences = (_record getOrDefault [_faction + "_licence", []]) select { _x isEqualType "" };
private _rank = _record getOrDefault [_faction + "_level", 0];
private _missing = [_shop, _licences, _rank] call ALife_fnc_shopAccess;
if (_missing == "") then { _missing = [[_licence, _level], _licences, _rank] call ALife_fnc_shopAccess; };
if (_missing != "") exitWith { format ["%1 needed.", _missing] call _fnc_reply; };

private _textures = [];
if (_colour != "") then {
    private _source = configFile >> "CfgVehicles" >> _class >> "TextureSources" >> _colour;
    if (isClass _source) then { _textures = getArray (_source >> "textures"); };
};
if (_name == "") then { _name = getText (configFile >> "CfgVehicles" >> _class >> "displayName"); };

// The shop's spawn point nearest the player, and it must be clear.
private _spawns = getArray (_place >> "vehicleSpawns") apply { [_unit distance2D getMarkerPos _x, _x] };
_spawns sort true;
if (_spawns isEqualTo []) exitWith { "This shop has nowhere to put vehicles." call _fnc_reply; };
private _marker = (_spawns select 0) select 1;
private _pos = getMarkerPos _marker;
if ((nearestObjects [_pos, ["LandVehicle", "Air", "Ship"], 7]) isNotEqualTo []) exitWith {
    "The spawn point is blocked. Move the vehicle that's on it and try again." call _fnc_reply;
};

private _paid = [_uid, _faction, _record, _price, _method, _name] call ALife_fnc_shopCharge;
if (_paid != "OK") exitWith {
    diag_log format ["[ALife] %1 vehicle %2: %3 by %4 refused: %5", _uid, _placement, _class, _method, _paid];
    (if (_paid in ["INSUFFICIENT", "ERROR"]) then {
        format ["Not enough %1 for the %2 ($%3).", ["cash", "money in the bank"] select (_method == "bank"), _name, [_price] call BIS_fnc_numberText]
    } else {
        "The purchase didn't go through. Try again."
    }) call _fnc_reply;
};

private _vehicle = createVehicle [_class, _pos, [], 0, "NONE"];
_vehicle setDir markerDir _marker;
_vehicle setPosATL [_pos select 0, _pos select 1, 0.1];
{ _vehicle setObjectTextureGlobal [_forEachIndex, _x]; } forEach _textures;
clearItemCargoGlobal _vehicle;
clearWeaponCargoGlobal _vehicle;
clearMagazineCargoGlobal _vehicle;
clearBackpackCargoGlobal _vehicle;
_vehicle lock 2;
_vehicle setVariable ["alife_owner", _uid, true];

private _added = ["vehicle", ["add", _uid, _faction, _class, str _price]] call ALife_fnc_extension;
private _plate = "";
if (_added select [0, 3] == "OK:") then {
    private _parts = _added splitString ":";
    _vehicle setVariable ["alife_vehicleId", _parts select 1];
    _plate = _parts select 2;
    _vehicle setVariable ["alife_plate", _plate, true];
} else {
    // Paid and spawned; only the record is missing. Logged for staff.
    diag_log format ["[ALife] %1 bought %2 but recording it failed: %3", _uid, _class, _added];
};

diag_log format ["[ALife] %1 bought %2 (%3, plate %4) for $%5 (%6) at %7", _uid, _class, _colour, _plate, _price, _method, _placement];
["bought", [_name, _plate]] remoteExecCall ["ALife_fnc_vehicleShop", _owner];
[format ["Bought the %1 for $%2. It's at the spawn point, locked: press U near it to unlock.", _name, [_price] call BIS_fnc_numberText], "success"] call _fnc_reply;
[_unit] call ALife_fnc_requestPhoneData;
