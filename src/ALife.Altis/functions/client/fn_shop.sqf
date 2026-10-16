/*
    ALife_fnc_shop (client)

    A shop window (ui\shop.hpp) for one CfgShops entry. Shows each item's
    price and how many you have; Pay cash / Pay bank ask the server to buy
    (ALife_fnc_shopBuy), which checks everything again and answers with a
    notice and fresh phone data (refreshing this window).

    Modes:
        ["open", shopClass]
        ["onLoad", display]
        ["refresh"]
        ["select"]          - show the selected item's description
        ["buy", "cash"|"bank"]
*/

#define IDD_SHOP     5100
#define IDC_TITLE    5101
#define IDC_BALANCE  5102
#define IDC_DESC     5103
#define IDC_LIST     5110
#define IDC_QTY      5120

params ["_mode", ["_arg", ""]];

private _display = findDisplay IDD_SHOP;
private _shop = missionNamespace getVariable ["ALife_shopClass", ""];
private _cfg = missionConfigFile >> "CfgShops" >> _shop;
private _data = missionNamespace getVariable ["ALife_phone", createHashMap];
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };

switch (_mode) do {
    case "open": {
        if (!isNull _display || { !isClass (missionConfigFile >> "CfgShops" >> _arg) }) exitWith {};
        ALife_shopClass = _arg;
        createDialog "ALife_Shop";
        [player] remoteExec ["ALife_fnc_requestPhoneData", 2];
    };

    case "onLoad": {
        _display = _arg;
        (_display displayCtrl IDC_TITLE) ctrlSetText toUpper getText (_cfg >> "displayName");
        private _list = _display displayCtrl IDC_LIST;
        lnbClear _list;
        {
            private _class = configName _x;
            private _row = _list lnbAddRow [getText (missionConfigFile >> "CfgItems" >> _class >> "displayName"), getNumber (_x >> "price") call _fnc_money, ""];
            _list lnbSetData [[_row, 0], _class];
            _list lnbSetPicture [[_row, 0], getText (missionConfigFile >> "CfgItems" >> _class >> "icon")];
        } forEach ("true" configClasses (_cfg >> "Items"));
        if ((lnbSize _list) select 0 > 0) then { _list lnbSetCurSelRow 0; };
        [] spawn { ["refresh"] call ALife_fnc_shop; }; // after the dialog has finished opening
    };

    case "refresh": {
        if (isNull _display) exitWith {};
        (_display displayCtrl IDC_BALANCE) ctrlSetStructuredText parseText format [
            "<t align='right'><t color='#c0c7d1'>Bank </t><t font='RobotoCondensedBold'>%1</t><t color='#c0c7d1'>   Cash </t><t font='RobotoCondensedBold'>%2</t></t>",
            (_data getOrDefault ["bank", 0]) call _fnc_money, (_data getOrDefault ["cash", 0]) call _fnc_money];

        private _owned = createHashMapFromArray ((_data getOrDefault ["items", []]) select { _x isEqualTypeParams ["", 0] });
        private _list = _display displayCtrl IDC_LIST;
        for "_row" from 0 to ((lnbSize _list) select 0) - 1 do {
            _list lnbSetText [[_row, 2], format ["x%1", _owned getOrDefault [_list lnbData [_row, 0], 0]]];
        };
        ["select"] call ALife_fnc_shop;
    };

    case "select": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        if (_row < 0) exitWith {};
        (_display displayCtrl IDC_DESC) ctrlSetStructuredText parseText format [
            "<t size='0.95' color='#c0c7d1'>%1</t>", getText (missionConfigFile >> "CfgItems" >> (_list lnbData [_row, 0]) >> "description")];
    };

    case "buy": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        private _qty = floor parseNumber ctrlText (_display displayCtrl IDC_QTY);
        if (_row < 0) exitWith { hint "Pick an item first."; };
        if (_qty < 1 || { _qty > 20 }) exitWith { hint "Buy between 1 and 20 at a time."; };
        [player, _shop, _list lnbData [_row, 0], _qty, _arg] remoteExec ["ALife_fnc_shopBuy", 2];
    };
};
