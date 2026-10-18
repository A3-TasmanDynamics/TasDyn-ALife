/*
    ALife_fnc_shopBuyClothing (server, called by the client)

    Buys the basket from a clothing shop (ALife_fnc_clothingShop): up to
    one item per slot, paid in one charge. Checked here: the player's
    faction, that they're at the shop, that the shop sells every item
    (ClothingShops rows), the licence / rank the shop and each row need, and
    that they can pay the total. On success the client keeps the outfit on
    (its "bought" mode); it's saved with the player's gear
    (ALife_fnc_savePlayer).

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - shop placement (CfgShops) of a clothing shop
        2: ARRAY  - the basket: [[list, classname], ...], list one of
                    "uniforms", "headgear", "goggles", "vests", "backpacks"
        3: STRING - "cash" or "bank"
*/

params [["_unit", objNull, [objNull]], ["_placement", "", [""]], ["_items", [], [[]]], ["_method", "", [""]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };

private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
private _info = [_placement] call ALife_fnc_shopInfo;
private _allLists = ["uniforms", "headgear", "goggles", "vests", "backpacks"];

if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" }) exitWith {};
if (_info isEqualTo [] || { (_info select 0) != "clothing" } || { !(_method in ["cash", "bank"]) }) exitWith {};
// One item per slot, at most one of each slot.
if (_items isEqualTo [] || { count _items > count _allLists }) exitWith {};
if ((_items findIf { !(_x isEqualType []) || { !(_x isEqualTypeParams ["", ""]) } || { !((_x select 0) in _allLists) } }) >= 0) exitWith {};
if (count ((_items apply { _x select 0 }) arrayIntersect (_items apply { _x select 0 })) != count _items) exitWith {};
_info params ["", "_place", "_shop"];

if (getText (_shop >> "side") != _faction) exitWith { "You can't buy that here." call _fnc_reply; };
private _radius = getNumber (_place >> "radius");
if ((getArray (_place >> "markers") findIf { _unit distance2D getMarkerPos _x <= _radius }) < 0) exitWith {
    "You're too far from the shop." call _fnc_reply;
};

private _licences = (_record getOrDefault [_faction + "_licence", []]) select { _x isEqualType "" };
private _rank = _record getOrDefault [_faction + "_level", 0];
private _missing = [_shop, _licences, _rank] call ALife_fnc_shopAccess;
if (_missing != "") exitWith { format ["%1 needed.", _missing] call _fnc_reply; };

private _total = 0;
private _names = [];
private _problem = "";
{
    _x params ["_list", "_class"];
    private _rows = getArray (_shop >> _list);
    private _index = _rows findIf { (_x select 0) == _class };
    if (_index < 0) exitWith { _problem = "You can't buy that here."; };
    (_rows select _index) params ["", "_name", "_price", ["_licence", ""], ["_level", 0]];
    private _need = [[_licence, _level], _licences, _rank] call ALife_fnc_shopAccess;
    if (_need != "") exitWith { _problem = format ["%1 needed.", _need]; };
    if (_name == "") then {
        // Backpacks are vehicles, glasses are CfgGlasses, the rest are CfgWeapons.
        private _root = ["CfgWeapons", "CfgVehicles", "CfgGlasses"] select ((["backpacks", "goggles"] find _list) + 1);
        _name = getText (configFile >> _root >> _class >> "displayName");
    };
    _total = _total + _price;
    _names pushBack _name;
} forEach _items;
if (_problem != "") exitWith { _problem call _fnc_reply; };

private _memo = if (count _names == 1) then { _names select 0 } else { format ["Outfit (%1 items)", count _names] };
private _paid = [_uid, _faction, _record, _total, _method, _memo] call ALife_fnc_shopCharge;
if (_paid != "OK") exitWith {
    diag_log format ["[ALife] %1 clothing %2: %3 by %4 refused: %5", _uid, _placement, _items, _method, _paid];
    (if (_paid in ["INSUFFICIENT", "ERROR"]) then {
        format ["Not enough %1 for your basket ($%2).", ["cash", "money in the bank"] select (_method == "bank"), [_total] call BIS_fnc_numberText]
    } else {
        "The purchase didn't go through. Try again."
    }) call _fnc_reply;
};

diag_log format ["[ALife] %1 bought %2 for $%3 (%4) at %5", _uid, _items, _total, _method, _placement];
["bought", _items] remoteExecCall ["ALife_fnc_clothingShop", _owner];
[format ["Bought %1 for $%2.", _names joinString ", ", [_total] call BIS_fnc_numberText], "success"] call _fnc_reply;
[_unit] call ALife_fnc_requestPhoneData;
