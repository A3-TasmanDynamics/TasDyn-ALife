/*
    ALife_fnc_phone (client)

    The phone (ui\phone.hpp, opened with Y). A home screen of app tiles; each
    app arranges the same set of generic controls (info text, two lists, an
    amount box, a message box, buttons) for itself. Everything that changes
    something is a request to the server; the server answers with fresh
    phone data (ALife_fnc_phoneData), which refreshes the open app.

    Apps: Inventory, Bank, Levels, Messages, Settings -- and Gang, Keys,
    Admin as "coming soon" for now.

    Modes:
        ["open"]                 - open on the home screen
        ["onLoad", display]
        ["home"]                 - back to the home screen
        ["app", index]           - open an app (tile index)
        ["refresh"]              - redraw the open app with current data
        ["select", idc, row]     - a list row was picked
        ["button", n]            - one of the app's buttons
*/

#define IDD_PHONE    5000
#define IDC_STATUS   5003
#define IDC_HOMEBAR  5004
#define IDC_ICON     5110
#define IDC_LABEL    5120
#define IDC_TILE     5130
#define IDC_HEADER   5201
#define IDC_BACK     5202
#define IDC_TITLE    5203
#define IDC_INFO     5210
#define IDC_LIST     5211
#define IDC_LIST2    5212
#define IDC_EDIT     5213
#define IDC_MESSAGE  5214
#define IDC_BUTTON   5220

params ["_mode"];
private _args = _this select [1];
private _display = uiNamespace getVariable ["ALife_phoneDisplay", displayNull];

private _apps = ["inventory", "bank", "levels", "messages", "gang", "keys", "admin", "settings"];
private _titles = ["Inventory", "Bank", "Levels", "Messages", "Gang", "Keys", "Admin", "Settings"];
private _data = missionNamespace getVariable ["ALife_phone", createHashMap];
private _faction = _data getOrDefault ["faction", missionNamespace getVariable ["ALife_faction", "civ"]];
private _factionName = switch (_faction) do { case "cop": { "Police" }; case "medic": { "EMS" }; default { "Civilian" }; };
private _appControls = [IDC_HEADER, IDC_BACK, IDC_TITLE, IDC_INFO, IDC_LIST, IDC_LIST2, IDC_EDIT, IDC_MESSAGE]
    + ([0, 1, 2, 3, 4, 5] apply { IDC_BUTTON + _x });

private _fnc_safe = { (_this splitString "<>&") joinString "" };
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };

// Place a control inside the app area: top/height (and left/width) are
// fractions of that area.
private _fnc_place = {
    params ["_idc", "_top", "_height", ["_left", 0], ["_width", 1]];
    private _ctrl = _display displayCtrl _idc;
    _ctrl ctrlSetPosition [
        safezoneX + safezoneW * (0.757 + 0.181 * _left),
        safezoneY + safezoneH * (0.375 + 0.555 * _top),
        safezoneW * 0.181 * _width,
        safezoneH * 0.555 * _height
    ];
    _ctrl ctrlCommit 0;
    _ctrl ctrlShow true;
    _ctrl
};
private _fnc_button = {
    params ["_n", "_text", "_top", "_height", ["_left", 0], ["_width", 1]];
    private _ctrl = [IDC_BUTTON + _n, _top, _height, _left, _width] call _fnc_place;
    _ctrl ctrlSetText _text;
    _ctrl ctrlSetBackgroundColor [1, 1, 1, 0.1];
    _ctrl ctrlSetTextColor [1, 1, 1, 1];
    _ctrl
};
private _fnc_status = {
    private _hour = floor daytime;
    private _minute = floor ((daytime - _hour) * 60);
    (_display displayCtrl IDC_STATUS) ctrlSetStructuredText parseText format [
        "<t align='left' font='RobotoCondensedBold'>%1:%2%3</t><t align='right' color='#c0c7d1'>%4</t>",
        _hour, ["", "0"] select (_minute < 10), _minute, _factionName];
};
// Players to pick from: everyone spawned in except you, optionally within range.
private _fnc_players = {
    params [["_range", -1]];
    private _list = allPlayers select {
        _x != player && { alive _x } && { !isObjectHidden _x } && { _range < 0 || { player distance _x <= _range } }
    };
    [_list, [], { name _x }] call BIS_fnc_sortBy
};
private _fnc_fillPlayers = {
    params ["_players", "_empty"];
    private _list = _display displayCtrl IDC_LIST2;
    private _keep = lnbCurSelRow _list;
    lnbClear _list;
    ALife_phoneTargets = _players;
    { _list lnbSetValue [[_list lnbAddRow [name _x call _fnc_safe], 0], _forEachIndex]; } forEach _players;
    if (_players isEqualTo []) then { _list lnbAddRow [_empty]; } else { _list lnbSetCurSelRow (_keep min (count _players - 1)); };
};
private _fnc_target = {
    private _list = _display displayCtrl IDC_LIST2;
    private _row = lnbCurSelRow _list;
    if (_row < 0 || { (missionNamespace getVariable ["ALife_phoneTargets", []]) isEqualTo [] }) exitWith { objNull };
    ALife_phoneTargets param [_list lnbValue [_row, 0], objNull]
};

