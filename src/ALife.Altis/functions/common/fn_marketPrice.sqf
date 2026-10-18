/*
    ALife_fnc_marketPrice (common)

    What a virtual item costs right now on the dynamic market
    (ALife_fnc_market): the server's live price, or on a client the last
    broadcast (ALife_marketPrices), or the CfgMarket base price before the
    first broadcast arrives.

    Arguments:
        0: STRING - item class (CfgMarket)
        1: BOOL   - true for what a shop pays when buying it from a player:
                    the current price scaled by the item's sellPrice / buyPrice.
                    Default false.

    Returns:
        NUMBER - dollars, rounded to whole dollars; -1 if not on the market
*/

params [["_class", "", [""]], ["_selling", false, [false]]];

private _cfg = missionConfigFile >> "CfgMarket";
if (!isClass (_cfg >> _class)) exitWith { -1 };

private _price = if (isServer && { !isNil "ALife_market" }) then {
    ALife_market getOrDefault [_class, getNumber (_cfg >> _class >> "buyPrice")]
} else {
    private _list = missionNamespace getVariable ["ALife_marketPrices", []];
    private _i = _list findIf { (_x select 0) == _class };
    // (Not `[a, b] select cond`: SQF builds both, and _list select -1 fails.)
    if (_i >= 0) then { (_list select _i) select 1 } else { getNumber (_cfg >> _class >> "buyPrice") }
};

if (_selling) then {
    _price = _price * getNumber (_cfg >> _class >> "sellPrice") / (getNumber (_cfg >> _class >> "buyPrice") max 1);
};
round _price
