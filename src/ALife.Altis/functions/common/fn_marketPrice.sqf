/*
    ALife_fnc_marketPrice (common)

    What a virtual item costs right now on the dynamic market
    (ALife_fnc_market): the server's live price, or on a client the last
    broadcast (ALife_marketPrices), or the CfgMarket base price before the
    first broadcast arrives.

    Arguments:
        0: STRING - item class (CfgMarket)
        1: BOOL   - true for what a shop pays when buying it from a player
                    (the price times sellRatio). Default false.

    Returns:
        NUMBER - dollars, rounded to whole dollars; -1 if not on the market
*/

params [["_class", "", [""]], ["_selling", false, [false]]];

private _cfg = missionConfigFile >> "CfgMarket";
if (!isClass (_cfg >> _class)) exitWith { -1 };

private _price = if (isServer && { !isNil "ALife_market" }) then {
    ALife_market getOrDefault [_class, getNumber (_cfg >> _class >> "price")]
} else {
    private _list = missionNamespace getVariable ["ALife_marketPrices", []];
    private _i = _list findIf { (_x select 0) == _class };
    [getNumber (_cfg >> _class >> "price"), (_list select _i) select 1] select (_i >= 0)
};

if (_selling) then {
    private _own = _cfg >> _class >> "sellRatio";
    _price = _price * ([getNumber (_cfg >> "sellRatio"), getNumber _own] select (isNumber _own));
};
round _price