switch (_mode) do {
    case "open": {
        if (!isNull (findDisplay IDD_PHONE)) exitWith {};
        createDialog "ALife_Phone";
        [player] remoteExec ["ALife_fnc_requestPhoneData", 2];
    };

    case "onLoad": {
        _args params ["_new"];
        uiNamespace setVariable ["ALife_phoneDisplay", _new];
        _display = _new;
        _display displayAddEventHandler ["KeyDown", {
            params ["", "_key"];
            if (_key == getNumber (missionConfigFile >> "CfgALife" >> "phoneKey")) exitWith { closeDialog 0; true };
            false
        }];
        ["home"] call ALife_fnc_phone;
    };

    case "home": {
        if (isNull _display) exitWith {};
        ALife_phoneApp = "";
        { (_display displayCtrl _x) ctrlShow false; } forEach _appControls;
        for "_i" from 0 to 7 do {
            { (_display displayCtrl (_x + _i)) ctrlShow true; } forEach [IDC_ICON, IDC_LABEL, IDC_TILE];
        };
        private _unread = missionNamespace getVariable ["ALife_unread", 0];
        (_display displayCtrl (IDC_LABEL + 3)) ctrlSetText (["Messages", format ["Messages (%1)", _unread]] select (_unread > 0));
        call _fnc_status;
    };

    case "app": {
        if (isNull _display) exitWith {};
        private _index = _args param [0, 0];
        ALife_phoneApp = _apps select _index;
        for "_i" from 0 to 7 do {
            { (_display displayCtrl (_x + _i)) ctrlShow false; } forEach [IDC_ICON, IDC_LABEL, IDC_TILE];
        };
        { (_display displayCtrl _x) ctrlShow false; } forEach _appControls;
        { (_display displayCtrl _x) ctrlShow true; } forEach [IDC_HEADER, IDC_BACK, IDC_TITLE];
        (_display displayCtrl IDC_TITLE) ctrlSetText (_titles select _index);
        (_display displayCtrl IDC_EDIT) ctrlSetText "";
        lnbClear (_display displayCtrl IDC_LIST);
        lnbClear (_display displayCtrl IDC_LIST2);

        switch (ALife_phoneApp) do {
            case "inventory": {
                [IDC_INFO, 0, 0.12] call _fnc_place;
                [IDC_LIST, 0.13, 0.42] call _fnc_place;
                [IDC_LIST2, 0.57, 0.2] call _fnc_place;
                ([IDC_EDIT, 0.79, 0.07, 0, 0.25] call _fnc_place) ctrlSetText "1";
                [0, "Give", 0.79, 0.07, 0.28, 0.72] call _fnc_button;
                [1, "Discard", 0.88, 0.07] call _fnc_button;
            };
            case "bank": {
                [IDC_INFO, 0, 0.22] call _fnc_place;
                [IDC_LIST2, 0.24, 0.48] call _fnc_place;
                [IDC_EDIT, 0.74, 0.07, 0, 0.4] call _fnc_place;
                [0, "Send", 0.74, 0.07, 0.43, 0.57] call _fnc_button;
            };
            case "messages": {
                ALife_unread = 0;
                if (isNil "ALife_phoneChannel") then { ALife_phoneChannel = "player"; };
                if (isNil "ALife_phoneAnon") then { ALife_phoneAnon = false; };
                [IDC_LIST, 0, 0.34] call _fnc_place;
                [IDC_INFO, 0.35, 0.16] call _fnc_place;
                [0, "Police", 0.53, 0.06, 0, 0.32] call _fnc_button;
                [1, "EMS", 0.53, 0.06, 0.34, 0.32] call _fnc_button;
                [2, "Player", 0.53, 0.06, 0.68, 0.32] call _fnc_button;
                [IDC_LIST2, 0.6, 0.12] call _fnc_place;
                [IDC_MESSAGE, 0.73, 0.13] call _fnc_place;
                [3, "", 0.88, 0.07, 0, 0.48] call _fnc_button;
                [4, "Send", 0.88, 0.07, 0.52, 0.48] call _fnc_button;
            };
            case "settings": {
                [IDC_INFO, 0, 0.18] call _fnc_place;
                [IDC_LIST, 0.2, 0.5] call _fnc_place;
                [0, "Apply view distance", 0.72, 0.07] call _fnc_button;
                [1, "", 0.81, 0.07] call _fnc_button;
                private _list = _display displayCtrl IDC_LIST;
                {
                    _list lnbSetValue [[_list lnbAddRow [format ["%1 m", _x], ""], 0], _x];
                } forEach [1000, 1500, 2000, 2500, 3000, 4000, 5000, 6000];
            };
            default {
                [IDC_INFO, 0, 1] call _fnc_place;
            };
        };
        ["refresh"] call ALife_fnc_phone;
    };

    case "refresh": {
        if (isNull _display) exitWith {};
        call _fnc_status;
        private _info = _display displayCtrl IDC_INFO;

        switch (missionNamespace getVariable ["ALife_phoneApp", ""]) do {
            case "": { ["home"] call ALife_fnc_phone; };

            case "inventory": {
                private _range = getNumber (missionConfigFile >> "CfgALife" >> "giveDistance");
                _info ctrlSetStructuredText parseText format [
                    "<t size='1.1' font='RobotoCondensedBold'>%1 items</t><br/><t size='0.85' color='#c0c7d1'>Pick an item, then give it to someone within %2 m, or discard it.</t>",
                    _factionName, _range];
                private _list = _display displayCtrl IDC_LIST;
                private _keep = lnbCurSelRow _list;
                lnbClear _list;
                private _items = (_data getOrDefault ["items", []]) select { (_x select 1) > 0 };
                {
                    _x params ["_class", "_count"];
                    private _row = _list lnbAddRow [getText (missionConfigFile >> "CfgItems" >> _class >> "displayName"), format ["x%1", _count]];
                    _list lnbSetData [[_row, 0], _class];
                    _list lnbSetPicture [[_row, 0], getText (missionConfigFile >> "CfgItems" >> _class >> "icon")];
                } forEach _items;
                if (_items isEqualTo []) then { _list lnbAddRow ["No items", ""]; } else { _list lnbSetCurSelRow ((_keep max 0) min (count _items - 1)); };
                [[_range] call _fnc_players, "Nobody nearby"] call _fnc_fillPlayers;
            };

            case "bank": {
                _info ctrlSetStructuredText parseText format [
                    "<t size='0.85' color='#c0c7d1'>%1 account</t><br/><t size='1.5' font='RobotoCondensedBold'>%2</t><t size='0.85' color='#c0c7d1'>  bank</t><br/><t size='1.1'>%3</t><t size='0.85' color='#c0c7d1'>  cash</t><br/><t size='0.8' color='#c0c7d1'>Send money from your bank to a player:</t>",
                    _factionName, (_data getOrDefault ["bank", 0]) call _fnc_money, (_data getOrDefault ["cash", 0]) call _fnc_money];
                [[] call _fnc_players, "Nobody else is online"] call _fnc_fillPlayers;
            };

            case "levels": {
                private _cop = _data getOrDefault ["copLevel", 0];
                private _medic = _data getOrDefault ["medicLevel", 0];
                private _staff = _data getOrDefault ["staffRank", ""];
                private _staffName = if (_staff == "") then { "None" } else {
                    ((_staff splitString "_") apply { (toUpper (_x select [0, 1])) + (_x select [1]) }) joinString " "
                };
                _info ctrlSetStructuredText parseText format [
                    "<t size='0.85' color='#c0c7d1'>POLICE</t><br/><t size='1.4' font='RobotoCondensedBold'>Level %1</t><br/><t size='0.85' color='#c0c7d1'>%2</t><br/><br/>" +
                    "<t size='0.85' color='#c0c7d1'>EMS</t><br/><t size='1.4' font='RobotoCondensedBold'>Level %3</t><br/><t size='0.85' color='#c0c7d1'>%4</t><br/><br/>" +
                    "<t size='0.85' color='#c0c7d1'>STAFF</t><br/><t size='1.2' font='RobotoCondensedBold'>%5</t>",
                    _cop, ["Not whitelisted", "Whitelisted"] select (_cop > 0),
                    _medic, ["Not whitelisted", "Whitelisted"] select (_medic > 0),
                    _staffName call _fnc_safe];
            };

            case "messages": {
                private _list = _display displayCtrl IDC_LIST;
                private _keep = lnbCurSelRow _list;
                lnbClear _list;
                private _inbox = missionNamespace getVariable ["ALife_inbox", []];
                {
                    _x params ["_from", "_text", "_time"];
                    _list lnbAddRow [(_from call _fnc_safe), _time];
                } forEach _inbox;
                if (_inbox isEqualTo []) then {
                    _list lnbAddRow ["No messages yet", ""];
                    _info ctrlSetStructuredText parseText "<t size='0.85' color='#c0c7d1'>Messages you receive show here. Write one below.</t>";
                } else {
                    _list lnbSetCurSelRow ((_keep max 0) min (count _inbox - 1));
                    ["select", IDC_LIST, lnbCurSelRow _list] call ALife_fnc_phone;
                };

                {
                    _x params ["_n", "_channel"];
                    private _button = _display displayCtrl (IDC_BUTTON + _n);
                    private _on = ALife_phoneChannel == _channel;
                    _button ctrlSetBackgroundColor ([[1, 1, 1, 0.1], [0.96, 0.62, 0.04, 1]] select _on);
                    _button ctrlSetTextColor ([[1, 1, 1, 1], [0.05, 0.05, 0.05, 1]] select _on);
                } forEach [[0, "police"], [1, "ems"], [2, "player"]];
                (_display displayCtrl IDC_LIST2) ctrlShow (ALife_phoneChannel == "player");
                if (ALife_phoneChannel == "player") then { [[] call _fnc_players, "Nobody else is online"] call _fnc_fillPlayers; };
                (_display displayCtrl (IDC_BUTTON + 3)) ctrlSetText (["Anonymous: Off", "Anonymous: On"] select ALife_phoneAnon);
            };

            case "settings": {
                (["get"] call ALife_fnc_phoneSettings) params ["_distance", "_grass"];
                _info ctrlSetStructuredText parseText format [
                    "<t size='0.85' color='#c0c7d1'>View distance</t><br/><t size='1.3' font='RobotoCondensedBold'>%1 m</t><br/><t size='0.8' color='#c0c7d1'>Pick a distance below and apply it.</t>",
                    _distance];
                (_display displayCtrl (IDC_BUTTON + 1)) ctrlSetText (["Grass: Off", "Grass: On"] select _grass);
            };

            default {
                _info ctrlSetStructuredText parseText format [
                    "<t size='1.2' font='RobotoCondensedBold'>Coming soon</t><br/><br/><t size='0.9' color='#c0c7d1'>%1</t>",
                    switch (ALife_phoneApp) do {
                        case "gang": { "Create or join a gang, invite members and manage your gang bank." };
                        case "keys": { "Give your vehicle keys to someone within 10 m. Arrives with vehicles." };
                        default { "Staff tools for moderating the server in game." };
                    }];
            };
        };
    };

    case "select": {
        _args params ["_idc", "_row"];
        if (isNull _display || { _row < 0 }) exitWith {};
        if (ALife_phoneApp == "messages" && { _idc == IDC_LIST }) then {
            private _inbox = missionNamespace getVariable ["ALife_inbox", []];
            if (_row < count _inbox) then {
                (_inbox select _row) params ["_from", "_text", "_time"];
                (_display displayCtrl IDC_INFO) ctrlSetStructuredText parseText format [
                    "<t size='0.85' color='#c0c7d1'>%1  |  %2</t><br/><t size='0.95'>%3</t>",
                    _from call _fnc_safe, _time, _text call _fnc_safe];
            };
        };
    };

    case "button": {
        _args params ["_n"];
        if (isNull _display) exitWith {};

        switch (ALife_phoneApp) do {
            case "inventory": {
                private _list = _display displayCtrl IDC_LIST;
                private _row = lnbCurSelRow _list;
                private _class = if (_row < 0) then { "" } else { _list lnbData [_row, 0] };
                private _qty = floor parseNumber ctrlText (_display displayCtrl IDC_EDIT);
                if (_class == "") exitWith { hint "Pick an item first."; };
                if (_qty < 1) exitWith { hint "Enter how many."; };
                if (_n == 0) then {
                    private _target = call _fnc_target;
                    if (isNull _target) exitWith { hint "Pick who to give it to."; };
                    [player, _target, _class, _qty] remoteExec ["ALife_fnc_giveItem", 2];
                } else {
                    [player, objNull, _class, _qty] remoteExec ["ALife_fnc_giveItem", 2];
                };
            };

            case "bank": {
                private _target = call _fnc_target;
                private _amount = floor parseNumber ctrlText (_display displayCtrl IDC_EDIT);
                if (isNull _target) exitWith { hint "Pick who to send it to."; };
                if (_amount < 1) exitWith { hint "Enter an amount."; };
                [player, _target, _amount] remoteExec ["ALife_fnc_bankTransfer", 2];
                (_display displayCtrl IDC_EDIT) ctrlSetText "";
            };

            case "messages": {
                switch (_n) do {
                    case 0: { ALife_phoneChannel = "police"; ["refresh"] call ALife_fnc_phone; };
                    case 1: { ALife_phoneChannel = "ems"; ["refresh"] call ALife_fnc_phone; };
                    case 2: { ALife_phoneChannel = "player"; ["refresh"] call ALife_fnc_phone; };
                    case 3: { ALife_phoneAnon = !ALife_phoneAnon; ["refresh"] call ALife_fnc_phone; };
                    case 4: {
                        private _text = ctrlText (_display displayCtrl IDC_MESSAGE);
                        private _target = [objNull, call _fnc_target] select (ALife_phoneChannel == "player");
                        if ((_text splitString " ") isEqualTo []) exitWith { hint "Type a message first."; };
                        if (ALife_phoneChannel == "player" && { isNull _target }) exitWith { hint "Pick who to message."; };
                        [player, ALife_phoneChannel, _target, _text, ALife_phoneAnon] remoteExec ["ALife_fnc_sendMessage", 2];
                        (_display displayCtrl IDC_MESSAGE) ctrlSetText "";
                    };
                };
            };

            case "settings": {
                if (_n == 0) then {
                    private _list = _display displayCtrl IDC_LIST;
                    private _row = lnbCurSelRow _list;
                    if (_row < 0) exitWith { hint "Pick a view distance first."; };
                    ["viewDistance", _list lnbValue [_row, 0]] call ALife_fnc_phoneSettings;
                } else {
                    ["toggleGrass"] call ALife_fnc_phoneSettings;
                };
                ["refresh"] call ALife_fnc_phone;
            };
        };
    };
};
