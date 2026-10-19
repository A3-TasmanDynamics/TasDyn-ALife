/*
    ALife_fnc_garageMenu (client)

    The garage window (ui\garage.hpp) at a vehicle shop: your vehicles for
    the faction you're playing, with their plate and where they are (in
    the garage, out, impounded + the release fee). Take one out (it appears
    at the shop's spawn point) or store one of yours parked nearby. The
    server does it all (ALife_fnc_garage) and sends the list back.

    Modes:
        ["open", placement]
        ["onLoad", display]
        ["list", rows]          - from the server: [[id, class, plate, status, fee], ...]
        ["select"]
        ["retrieve"] / ["store"]
*/

#define IDD 5900
#define IDC_TITLE 5901
#define IDC_BALANCE 5902
#define IDC_DETAILS 5903
#define IDC_LIST 5910

params ["_mode", ["_arg", ""]];

private _display = findDisplay IDD;
private _place = missionNamespace getVariable ["ALife_garagePlace", ""];
private _data = missionNamespace getVariable ["ALife_phone", createHashMap];
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };
private _name = { private _n = getText (configFile >> "CfgVehicles" >> _this >> "displayName"); [_this, _n] select (_n != "") };
private _statusText = {
    params ["_status", "_fee"];
    switch (_status) do {
        case "garaged": { "In the garage" };
        case "spawned": { "Out" };
        case "impounded": { format ["Impounded (%1)", _fee call _fnc_money] };
        default { _status };
    };
};

switch (_mode) do {
    case "open": {
        if (dialog) then { closeDialog 0; };
        ALife_garagePlace = _arg;
        ALife_garageRows = [];
        createDialog "ALife_Garage";
    };

    case "onLoad": {
        _display = _arg;
        (_display displayCtrl IDC_TITLE) ctrlSetText "GARAGE";
        (_display displayCtrl IDC_BALANCE) ctrlSetStructuredText parseText format [
            "<t align='right'><t color='#8b98b3'>Bank </t><t font='RobotoCondensedBold'>%1</t></t>", (_data getOrDefault ["bank", 0]) call _fnc_money];
        (_display displayCtrl IDC_DETAILS) ctrlSetStructuredText parseText "<t color='#8b98b3'>Loading your vehicles...</t>";
        [player, "list", _place] remoteExecCall ["ALife_fnc_garage", 2];
    };

    case "list": {
        ALife_garageRows = _arg;
        if (isNull _display) exitWith {};
        (_display displayCtrl IDC_BALANCE) ctrlSetStructuredText parseText format [
            "<t align='right'><t color='#8b98b3'>Bank </t><t font='RobotoCondensedBold'>%1</t></t>", (_data getOrDefault ["bank", 0]) call _fnc_money];
        private _list = _display displayCtrl IDC_LIST;
        private _keep = _list lnbData [(lnbCurSelRow _list) max 0, 0];
        lnbClear _list;
        {
            _x params ["_id", "_class", "_plate", "_status", "_fee"];
            private _row = _list lnbAddRow [_class call _name, _plate, [_status, _fee] call _statusText];
            _list lnbSetData [[_row, 0], _id];
            _list lnbSetPicture [[_row, 0], getText (configFile >> "CfgVehicles" >> _class >> "picture")];
            _list lnbSetColor [[_row, 2], switch (_status) do {
                case "impounded": { [0.97, 0.44, 0.41, 1] };
                case "spawned": { [0.545, 0.596, 0.702, 1] };
                default { [0.29, 0.81, 0.59, 1] };
            }];
            if (_id == _keep) then { _list lnbSetCurSelRow _row; };
        } forEach _arg;
        if (_arg isEqualTo []) exitWith {
            (_display displayCtrl IDC_DETAILS) ctrlSetStructuredText parseText "<t color='#8b98b3'>You don't have any vehicles for this faction yet. Buy one at the shop here.</t>";
        };
        if (lnbCurSelRow _list < 0) then { _list lnbSetCurSelRow 0; };
        ["select"] call ALife_fnc_garageMenu;
    };

    case "select": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        if (_row < 0) exitWith {};
        private _id = _list lnbData [_row, 0];
        private _i = ALife_garageRows findIf { (_x select 0) == _id };
        if (_i < 0) exitWith {};
        (ALife_garageRows select _i) params ["", "_class", "_plate", "_status", "_fee"];
        private _trunk = getNumber (missionConfigFile >> "CfgVehicles" >> _class >> "trunk");
        (_display displayCtrl IDC_DETAILS) ctrlSetStructuredText parseText format [
            "<t font='RobotoCondensedBold'>%1</t>  <t color='#8b98b3'>%2%3</t><br/><t color='#8b98b3'>%4</t>",
            _class call _name, _plate, if (_trunk > 0) then { format ["   Trunk %1", _trunk] } else { "" },
            switch (_status) do {
                case "garaged": { "Take it out and it appears at the spawn point, locked, with what's in its trunk." };
                case "spawned": { "It's out somewhere on the map. Store it at any garage to bring it back here." };
                case "impounded": { format ["The police impounded it. Pay %1 from your bank to take it out.", _fee call _fnc_money] };
                default { "" };
            }];
    };

    case "retrieve": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        if (_row < 0 || { ALife_garageRows isEqualTo [] }) exitWith { ["Pick a vehicle first.", "warning"] call ALife_fnc_toast; };
        [player, "retrieve", _place, _list lnbData [_row, 0]] remoteExecCall ["ALife_fnc_garage", 2];
    };

    case "store": {
        [player, "store", _place] remoteExecCall ["ALife_fnc_garage", 2];
    };
};
