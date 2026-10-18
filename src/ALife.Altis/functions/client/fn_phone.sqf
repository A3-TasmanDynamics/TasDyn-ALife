/*
    ALife_fnc_phone (client)

    The phone (ui\phone.hpp, opened with Y). The home screen has a clock and
    a 3 x 3 grid of app icons; each app arranges the same set of generic
    controls (cards, labels, info text, two lists, an amount box, a message
    box, buttons) for itself. Everything that changes something is a
    request to the server, which answers with fresh phone data
    (ALife_fnc_phoneData) -- that refreshes the open app.

    Apps: Messages, Bank, Inventory, Player ID, Settings; Perks, Gang, Keys
    and Admin are "coming soon".

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
#define IDC_BODY     5001
#define IDC_STATUS   5003
#define IDC_CLOCK    5005
#define IDC_ICON     5110
#define IDC_LABEL    5120
#define IDC_TILE     5130
#define IDC_HEADER   5201
#define IDC_BACK     5202
#define IDC_TITLE    5203
#define IDC_APPICON  5204
#define IDC_INFO     5210
#define IDC_LIST     5211
#define IDC_LIST2    5212
#define IDC_EDIT     5213
#define IDC_MESSAGE  5214
#define IDC_LABEL0   5215
#define IDC_BUTTON   5220
#define IDC_CARD0    5230
#define IDC_COMBO    5240
#define IDC_COMBO2   5241

params ["_mode"];
private _args = _this select [1];
private _display = uiNamespace getVariable ["ALife_phoneDisplay", displayNull];

private _apps = ["messages", "bank", "inventory", "playerid", "perks", "gang", "keys", "admin", "settings"];
private _titles = ["Messages", "Bank", "Inventory", "Player ID", "Perks", "Gang", "Keys", "Admin", "Settings"];
private _data = missionNamespace getVariable ["ALife_phone", createHashMap];
private _faction = _data getOrDefault ["faction", missionNamespace getVariable ["ALife_faction", "civ"]];
private _factionName = switch (_faction) do { case "cop": { "Police" }; case "medic": { "EMS" }; default { "Civilian" }; };
private _appControls = [IDC_HEADER, IDC_BACK, IDC_TITLE, IDC_APPICON, IDC_INFO, IDC_LIST, IDC_LIST2, IDC_EDIT, IDC_MESSAGE]
    + ([0, 1, 2] apply { IDC_LABEL0 + _x })
    + ([0, 1] apply { IDC_CARD0 + _x })
    + [IDC_COMBO, IDC_COMBO2]
    + ([0, 1, 2, 3, 4, 5] apply { IDC_BUTTON + _x });

private _fnc_safe = { (_this splitString "<>&") joinString "" };

// Sharp text: structured text sits on a 1-pixel base (ui\phone.hpp), so
// every <t size> is a whole number of screen pixels. Sizes below are for a
// 1080p screen and scale with the resolution, like the phone itself.
private _k = (safezoneH / pixelH) / 1080;
private _fnc_px = { str ((round (_this * _k)) max 8) };
private _fnc_snapX = { (round (_this / pixelW)) * pixelW };
private _fnc_snapY = { (round (_this / pixelH)) * pixelH };
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };

// Place a control in the app area: top/height (and left/width) are
// fractions of that area.
private _fnc_place = {
    params ["_idc", "_top", "_height", ["_left", 0], ["_width", 1]];
    private _ctrl = _display displayCtrl _idc;
    _ctrl ctrlSetPosition [
        (safezoneX + safezoneW * (0.757 + 0.181 * _left)) call _fnc_snapX,
        (safezoneY + safezoneH * (0.382 + 0.545 * _top)) call _fnc_snapY,
        (safezoneW * 0.181 * _width) call _fnc_snapX,
        (safezoneH * 0.545 * _height) call _fnc_snapY
    ];
    _ctrl ctrlCommit 0;
    _ctrl ctrlShow true;
    _ctrl
};
private _fnc_label = {
    params ["_n", "_text", "_top", ["_left", 0], ["_width", 1]];
    ([IDC_LABEL0 + _n, _top, 0.045, _left, _width] call _fnc_place) ctrlSetText _text;
};
// Buttons: grey, or amber for the app's main action.
private _fnc_button = {
    params ["_n", "_text", "_top", "_height", ["_left", 0], ["_width", 1], ["_primary", false]];
    private _ctrl = [IDC_BUTTON + _n, _top, _height, _left, _width] call _fnc_place;
    _ctrl ctrlSetText _text;
    _ctrl ctrlSetBackgroundColor ([[1, 1, 1, 0.09], [0.96, 0.62, 0.04, 1]] select _primary);
    _ctrl ctrlSetTextColor ([[1, 1, 1, 1], [0.118, 0.071, 0, 1]] select _primary);
    _ctrl
};
private _fnc_card = {
    params ["_n", "_top", "_height", ["_colour", [1, 1, 1, 0.05]]];
    ([IDC_CARD0 + _n, _top, _height] call _fnc_place) ctrlSetBackgroundColor _colour;
};
private _fnc_time = {
    private _hour = floor daytime;
    private _minute = floor ((daytime - _hour) * 60);
    format ["%1:%2%3", _hour, ["", "0"] select (_minute < 10), _minute]
};
private _fnc_status = {
    (_display displayCtrl IDC_STATUS) ctrlSetStructuredText parseText format [
        "<t align='left' size='%3' font='RobotoCondensedBold'>%1</t><t align='right' size='%4' color='#8b98b3'>%2</t>",
        call _fnc_time, _factionName, 24 call _fnc_px, 22 call _fnc_px];
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
    if (_players isEqualTo []) then {
        _list lnbAddRow [_empty];
        _list lnbSetColor [[0, 0], [0.545, 0.596, 0.702, 1]];
    } else {
        _list lnbSetCurSelRow ((_keep max 0) min (count _players - 1));
    };
};
private _fnc_target = {
    private _list = _display displayCtrl IDC_LIST2;
    private _row = lnbCurSelRow _list;
    if (_row < 0 || { (missionNamespace getVariable ["ALife_phoneTargets", []]) isEqualTo [] }) exitWith { objNull };
    ALife_phoneTargets param [_list lnbValue [_row, 0], objNull]
};
private _fnc_comingSoon = {
    params ["_what"];
    [0, 0, 0.42] call _fnc_card;
    ([IDC_INFO, 0.03, 0.38, 0.05, 0.9] call _fnc_place) ctrlSetStructuredText parseText format [
        "<t size='%2' color='#fbbf24' font='RobotoCondensedBold'>COMING SOON</t><br/><br/><t size='%3' color='#d6dbe2'>%1</t>",
        _what, 22 call _fnc_px, 27 call _fnc_px];
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
        // Whole-pixel positions and font sizes keep the text sharp.
        {
            (ctrlPosition _x) params ["_px", "_py", "_pw", "_ph"];
            _x ctrlSetPosition [_px call _fnc_snapX, _py call _fnc_snapY, _pw call _fnc_snapX, _ph call _fnc_snapY];
            _x ctrlCommit 0;
            if (ctrlType _x in [0, 1, 2, 4, 102]) then {
                _x ctrlSetFontHeight ((ctrlFontHeight _x) call _fnc_snapY);
            };
        } forEach (allControls _display);
        _display displayAddEventHandler ["KeyDown", {
            params ["_disp", "_key"];
            if (_key != getNumber (missionConfigFile >> "CfgALife" >> "phoneKey")) exitWith { false };
            // Typing in a text box (message, amount): Y is just a letter.
            if (ctrlType (focusedCtrl _disp) == 2) exitWith { false };
            closeDialog 0;
            true
        }];
        ["home"] call ALife_fnc_phone;
    };

    case "home": {
        if (isNull _display) exitWith {};
        ALife_phoneApp = "";
        { (_display displayCtrl _x) ctrlShow false; } forEach _appControls;
        for "_i" from 0 to 8 do {
            { (_display displayCtrl (_x + _i)) ctrlShow true; } forEach [IDC_ICON, IDC_LABEL, IDC_TILE];
        };
        (_display displayCtrl IDC_CLOCK) ctrlShow true;
        private _unread = missionNamespace getVariable ["ALife_unread", 0];
        (_display displayCtrl IDC_LABEL) ctrlSetText (["Messages", format ["Messages (%1)", _unread]] select (_unread > 0));

        (_display displayCtrl IDC_BODY) ctrlSetText (["wallpaperTexture"] call ALife_fnc_phoneSettings);

        // Stacked clock (hours over minutes) and "Sat, 24 Jun", top-left.
        date params ["_year", "_month", "_day"];
        private _months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
        // Day of the week (Zeller's congruence, 0 = Saturday).
        private _m = [_month, _month + 12] select (_month < 3);
        private _y = [_year, _year - 1] select (_month < 3);
        private _weekday = (_day + floor (13 * (_m + 1) / 5) + (_y % 100) + floor ((_y % 100) / 4) + floor ((floor (_y / 100)) / 4) + 5 * floor (_y / 100)) % 7;
        private _hour = floor daytime;
        private _minute = floor ((daytime - _hour) * 60);
        (_display displayCtrl IDC_CLOCK) ctrlSetStructuredText parseText format [
            "<t size='%4' font='RobotoCondensedLight'>%1</t><br/><t size='%4' font='RobotoCondensedLight'>%2</t><br/><t size='%5' color='#f1f5f9'>%3</t>",
            _hour, (["", "0"] select (_minute < 10)) + str _minute,
            format ["%1, %2 %3", ["Sat", "Sun", "Mon", "Tue", "Wed", "Thu", "Fri"] select _weekday, _day, _months select ((_month - 1) max 0)],
            64 call _fnc_px, 24 call _fnc_px];
        call _fnc_status;
    };

    case "app": {
        if (isNull _display) exitWith {};
        private _index = _args param [0, 0];
        ALife_phoneApp = _apps select _index;
        for "_i" from 0 to 8 do {
            { (_display displayCtrl (_x + _i)) ctrlShow false; } forEach [IDC_ICON, IDC_LABEL, IDC_TILE];
        };
        (_display displayCtrl IDC_CLOCK) ctrlShow false;
        { (_display displayCtrl _x) ctrlShow false; } forEach _appControls;
        { (_display displayCtrl _x) ctrlShow true; } forEach [IDC_HEADER, IDC_BACK, IDC_TITLE, IDC_APPICON];
        (_display displayCtrl IDC_TITLE) ctrlSetText (_titles select _index);
        (_display displayCtrl IDC_APPICON) ctrlSetText format ["ui\icons\%1_ca.paa", ALife_phoneApp];
        (_display displayCtrl IDC_BODY) ctrlSetText getText (missionConfigFile >> "CfgPhone" >> "appBackground");
        (_display displayCtrl IDC_EDIT) ctrlSetText "";
        lnbClear (_display displayCtrl IDC_LIST);
        lnbClear (_display displayCtrl IDC_LIST2);
        (_display displayCtrl IDC_LIST) lnbSetColumnsPos [0.04, 0.72];
        (_display displayCtrl IDC_LIST2) lnbSetColumnsPos [0.04];

        switch (ALife_phoneApp) do {
            case "messages": {
                ALife_unread = 0;
                if (isNil "ALife_phoneChannel") then { ALife_phoneChannel = "player"; };
                if (isNil "ALife_phoneAnon") then { ALife_phoneAnon = false; };
                [0, "INBOX", 0] call _fnc_label;
                [IDC_LIST, 0.05, 0.26] call _fnc_place;
                [0, 0.33, 0.14] call _fnc_card;
                [IDC_INFO, 0.335, 0.13, 0.03, 0.94] call _fnc_place;
                [1, "NEW MESSAGE", 0.5] call _fnc_label;
                [0, "Police", 0.55, 0.06, 0, 0.32] call _fnc_button;
                [1, "EMS", 0.55, 0.06, 0.34, 0.32] call _fnc_button;
                [2, "Player", 0.55, 0.06, 0.68, 0.32] call _fnc_button;
                [IDC_LIST2, 0.63, 0.1] call _fnc_place;
                [2, "", 0.63] call _fnc_label;
                [IDC_MESSAGE, 0.745, 0.12] call _fnc_place;
                [3, "", 0.885, 0.075, 0, 0.48] call _fnc_button;
                [4, "Send", 0.885, 0.075, 0.52, 0.48, true] call _fnc_button;
            };
            case "bank": {
                [0, 0, 0.25, [0.96, 0.62, 0.04, 0.14]] call _fnc_card;
                [IDC_INFO, 0.015, 0.23, 0.04, 0.92] call _fnc_place;
                [0, "SEND MONEY TO", 0.29] call _fnc_label;
                [IDC_LIST2, 0.34, 0.36] call _fnc_place;
                [1, "AMOUNT ($)", 0.72] call _fnc_label;
                [IDC_EDIT, 0.77, 0.075, 0, 0.45] call _fnc_place;
                [0, "Send", 0.77, 0.075, 0.5, 0.5, true] call _fnc_button;
            };
            case "inventory": {
                [0, toUpper _factionName + " ITEMS", 0] call _fnc_label;
                [IDC_LIST, 0.05, 0.34] call _fnc_place;
                [1, format ["GIVE TO (WITHIN %1 M)", getNumber (missionConfigFile >> "CfgALife" >> "giveDistance")], 0.42] call _fnc_label;
                [IDC_LIST2, 0.47, 0.22] call _fnc_place;
                [2, "QTY", 0.72, 0, 0.25] call _fnc_label;
                ([IDC_EDIT, 0.77, 0.075, 0, 0.25] call _fnc_place) ctrlSetText "1";
                [0, "Give", 0.77, 0.075, 0.28, 0.72, true] call _fnc_button;
                [1, "Drop", 0.87, 0.07] call _fnc_button;
            };
            case "playerid": {
                [0, 0, 0.36, [0.08, 0.63, 0.59, 0.16]] call _fnc_card;
                [IDC_INFO, 0.02, 0.33, 0.04, 0.92] call _fnc_place;
                [0, "LICENCES", 0.4] call _fnc_label;
                [IDC_LIST, 0.45, 0.48] call _fnc_place;
            };
            case "settings": {
                [0, "VIEW DISTANCE", 0] call _fnc_label;
                private _combo = [IDC_COMBO, 0.05, 0.07] call _fnc_place;
                [1, "GRASS", 0.16] call _fnc_label;
                [1, "", 0.21, 0.075] call _fnc_button;
                [2, "WALLPAPER", 0.32] call _fnc_label;
                private _combo2 = [IDC_COMBO2, 0.37, 0.07] call _fnc_place;
                [IDC_INFO, 0.48, 0.2] call _fnc_place;
                // Fill the drop-downs; picking an option applies it straight away.
                (["get"] call ALife_fnc_phoneSettings) params ["_current", "", "_currentWallpaper"];
                ALife_phoneFilling = true;
                lbClear _combo2;
                {
                    private _row = _combo2 lbAdd getText (_x >> "displayName");
                    _combo2 lbSetData [_row, configName _x];
                    if (configName _x == _currentWallpaper) then { _combo2 lbSetCurSel _row; };
                } forEach ("true" configClasses (missionConfigFile >> "CfgPhone" >> "Wallpapers"));
                lbClear _combo;
                {
                    private _row = _combo lbAdd format ["%1 m", _x];
                    _combo lbSetValue [_row, _x];
                    if (_x == _current) then { _combo lbSetCurSel _row; };
                } forEach [500, 1000, 1500, 2000, 2500, 3000, 3500, 4000, 5000, 6000];
                ALife_phoneFilling = false;
            };
            case "perks": { ["A talent tree: earn perk points as you play and spend them on perks for your faction -- faster repairs, better prices, quicker revives and more."] call _fnc_comingSoon; };
            case "gang": { ["Create or join a gang, invite members, manage ranks and the gang bank."] call _fnc_comingSoon; };
            case "keys": { ["Your vehicle keys: give a key to someone within 10 m. Arrives with vehicles."] call _fnc_comingSoon; };
            case "admin": { ["Staff tools for moderating the server in game."] call _fnc_comingSoon; };
        };
        ["refresh"] call ALife_fnc_phone;
    };

    case "refresh": {
        if (isNull _display) exitWith {};
        call _fnc_status;
        private _info = _display displayCtrl IDC_INFO;

        switch (missionNamespace getVariable ["ALife_phoneApp", ""]) do {
            case "": { ["home"] call ALife_fnc_phone; };

            case "messages": {
                private _list = _display displayCtrl IDC_LIST;
                private _keep = lnbCurSelRow _list;
                lnbClear _list;
                private _inbox = missionNamespace getVariable ["ALife_inbox", []];
                {
                    _x params ["_from", "_text", "_time"];
                    _list lnbAddRow [_from call _fnc_safe, _time];
                } forEach _inbox;
                if (_inbox isEqualTo []) then {
                    _list lnbAddRow ["No messages yet", ""];
                    _list lnbSetColor [[0, 0], [0.545, 0.596, 0.702, 1]];
                    _info ctrlSetStructuredText parseText format ["<t size='%1' color='#8b98b3'>Messages you receive show up here.</t>", 26 call _fnc_px];
                } else {
                    _list lnbSetCurSelRow ((_keep max 0) min (count _inbox - 1));
                    ["select", IDC_LIST, lnbCurSelRow _list] call ALife_fnc_phone;
                };

                {
                    _x params ["_n", "_channel"];
                    private _button = _display displayCtrl (IDC_BUTTON + _n);
                    private _on = ALife_phoneChannel == _channel;
                    _button ctrlSetBackgroundColor ([[1, 1, 1, 0.09], [1, 1, 1, 0.85]] select _on);
                    _button ctrlSetTextColor ([[1, 1, 1, 1], [0.118, 0.071, 0, 1]] select _on);
                } forEach [[0, "police"], [1, "ems"], [2, "player"]];

                private _toPlayer = ALife_phoneChannel == "player";
                (_display displayCtrl IDC_LIST2) ctrlShow _toPlayer;
                (_display displayCtrl (IDC_LABEL0 + 2)) ctrlShow !_toPlayer;
                (_display displayCtrl (IDC_LABEL0 + 2)) ctrlSetText (["Goes to every on-duty police officer.", "Goes to every on-duty medic."] select (ALife_phoneChannel == "ems"));
                if (_toPlayer) then { [[] call _fnc_players, "Nobody else is online"] call _fnc_fillPlayers; };
                (_display displayCtrl (IDC_BUTTON + 3)) ctrlSetText (["Anonymous: Off", "Anonymous: On"] select ALife_phoneAnon);
            };

            case "bank": {
                _info ctrlSetStructuredText parseText format [
                    "<t size='%4' color='#f5c26b' font='RobotoCondensedBold'>%1 ACCOUNT</t><br/><t size='%5' font='RobotoCondensedBold'>%2</t><br/><t size='%6' color='#d6dbe2'>Cash on hand  </t><t size='%6' font='RobotoCondensedBold'>%3</t>",
                    toUpper _factionName, (_data getOrDefault ["bank", 0]) call _fnc_money, (_data getOrDefault ["cash", 0]) call _fnc_money,
                    22 call _fnc_px, 54 call _fnc_px, 26 call _fnc_px];
                [[] call _fnc_players, "Nobody else is online"] call _fnc_fillPlayers;
            };

            case "inventory": {
                private _list = _display displayCtrl IDC_LIST;
                private _keep = lnbCurSelRow _list;
                lnbClear _list;
                private _items = (_data getOrDefault ["items", []]) select { (_x select 1) > 0 };
                // Cash on hand first (physical money: give it or drop it like an item).
                private _cash = _data getOrDefault ["cash", 0];
                if (_cash > 0) then {
                    private _row = _list lnbAddRow ["Cash", _cash call _fnc_money];
                    _list lnbSetData [[_row, 0], "cash"];
                    _list lnbSetPicture [[_row, 0], getText (missionConfigFile >> "CfgALife" >> "cashIcon")];
                    _items = [["cash", _cash]] + _items; // keeps the row count right below
                };
                {
                    _x params ["_class", "_count"];
                    if (_class == "cash") then { continue; };
                    private _row = _list lnbAddRow [getText (missionConfigFile >> "CfgItems" >> _class >> "displayName"), format ["x%1", _count]];
                    _list lnbSetData [[_row, 0], _class];
                    _list lnbSetPicture [[_row, 0], getText (missionConfigFile >> "CfgItems" >> _class >> "icon")];
                } forEach _items;
                if (_items isEqualTo []) then {
                    _list lnbAddRow ["No items", ""];
                    _list lnbSetColor [[0, 0], [0.545, 0.596, 0.702, 1]];
                } else {
                    _list lnbSetCurSelRow ((_keep max 0) min (count _items - 1));
                };
                [[getNumber (missionConfigFile >> "CfgALife" >> "giveDistance")] call _fnc_players, "Nobody nearby"] call _fnc_fillPlayers;
            };

            case "playerid": {
                private _cop = _data getOrDefault ["copLevel", 0];
                private _medic = _data getOrDefault ["medicLevel", 0];
                private _staff = _data getOrDefault ["staffRank", ""];
                private _staffName = if (_staff == "") then { "" } else {
                    ((_staff splitString "_") apply { (toUpper (_x select [0, 1])) + (_x select [1]) }) joinString " "
                };
                _info ctrlSetStructuredText parseText format [
                    "<t size='%6' color='#5eead4' font='RobotoCondensedBold'>ALTIS IDENTITY CARD</t><br/>" +
                    "<t size='%7' font='RobotoCondensedBold'>%1</t><br/>" +
                    "<t size='%8' color='#d6dbe2'>%2</t><br/><br/>" +
                    "<t size='%9' color='#8b98b3'>Police  </t><t size='%9' font='RobotoCondensedBold'>%3</t><br/>" +
                    "<t size='%9' color='#8b98b3'>EMS  </t><t size='%9' font='RobotoCondensedBold'>%4</t>%5",
                    name player call _fnc_safe, _factionName,
                    ["Not whitelisted", format ["Level %1", _cop]] select (_cop > 0),
                    ["Not whitelisted", format ["Level %1", _medic]] select (_medic > 0),
                    ["", format ["<br/><t size='%2' color='#8b98b3'>Staff  </t><t size='%2' font='RobotoCondensedBold'>%1</t>", _staffName call _fnc_safe, 25 call _fnc_px]] select (_staff != ""),
                    22 call _fnc_px, 42 call _fnc_px, 26 call _fnc_px, 25 call _fnc_px];

                private _list = _display displayCtrl IDC_LIST;
                lnbClear _list;
                private _licences = _data getOrDefault ["licences", []];
                {
                    private _cfg = missionConfigFile >> "CfgLicences" >> _x;
                    _list lnbAddRow [[_x, getText (_cfg >> "displayName")] select (isClass _cfg), ""];
                } forEach _licences;
                if (_licences isEqualTo []) then {
                    _list lnbAddRow [format ["No %1 licences yet", toLower _factionName], ""];
                    _list lnbSetColor [[0, 0], [0.545, 0.596, 0.702, 1]];
                };
            };

            case "settings": {
                (["get"] call ALife_fnc_phoneSettings) params ["_distance", "_grass"];
                _info ctrlSetStructuredText parseText format [
                    "<t size='%2' color='#8b98b3'>Your view distance is %1 m. Lower it if the game runs slowly.<br/><br/>The wallpaper shows on the home screen; apps keep the dark background.</t>", _distance, 24 call _fnc_px];
                (_display displayCtrl (IDC_BUTTON + 1)) ctrlSetText (["Grass: Off", "Grass: On"] select _grass);
            };
        };
    };

    case "select": {
        _args params ["_idc", "_row"];
        if (isNull _display || { _row < 0 }) exitWith {};
        if (_idc == IDC_COMBO) exitWith {
            if (missionNamespace getVariable ["ALife_phoneFilling", false]) exitWith {};
            ["viewDistance", (_display displayCtrl IDC_COMBO) lbValue _row] call ALife_fnc_phoneSettings;
            ["refresh"] call ALife_fnc_phone;
        };
        if (_idc == IDC_COMBO2) exitWith {
            if (missionNamespace getVariable ["ALife_phoneFilling", false]) exitWith {};
            ["wallpaper", (_display displayCtrl IDC_COMBO2) lbData _row] call ALife_fnc_phoneSettings;
        };
        if (ALife_phoneApp == "messages" && { _idc == IDC_LIST }) then {
            private _inbox = missionNamespace getVariable ["ALife_inbox", []];
            if (_row < count _inbox) then {
                (_inbox select _row) params ["_from", "_text", "_time"];
                (_display displayCtrl IDC_INFO) ctrlSetStructuredText parseText format [
                    "<t size='%4' color='#8b98b3'>%1  |  %2</t><br/><t size='%5'>%3</t>",
                    _from call _fnc_safe, _time, _text call _fnc_safe, 22 call _fnc_px, 27 call _fnc_px];
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
                if (_class == "") exitWith { ["Pick an item first.", "warning"] call ALife_fnc_toast; };
                if (_qty < 1) exitWith { [["Enter how many.", "Enter an amount ($)."] select (_class == "cash"), "warning"] call ALife_fnc_toast; };
                if (_n == 0) then {
                    private _target = call _fnc_target;
                    if (isNull _target) exitWith { ["Pick who to give it to.", "warning"] call ALife_fnc_toast; };
                    [player, _target, _class, _qty] remoteExec ["ALife_fnc_giveItem", 2];
                } else {
                    // Drop: into a bag at your feet (cash: a money stack). Windows key to pick it up.
                    if (_class == "cash") then {
                        [player, "", "store", [], _qty] remoteExecCall ["ALife_fnc_containerTransfer", 2];
                    } else {
                        [player, "", "store", [[_class, _qty]], 0] remoteExecCall ["ALife_fnc_containerTransfer", 2];
                    };
                };
            };

            case "bank": {
                private _target = call _fnc_target;
                private _amount = floor parseNumber ctrlText (_display displayCtrl IDC_EDIT);
                if (isNull _target) exitWith { ["Pick who to send it to.", "warning"] call ALife_fnc_toast; };
                if (_amount < 1) exitWith { ["Enter an amount.", "warning"] call ALife_fnc_toast; };
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
                        if ((_text splitString " ") isEqualTo []) exitWith { ["Type a message first.", "warning"] call ALife_fnc_toast; };
                        if (ALife_phoneChannel == "player" && { isNull _target }) exitWith { ["Pick who to message.", "warning"] call ALife_fnc_toast; };
                        [player, ALife_phoneChannel, _target, _text, ALife_phoneAnon] remoteExec ["ALife_fnc_sendMessage", 2];
                        (_display displayCtrl IDC_MESSAGE) ctrlSetText "";
                    };
                };
            };

            case "settings": {
                if (_n == 1) then {
                    ["toggleGrass"] call ALife_fnc_phoneSettings;
                    ["refresh"] call ALife_fnc_phone;
                };
            };
        };
    };
};
