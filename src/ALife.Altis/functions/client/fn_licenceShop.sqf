/*
    ALife_fnc_licenceShop (client)

    The licence office window (ui\licenceShop.hpp), opened from a licence
    vendor NPC (CfgShops type "licences"): the licences it sells, their
    price and which you already have, and what each one unlocks (the shops
    and vehicles that need it). Buying goes to the server
    (ALife_fnc_buyLicence), which charges the bank, saves it and tells the
    window.

    Modes:
        ["open", placement]
        ["onLoad", display]
        ["refresh"]
        ["select"]
        ["buy"]
        ["bought", licences]   - from the server: your licences now
*/

#define IDD 6000
#define IDC_TITLE 6001
#define IDC_BALANCE 6002
#define IDC_DETAILS 6003
#define IDC_LIST 6010
#define IDC_BUY 6020

params ["_mode", ["_arg", ""]];

// The display: passed in while it's loading, otherwise found.
private _display = if (_arg isEqualType displayNull) then { _arg } else { findDisplay IDD };
private _place = missionNamespace getVariable ["ALife_licencePlace", ""];
private _office = missionConfigFile >> "CfgShops" >> _place;
private _data = missionNamespace getVariable ["ALife_phone", createHashMap];
private _owned = (_data getOrDefault ["licences", []]) select { _x isEqualType "" };
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };

switch (_mode) do {
    case "open": {
        if (dialog) then { closeDialog 0; };
        ALife_licencePlace = _arg;
        createDialog "ALife_LicenceShop";
    };

    case "onLoad": {
        (_display displayCtrl IDC_TITLE) ctrlSetText toUpper getText (_office >> "name");
        ["refresh", _display] call ALife_fnc_licenceShop;
    };

    case "refresh": {
        if (isNull _display) exitWith {};
        (_display displayCtrl IDC_BALANCE) ctrlSetStructuredText parseText format [
            "<t align='right'><t color='#8b98b3'>Bank </t><t font='RobotoCondensedBold'>%1</t></t>", (_data getOrDefault ["bank", 0]) call _fnc_money];
        private _list = _display displayCtrl IDC_LIST;
        private _keep = _list lnbData [(lnbCurSelRow _list) max 0, 0];
        lnbClear _list;
        {
            private _cfg = missionConfigFile >> "CfgLicences" >> _x;
            if (isClass _cfg && { getNumber (_cfg >> "price") > 0 }) then {
                private _have = _x in _owned;
                private _row = _list lnbAddRow [getText (_cfg >> "displayName"), getNumber (_cfg >> "price") call _fnc_money, ["", "Owned"] select _have];
                _list lnbSetData [[_row, 0], _x];
                if (_have) then {
                    _list lnbSetColor [[_row, 2], [0.29, 0.81, 0.59, 1]];
                    _list lnbSetColor [[_row, 1], [0.545, 0.596, 0.702, 1]];
                };
                if (_x == _keep) then { _list lnbSetCurSelRow _row; };
            };
        } forEach getArray (_office >> "licences");
        if (lnbCurSelRow _list < 0) then { _list lnbSetCurSelRow 0; };
        ["select", _display] call ALife_fnc_licenceShop;
    };

    case "select": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        if (_row < 0) exitWith {};
        private _key = _list lnbData [_row, 0];
        private _cfg = missionConfigFile >> "CfgLicences" >> _key;
        // What needs it: whole shops (their licence) and single vehicles.
        private _shops = [];
        {
            private _root = _x;
            {
                if (getText (_x >> "licence") == _key) then { _shops pushBackUnique getText (_x >> "name"); };
            } forEach ("true" configClasses (missionConfigFile >> _root));
        } forEach ["VirtualShops", "WeaponShops", "ClothingShops", "CarShops"];
        private _vehicles = [];
        {
            {
                _x params ["_class", ["_rowName", ""], "", "", ["_needs", ""]];
                if (_needs == _key) then { _vehicles pushBackUnique ([getText (configFile >> "CfgVehicles" >> _class >> "displayName"), _rowName] select (_rowName != "")); };
            } forEach getArray (_x >> "vehicles");
        } forEach ("true" configClasses (missionConfigFile >> "CarShops"));
        private _uses = [];
        if (_shops isNotEqualTo []) then { _uses pushBack ("Shops: " + (_shops joinString ", ")); };
        if (_vehicles isNotEqualTo []) then { _uses pushBack ("Vehicles: " + (_vehicles joinString ", ")); };
        if (_uses isEqualTo []) then { _uses pushBack "Needed for things still to come."; };
        (_display displayCtrl IDC_DETAILS) ctrlSetStructuredText parseText format [
            "<t font='RobotoCondensedBold'>%1</t>  <t color='#8b98b3'>%2 from your bank%3</t><br/><t color='#8b98b3' size='0.9'>%4</t>",
            getText (_cfg >> "displayName"), getNumber (_cfg >> "price") call _fnc_money,
            ["", "  -  you have it"] select (_key in _owned), _uses joinString "<br/>"];
        (_display displayCtrl IDC_BUY) ctrlEnable !(_key in _owned);
    };

    case "buy": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        if (_row < 0) exitWith { ["Pick a licence first.", "warning"] call ALife_fnc_toast; };
        private _key = _list lnbData [_row, 0];
        if (_key in _owned) exitWith {};
        (_display displayCtrl IDC_BUY) ctrlEnable false;
        [player, _place, _key] remoteExecCall ["ALife_fnc_buyLicence", 2];
        // Not bought (not enough money): the button comes back.
        [] spawn { uiSleep 2; ["select"] call ALife_fnc_licenceShop; };
    };

    case "bought": {
        // The phone data follows from the server too; this keeps the window
        // right straight away.
        _data set ["licences", _arg];
        ["refresh"] call ALife_fnc_licenceShop;
    };
};
