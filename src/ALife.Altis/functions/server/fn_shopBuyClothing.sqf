/*
    ALife_fnc_shopBuyClothing (server, called by the client)

    Buys a piece of clothing the player is trying on in a clothing shop
    (ALife_fnc_clothingShop). Checked here: the player's faction, that
    they're at the shop, that the shop sells it (ClothingShops row), the
    licence / rank the shop and the row need, and that they can pay. On
    success the client keeps it on (its "bought" mode); the outfit is saved
    with the player's gear (ALife_fnc_savePlayer).

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - shop placement (CfgShops) of a clothing shop
        2: STRING - list: "uniforms", "headgear", "goggles", "vests", "backpacks"
        3: STRING - classname
        4: STRING - "cash" or "bank"
*/

params [["_unit", objNull, [objNull]], ["_placement", "", [""]], ["_list", "", [""]], ["_class", "", [""]], ["_method", "", [""]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };

private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
private _info = [_placement] call ALife_fnc_shopInfo;

if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" }) exitWith {};
if (_info isEqualTo [] || { (_info select 0) != "clothing" } || { !(_list in ["uniforms", "headgear", "goggles", "vests", "backpacks"]) } || { !(_method in ["cash", "bank"]) }) exitWith {};
_info params ["", "_place", "_shop"];

private _rows = getArray (_shop >> _list);
private _index = _rows findIf { (_x select 0) == _class };
if (_index < 0 || { getText (_shop >> "side") != _faction }) exitWith { "You can't buy that here." call _fnc_reply; };
private _row = _rows select _index;
private _radius = getNumber (_place >> "radius");
if ((getArray (_place >> "markers") findIf { _unit distance2D getMarkerPos _x <= _radius }) < 0) exitWith {
    "You're too far from the shop." call _fnc_reply;
};

_row params ["", "_name", "_price", ["_licence", ""], ["_level", 0]];
private _licences = (_record getOrDefault [_faction + "_licence", []]) select { _x isEqualType "" };
private _rank = _record getOrDefault [_faction + "_level", 0];
private _missing = [_shop, _licences, _rank] call ALife_fnc_shopAccess;
if (_missing == "") then { _missing = [[_licence, _level], _licences, _rank] call ALife_fnc_shopAccess; };
if (_missing != "") exitWith { format ["%1 needed.", _missing] call _fnc_reply; };

if (_name == "") then {
    // Backpacks are vehicles, glasses are CfgGlasses, the rest are CfgWeapons.
    private _root = ["CfgWeapons", "CfgVehicles", "CfgGlasses"] select ((["backpacks", "goggles"] find _list) + 1);
    _name = getText (configFile >> _root >> _class >> "displayName");
};
private _paid = [_uid, _faction, _record, _price, _method, _name] call ALife_fnc_shopCharge;
if (_paid != "OK") exitWith {
    diag_log format ["[ALife] %1 clothing %2: %3 by %4 refused: %5", _uid, _placement, _class, _method, _paid];
    (if (_paid in ["INSUFFICIENT", "ERROR"]) then {
        format ["Not enough %1 for %2 ($%3).", ["cash", "money in the bank"] select (_method == "bank"), _name, [_price] call BIS_fnc_numberText]
    } else {
        "The purchase didn't go through. Try again."
    }) call _fnc_reply;
};

diag_log format ["[ALife] %1 bought %2 for $%3 (%4) at %5", _uid, _class, _price, _method, _placement];
["bought", [_list, _class]] remoteExecCall ["ALife_fnc_clothingShop", _owner];
[format ["Bought %1 for $%2.", _name, [_price] call BIS_fnc_numberText], "success"] call _fnc_reply;
[_unit] call ALife_fnc_requestPhoneData;
