/*
    ALife_fnc_market (server)

    The dynamic market for virtual items (CfgMarket.hpp). The server keeps
    the current buy price of every item in ALife_market (class -> price);
    the sell price moves with it (ALife_fnc_marketPrice):

      - "init": base prices from CfgMarket, then the saved prices from the
        database (market_prices) on top, so prices carry over restarts. Then
        starts the recovery loop. Run once, when the database is ready.
      - "trade" [class, qty]: qty units bought (positive) or sold (negative).
        Each unit bought multiplies the price by (1 + step), each sold by
        (1 - step), clamped to min..max. Saved and broadcast.
      - The loop: every recoveryInterval seconds each price moves `recovery`
        of the way back toward its base (new stock arriving), saved and
        broadcast when it changed.

    Clients read prices from ALife_marketPrices ([[class, price], ...],
    public) through ALife_fnc_marketPrice. A wipe or economy reset is
    emptying market_prices (["market", ["reset"]]) and restarting.

    Arguments:
        0: STRING - "init" or "trade"
        1: ARRAY  - for "trade": [class, qty]
*/

params [["_mode", "", [""]], ["_args", []]];

if (!isServer) exitWith {};

private _cfg = missionConfigFile >> "CfgMarket";
private _setting = {
    params ["_class", "_key"];
    private _own = _cfg >> _class >> _key;
    [getNumber (_cfg >> _key), getNumber _own] select (isNumber _own)
};
private _publish = {
    ALife_marketPrices = (keys ALife_market) apply { [_x, ALife_market get _x] };
    publicVariable "ALife_marketPrices";
};
private _save = {
    params ["_class", "_price"];
    private _answer = ["market", ["set", _class, _price toFixed 2]] call ALife_fnc_extension;
    if (_answer != "OK") then { diag_log format ["[ALife] market: saving %1 = %2 failed: %3", _class, _price, _answer]; };
};

switch (_mode) do {
    case "init": {
        ALife_market = createHashMap;
        {
            ALife_market set [configName _x, getNumber (_x >> "buyPrice")];
        } forEach ("true" configClasses _cfg);

        private _saved = ["market", ["load"]] call ALife_fnc_extension;
        if (_saved select [0, 6] == "ERROR:") then {
            diag_log format ["[ALife] market: couldn't load saved prices, using base prices: %1", _saved];
        } else {
            private _loaded = 0;
            {
                _x params ["_class", "_price"];
                // Only items still in CfgMarket; a removed item's row is ignored.
                if (_class in ALife_market) then {
                    private _item = _cfg >> _class;
                    ALife_market set [_class, (_price max getNumber (_item >> "min")) min getNumber (_item >> "max")];
                    _loaded = _loaded + 1;
                };
            } forEach (parseSimpleArray _saved);
            diag_log format ["[ALife] market: %1 items, %2 saved prices loaded (the rest at base)", count ALife_market, _loaded];
        };
        call _publish;

        // Recovery: prices drift back toward base between trades.
        [_cfg, _setting, _publish, _save] spawn {
            params ["_cfg", "_setting", "_publish", "_save"];
            while { true } do {
                sleep (getNumber (_cfg >> "recoveryInterval") max 30);
                private _changed = false;
                {
                    private _current = ALife_market get _x;
                    private _base = getNumber (_cfg >> _x >> "buyPrice");
                    private _gap = _base - _current;
                    if (abs _gap >= 0.01) then {
                        private _new = _current + _gap * ([_x, "recovery"] call _setting);
                        // Snap the last cent so a price actually arrives back at base.
                        if (abs (_base - _new) < 0.01) then { _new = _base; };
                        _new = (round (_new * 100)) / 100;
                        ALife_market set [_x, _new];
                        [_x, _new] call _save;
                        _changed = true;
                    };
                } forEach keys ALife_market; // a copy of the keys: prices change as we go
                if (_changed) then { call _publish; };
            };
        };
    };

    case "trade": {
        _args params [["_class", "", [""]], ["_qty", 0, [0]]];
        if (isNil "ALife_market" || { !(_class in ALife_market) } || { _qty == 0 }) exitWith {};
        private _item = _cfg >> _class;
        private _step = [_class, "step"] call _setting;
        private _dir = [-1, 1] select (_qty > 0); // bought pushes up, sold pushes down
        private _price = (ALife_market get _class) * ((1 + _step * _dir) ^ abs _qty);
        _price = ((_price max getNumber (_item >> "min")) min getNumber (_item >> "max"));
        _price = (round (_price * 100)) / 100;
        ALife_market set [_class, _price];
        [_class, _price] call _save;
        call _publish;
    };
};
