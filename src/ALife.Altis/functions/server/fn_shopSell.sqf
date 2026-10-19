/*
    ALife_fnc_shopSell (server, called by the client)

    Sells virtual items to a trader (a VirtualShops shop's buys[], e.g. the
    Resource Trader). Checked here: the player's faction, that they're at
    the shop, that it buys the item, the shop's licence / rank, and that
    they have that many. Paid in cash at the market's sell price; selling
    pushes the price down (ALife_fnc_market).

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - shop placement (CfgShops) of a market
        2: STRING - item class
        3: NUMBER - quantity (1-999)
*/

params [["_unit", objNull, [objNull]], ["_shop", "", [""]], ["_class", "", [""]], ["_qty", 0, [0]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };

private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
private _info = [_shop] call ALife_fnc_shopInfo;
private _placeCfg = _info param [1, configNull];
private _shopCfg = _info param [2, configNull];

if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" }) exitWith {};
if (_info isEqualTo [] || { (_info select 0) != "market" } || { !isClass (missionConfigFile >> "CfgMarket" >> _class) }
    || { !(_class in getArray (_shopCfg >> "buys")) } || { getText (_shopCfg >> "side") != _faction }) exitWith {
    "They don't buy that here." call _fnc_reply;
};
if (!([_unit, _placeCfg] call ALife_fnc_atPlace)) exitWith {
    "You're too far from the shop." call _fnc_reply;
};
private _licences = (_record getOrDefault [_faction + "_licence", []]) select { _x isEqualType "" };
private _missing = [_shopCfg, _licences, _record getOrDefault [_faction + "_level", 0]] call ALife_fnc_shopAccess;
if (_missing != "") exitWith { format ["%1 needed.", _missing] call _fnc_reply; };
_qty = round _qty;
if (_qty < 1 || { _qty > 999 }) exitWith {};

private _name = getText (missionConfigFile >> "CfgItems" >> _class >> "displayName");
private _total = ([_class, true] call ALife_fnc_marketPrice) * _qty;
if (!([_uid, _faction, _class, -_qty] call ALife_fnc_itemsChange)) exitWith {
    format ["You don't have %1 %2.", _qty, _name] call _fnc_reply;
};

private _token = format ["sell-%1-%2-%3", _uid, round (diag_tickTime * 1000), floor random 1e6];
private _paid = ["save", [_uid, _faction + "_cash", _total toFixed 0, _token]] call ALife_fnc_extension;
if (_paid != "OK") exitWith {
    // Not paid: give the items back.
    [_uid, _faction, _class, _qty] call ALife_fnc_itemsChange;
    diag_log format ["[ALife] %1 selling %2 x%3 at %4: payment failed (%5), items returned", _uid, _class, _qty, _shop, _paid];
    "The sale didn't go through. Try again." call _fnc_reply;
};
_record set [_faction + "_cash", (_record getOrDefault [_faction + "_cash", 0]) + _total];

diag_log format ["[ALife] %1 sold %2 x%3 for $%4 at %5", _uid, _class, _qty, _total, _shop];
["trade", [_class, -_qty]] call ALife_fnc_market;
[format ["Sold %1 x%2 for $%3.", _name, _qty, [_total] call BIS_fnc_numberText], "success"] call _fnc_reply;
[_unit] call ALife_fnc_requestPhoneData;
