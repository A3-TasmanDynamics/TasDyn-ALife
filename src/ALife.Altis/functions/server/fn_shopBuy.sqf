/*
    ALife_fnc_shopBuy (server, called by the client)

    Buys virtual items from a shop. Everything is checked here: the
    player's faction, that they're standing at the shop, that the shop sells
    it (CfgShops items[]), the licence / rank the shop and the item need
    (CfgMarket), the current market price, and that they can pay. Paid by
    cash or bank; the purchase then moves the market (ALife_fnc_market).

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - shop placement (CfgShops) of a market
        2: STRING - item class
        3: NUMBER - quantity (1-20)
        4: STRING - "cash" or "bank"
*/

params [["_unit", objNull, [objNull]], ["_shop", "", [""]], ["_class", "", [""]], ["_qty", 0, [0]], ["_method", "", [""]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };

private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
private _info = [_shop] call ALife_fnc_shopInfo;
private _placeCfg = _info param [1, configNull];
private _shopCfg = _info param [2, configNull]; // the VirtualShops shop
private _itemCfg = missionConfigFile >> "CfgMarket" >> _class;

if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" }) exitWith {};
if (_info isEqualTo [] || { (_info select 0) != "market" } || { !isClass _itemCfg } || { !(_class in getArray (_shopCfg >> "items")) } || { getText (_shopCfg >> "side") != _faction }) exitWith {
    "You can't buy that here." call _fnc_reply;
};
private _radius = getNumber (_placeCfg >> "radius");
if ((getArray (_placeCfg >> "markers") findIf { _unit distance2D getMarkerPos _x <= _radius }) < 0) exitWith {
    "You're too far from the shop." call _fnc_reply;
};
private _licences = (_record getOrDefault [_faction + "_licence", []]) select { _x isEqualType "" };
private _level = _record getOrDefault [_faction + "_level", 0];
private _missing = [_shopCfg, _licences, _level] call ALife_fnc_shopAccess;
if (_missing == "") then { _missing = [_itemCfg, _licences, _level] call ALife_fnc_shopAccess; };
if (_missing != "") exitWith {
    format ["%1 needed.", _missing] call _fnc_reply;
};
_qty = round _qty;
if (_qty < 1 || { _qty > 20 } || { !(_method in ["cash", "bank"]) }) exitWith {};

private _total = ([_class] call ALife_fnc_marketPrice) * _qty;
private _name = getText (missionConfigFile >> "CfgItems" >> _class >> "displayName");
private _paid = [_uid, _faction, _record, _total, _method, format ["%1 x%2", _name, _qty]] call ALife_fnc_shopCharge;

if (_paid != "OK") exitWith {
    diag_log format ["[ALife] %1 shop %2: %3 x%4 by %5 refused: %6", _uid, _shop, _class, _qty, _method, _paid];
    (if (_paid in ["INSUFFICIENT", "ERROR"]) then {
        format ["Not enough %1 for %2 x%3 ($%4).", ["cash", "money in the bank"] select (_method == "bank"), _name, _qty, [_total] call BIS_fnc_numberText]
    } else {
        "The purchase didn't go through. Try again."
    }) call _fnc_reply;
};

if (!([_uid, _faction, _class, _qty] call ALife_fnc_itemsChange)) exitWith {
    diag_log format ["[ALife] %1 paid %2 for %3 x%4 but the items didn't save -- needs a refund", _uid, _total, _class, _qty];
    "You were charged but the items didn't save. Tell staff (it's in the server log)." call _fnc_reply;
};

diag_log format ["[ALife] %1 bought %2 x%3 for $%4 (%5) at %6", _uid, _class, _qty, _total, _method, _shop];
["trade", [_class, _qty]] call ALife_fnc_market;
[format ["Bought %1 x%2 for $%3.", _name, _qty, [_total] call BIS_fnc_numberText], "success"] call _fnc_reply;
[_unit] call ALife_fnc_requestPhoneData;
