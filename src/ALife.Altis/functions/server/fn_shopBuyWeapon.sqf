/*
    ALife_fnc_shopBuyWeapon (server, called by the client)

    Buys a weapon shop cart (WeaponShops, CfgWeapons.hpp; the window is
    ALife_fnc_weaponShop) in one payment. Checked here: the player's
    faction, that they're at the shop, that the shop sells every line in
    that list, the licence / rank the shop and each row need, that it all
    fits on them, and that they can pay. On success the client puts it on
    (its "bought" mode); it's saved with the player's gear
    (ALife_fnc_savePlayer).

    Fitting: weapons go into their empty slots (rifle, pistol, launcher,
    binoculars) and a map / compass / watch / radio / GPS / night vision
    into its empty slot; everything else must fit by weight in the uniform,
    vest and backpack.

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - shop placement (CfgShops) of a weapon shop
        2: ARRAY  - the cart: [[list, class, qty], ...]; list "items"
                    (weapons and kit), "mags" or "accs"; qty 1-20; 15 lines
        3: STRING - "cash" or "bank"
*/

params [["_unit", objNull, [objNull]], ["_placement", "", [""]], ["_cart", [], [[]]], ["_method", "", [""]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith { ["spoof", _unit, "shopBuyWeapon", remoteExecutedOwner] call ALife_fnc_anticheat; };
if (isRemoteExecuted && { ["rate", _unit, "buy"] call ALife_fnc_anticheat }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };

private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
private _info = [_placement] call ALife_fnc_shopInfo;

if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" }) exitWith {};
if (_info isEqualTo [] || { (_info select 0) != "weapons" } || { !(_method in ["cash", "bank"]) }) exitWith {};
if (_cart isEqualTo [] || { count _cart > 15 }) exitWith {};
if ((_cart findIf { !(_x isEqualTypeParams ["", "", 0]) || { !((_x select 0) in ["items", "mags", "accs"]) } || { (_x select 2) < 1 } || { (_x select 2) > 20 } }) >= 0) exitWith {};
_info params ["", "_place", "_shop"];

if (getText (_shop >> "side") != _faction) exitWith { "You can't buy that here." call _fnc_reply; };
if (!([_unit, _place] call ALife_fnc_atPlace)) exitWith { "You're too far from the shop." call _fnc_reply; };

private _licences = (_record getOrDefault [_faction + "_licence", []]) select { _x isEqualType "" };
private _rank = _record getOrDefault [_faction + "_level", 0];
private _missing = [_shop, _licences, _rank] call ALife_fnc_shopAccess;
if (_missing != "") exitWith { format ["%1 needed.", _missing] call _fnc_reply; };

// Each line: in this shop, allowed, priced; and what it weighs that needs room.
private _total = 0;
private _mass = 0;
private _lines = [];
private _problem = "";
private _slotsUsed = [];
{
    _x params ["_list", "_class", "_qty"];
    _qty = floor _qty;
    private _rows = getArray (_shop >> _list);
    private _index = _rows findIf { (_x param [0, ""]) == _class };
    if (_index < 0) exitWith { _problem = "You can't buy that here."; };
    (_rows select _index) params ["", ["_name", ""], ["_price", 0], ["", 0], ["_licence", ""], ["_level", 0]];
    private _cfg = configFile >> (["CfgWeapons", "CfgMagazines"] select (_list == "mags")) >> _class;
    if (!isClass _cfg) exitWith { _problem = "You can't buy that here."; };
    if (_name == "") then { _name = getText (_cfg >> "displayName"); };
    private _need = [[_licence, _level], _licences, _rank] call ALife_fnc_shopAccess;
    if (_need != "") exitWith { _problem = format ["%1 needed for %2.", _need, _name]; };

    private _type = if (_list == "items") then { getNumber (_cfg >> "type") } else { 0 };
    private _each = switch (true) do {
        case (_list == "mags"): { getNumber (_cfg >> "mass") };
        case (_type in [1, 2, 4, 4096]): { getNumber (_cfg >> "WeaponSlotsInfo" >> "mass") };
        default { getNumber (_cfg >> "ItemInfo" >> "mass") };
    };
    // One of it into an empty slot weighs nothing here.
    private _free = 0;
    if (_type in [1, 2, 4, 4096]) then {
        private _slot = switch (_type) do {
            case 1: { primaryWeapon _unit };
            case 2: { handgunWeapon _unit };
            case 4: { secondaryWeapon _unit };
            default { binocular _unit };
        };
        if (_slot == "" && { !(_type in _slotsUsed) }) then { _free = 1; _slotsUsed pushBack _type; };
    } else {
        private _sim = getText (_cfg >> "simulation");
        if (_list == "items" && { _sim in ["ItemMap", "ItemCompass", "ItemWatch", "ItemRadio", "ItemGPS", "NVGoggles"] } && { !(_sim in _slotsUsed) }
            && { (assignedItems _unit) findIf { getText (configFile >> "CfgWeapons" >> _x >> "simulation") == _sim } < 0 }) then {
            _free = 1;
            _slotsUsed pushBack _sim;
        };
    };
    _mass = _mass + _each * (_qty - _free);
    _total = _total + _price * _qty;
    _lines pushBack [_list, _class, _qty, _name];
} forEach _cart;
if (_problem != "") exitWith { _problem call _fnc_reply; };

// Room left in the uniform, vest and backpack.
private _room = 0;
{
    _x params ["_item", "_container"];
    if (_item != "" && { !isNull _container }) then {
        _room = _room + (getContainerMaxLoad _item) * (1 - load _container);
    };
} forEach [[uniform _unit, uniformContainer _unit], [vest _unit, vestContainer _unit], [backpack _unit, backpackContainer _unit]];
if (_mass > _room + 0.5) exitWith { "That won't all fit on you. Make space in your uniform, vest or backpack, or buy less." call _fnc_reply; };

private _memo = if (count _lines == 1) then {
    (_lines select 0) params ["", "", "_qty", "_name"];
    [format ["%1 x%2", _name, _qty], _name] select (_qty == 1)
} else {
    format ["%1 (%2 items)", getText (_shop >> "name"), count _lines]
};
private _paid = [_uid, _faction, _record, _total, _method, _memo] call ALife_fnc_shopCharge;
if (_paid != "OK") exitWith {
    diag_log format ["[ALife] %1 weapon shop %2: %3 by %4 refused: %5", _uid, _placement, _cart, _method, _paid];
    (if (_paid in ["INSUFFICIENT", "ERROR"]) then {
        format ["Not enough %1 for your cart ($%2).", ["cash", "money in the bank"] select (_method == "bank"), [_total] call BIS_fnc_numberText]
    } else {
        "The purchase didn't go through. Try again."
    }) call _fnc_reply;
};

diag_log format ["[ALife] %1 bought %2 for $%3 (%4) at %5", _uid, _lines apply { [_x select 1, _x select 2] }, _total, _method, _placement];
["log", _unit, "shop.buy_gear", [["shop", _placement], ["items", _lines apply { [_x select 1, _x select 2] }], ["total", _total], ["paid_by", _method]]] call ALife_fnc_anticheat;
["bought", _lines apply { [_x select 0, _x select 1, _x select 2] }] remoteExecCall ["ALife_fnc_weaponShop", _owner];
[format ["Bought %1 for $%2.", _memo, [_total] call BIS_fnc_numberText], "success"] call _fnc_reply;
[_unit] call ALife_fnc_requestPhoneData;
