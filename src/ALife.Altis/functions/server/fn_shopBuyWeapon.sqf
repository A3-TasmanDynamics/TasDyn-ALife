/*
    ALife_fnc_shopBuyWeapon (server, called by the client)

    Buys from a weapon shop (WeaponShops, CfgWeapons.hpp; the window is
    ALife_fnc_weaponShop). Checked here: the player's faction, that they're
    at the shop, that the shop sells it in that list, the licence / rank the
    shop and the row need, that it fits on them, and that they can pay. On
    success the client puts it on (its "bought" mode); it's saved with the
    player's gear (ALife_fnc_savePlayer).

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - shop placement (CfgShops) of a weapon shop
        2: STRING - list: "items" (weapons and kit), "mags" or "accs"
        3: STRING - classname
        4: NUMBER - quantity (1-20; weapons 1)
        5: STRING - "cash" or "bank"
*/

params [["_unit", objNull, [objNull]], ["_placement", "", [""]], ["_list", "", [""]], ["_class", "", [""]], ["_qty", 0, [0]], ["_method", "", [""]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };

private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
private _info = [_placement] call ALife_fnc_shopInfo;
_qty = floor _qty;

if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" }) exitWith {};
if (_info isEqualTo [] || { (_info select 0) != "weapons" } || { !(_list in ["items", "mags", "accs"]) } || { !(_method in ["cash", "bank"]) }) exitWith {};
if (_qty < 1 || { _qty > 20 }) exitWith {};
_info params ["", "_place", "_shop"];

if (getText (_shop >> "side") != _faction) exitWith { "You can't buy that here." call _fnc_reply; };
if (!([_unit, _place] call ALife_fnc_atPlace)) exitWith { "You're too far from the shop." call _fnc_reply; };

private _rows = getArray (_shop >> _list);
private _index = _rows findIf { (_x param [0, ""]) == _class };
if (_index < 0) exitWith { "You can't buy that here." call _fnc_reply; };
(_rows select _index) params ["", ["_name", ""], ["_price", 0], ["", 0], ["_licence", ""], ["_level", 0]];

private _licences = (_record getOrDefault [_faction + "_licence", []]) select { _x isEqualType "" };
private _rank = _record getOrDefault [_faction + "_level", 0];
private _missing = [_shop, _licences, _rank] call ALife_fnc_shopAccess;
if (_missing == "") then { _missing = [[_licence, _level], _licences, _rank] call ALife_fnc_shopAccess; };
if (_missing != "") exitWith { format ["%1 needed.", _missing] call _fnc_reply; };

private _cfg = configFile >> (["CfgWeapons", "CfgMagazines"] select (_list == "mags")) >> _class;
if (!isClass _cfg) exitWith { "You can't buy that here." call _fnc_reply; };
if (_name == "") then { _name = getText (_cfg >> "displayName"); };

// Room for it: a weapon in its empty slot or a container; kit in its empty
// slot (map, GPS, NVGs...) or the inventory.
private _type = if (_list == "items") then { getNumber (_cfg >> "type") } else { 0 };
private _fits = if (_type in [1, 2, 4, 4096]) then {
    _qty = 1;
    private _slot = switch (_type) do {
        case 1: { primaryWeapon _unit };
        case 2: { handgunWeapon _unit };
        case 4: { secondaryWeapon _unit };
        default { binocular _unit };
    };
    _slot == "" || { _unit canAddItemToBackpack _class } || { _unit canAddItemToVest _class } || { _unit canAddItemToUniform _class }
} else {
    private _sim = getText (_cfg >> "simulation");
    _unit canAdd [_class, _qty] || {
        _qty == 1 && { _sim in ["ItemMap", "ItemCompass", "ItemWatch", "ItemRadio", "ItemGPS", "NVGoggles"] }
            && { (assignedItems _unit) findIf { getText (configFile >> "CfgWeapons" >> _x >> "simulation") == _sim } < 0 }
    }
};
if (!_fits) exitWith { format ["You don't have room for %1. Make space in your uniform, vest or backpack.", _name] call _fnc_reply; };

private _total = _price * _qty;
private _memo = if (_qty == 1) then { _name } else { format ["%1 x%2", _name, _qty] };
private _paid = [_uid, _faction, _record, _total, _method, _memo] call ALife_fnc_shopCharge;
if (_paid != "OK") exitWith {
    diag_log format ["[ALife] %1 weapon shop %2: %3 x%4 by %5 refused: %6", _uid, _placement, _class, _qty, _method, _paid];
    (if (_paid in ["INSUFFICIENT", "ERROR"]) then {
        format ["Not enough %1 for %2 ($%3).", ["cash", "money in the bank"] select (_method == "bank"), _memo, [_total] call BIS_fnc_numberText]
    } else {
        "The purchase didn't go through. Try again."
    }) call _fnc_reply;
};

diag_log format ["[ALife] %1 bought %2 x%3 for $%4 (%5) at %6", _uid, _class, _qty, _total, _method, _placement];
["bought", [_list, _class, _qty]] remoteExecCall ["ALife_fnc_weaponShop", _owner];
[format ["Bought %1 for $%2.", _memo, [_total] call BIS_fnc_numberText], "success"] call _fnc_reply;
[_unit] call ALife_fnc_requestPhoneData;
