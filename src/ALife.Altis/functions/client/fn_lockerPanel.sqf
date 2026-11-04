/*
    ALife_fnc_lockerPanel (client)

    One locker for everything: while your locker is open in Arma's inventory
    (the gear crate, ALife_fnc_lockerClient), this panel is drawn inside the
    same screen, attached to the left of Arma's inventory window, the same
    height and in its look, for your virtual items -- what you carry, what's
    in the locker, a quantity, and Store / Take (the server moves and saves
    them, ALife_fnc_containerTransfer, container kind "locker").

    Modes:
        ["build"]   - once Arma's inventory (display 602) is open
        ["refresh"] - new locker contents (ALife_fnc_containerData) or your
                      own items (phone data)
        ["move", "store" | "take"]
*/

#define IDC_MINE 9310
#define IDC_THEIRS 9311
#define IDC_QTY 9312
#define IDC_HEAD 9313

params [["_mode", "", [""]], ["_arg", ""]];

private _display = findDisplay 602;
if (isNull _display) exitWith {};

switch (_mode) do {
    case "build": {
        if (!isNull (_display displayCtrl IDC_MINE)) exitWith { ["refresh"] call ALife_fnc_lockerPanel; };
        // Where Arma's inventory window is on this screen: its leftmost
        // control, and the top / bottom of the whole window.
        private _left = safezoneX + safezoneW * 0.27;
        private _top = safezoneY + safezoneH * 0.25;
        private _bottom = safezoneY + safezoneH * 0.75;
        private _parts = (allControls _display) select {
            // Arma's own controls only: ours (this panel, notifications) are ALife_ classes.
            ctrlShown _x && { (ctrlClassName _x) find "ALife_" != 0 } && {
                (ctrlPosition _x) params ["_cx", "_cy", "_cw", "_ch"];
                _cw > 0.01 && { _ch > 0.01 } && { _cw < safezoneW * 0.9 } && { _ch < safezoneH * 0.9 } && { _cx >= safezoneX } && { _cy >= safezoneY }
            }
        };
        if (_parts isNotEqualTo []) then {
            _left = selectMin (_parts apply { (ctrlPosition _x) select 0 });
            _top = selectMin (_parts apply { (ctrlPosition _x) select 1 });
            _bottom = selectMax (_parts apply { ((ctrlPosition _x) select 1) + ((ctrlPosition _x) select 3) });
        };
        private _gap = safezoneW * 0.006;
        private _w = (safezoneW * 0.15) min (_left - _gap - (safezoneX + safezoneW * 0.005));
        if (_w < safezoneW * 0.1) then { _w = safezoneW * 0.15; }; // no room beside it: overlap rather than squash
        private _x0 = _left - _gap - _w;
        private _h = _bottom - _top;
        private _pad = _w * 0.04;
        private _line = safezoneH * 0.026;     // one row, like Arma's inventory
        private _fh = safezoneH * 0.021;       // text height

        private _place = {
            params ["_class", "_idc", "_yy", "_hh", ["_dx", 0], ["_ww", -1]];
            private _c = _display ctrlCreate [_class, _idc];
            _c ctrlSetPosition [_x0 + _dx, _yy, [_ww, _w] select (_ww < 0), _hh];
            _c ctrlCommit 0;
            _c
        };
        // The window: translucent black with a black title bar, like Arma's.
        (["ALife_RscBackground", 9300, _top, _h] call _place) ctrlSetBackgroundColor [0, 0, 0, 0.6];
        (["ALife_RscBackground", 9301, _top, _line] call _place) ctrlSetBackgroundColor [0, 0, 0, 0.85];
        private _inner = _w - 2 * _pad;
        (["ALife_RscStructuredText", IDC_HEAD, _top, _line, _pad, _inner] call _place) ctrlSetFontHeight _fh;

        private _label = {
            params ["_text", "_yy"];
            private _c = ["ALife_RscText", -1, _yy, _line, _pad, _inner] call _place;
            _c ctrlSetText _text;
            _c ctrlSetFontHeight (_fh * 0.9);
            _c ctrlSetTextColor [0.75, 0.78, 0.84, 1];
        };
        private _btnH = _line * 1.4;
        private _listH = (_h - _line - 2 * _line - _btnH - 4 * _pad) / 2;
        private _y = _top + _line + _pad;
        ["You carry", _y] call _label;
        private _mine = ["ALife_RscListBox", IDC_MINE, _y + _line, _listH, _pad, _inner] call _place;
        _y = _y + _line + _listH + _pad;
        ["In your locker", _y] call _label;
        private _theirs = ["ALife_RscListBox", IDC_THEIRS, _y + _line, _listH, _pad, _inner] call _place;
        {
            _x ctrlSetFontHeight _fh;
            _x ctrlSetBackgroundColor [0, 0, 0, 0.35];
        } forEach [_mine, _theirs];

        // Qty, Store and Take along the bottom.
        private _by = _bottom - _pad - _btnH;
        private _qty = ["ALife_RscEdit", IDC_QTY, _by, _btnH, _pad, _inner * 0.22] call _place;
        _qty ctrlSetText "1";
        _qty ctrlSetFontHeight _fh;
        _qty ctrlSetTooltip "How many";
        {
            _x params ["_idc", "_text", "_dx", "_dir", "_primary"];
            private _b = ["ALife_RscButton", _idc, _by, _btnH, _pad + _inner * _dx, _inner * 0.37] call _place;
            _b ctrlSetText _text;
            _b ctrlSetFontHeight _fh;
            _b ctrlSetBackgroundColor ([[1, 1, 1, 0.12], [0.96, 0.62, 0.04, 0.9]] select _primary);
            _b ctrlSetTextColor ([[1, 1, 1, 1], [0.118, 0.071, 0, 1]] select _primary);
            _b setVariable ["alife_dir", _dir];
            _b ctrlAddEventHandler ["ButtonClick", { ["move", (_this select 0) getVariable "alife_dir"] call ALife_fnc_lockerPanel; }];
        } forEach [[9314, "Store", 0.25, "store", true], [9315, "Take", 0.63, "take", false]];
        ["refresh"] call ALife_fnc_lockerPanel;
    };

    case "refresh": {
        private _fill = {
            params ["_list", "_pairs", "_empty"];
            private _keep = if (lbCurSel _list < 0) then { "" } else { _list lbData (lbCurSel _list) };
            lbClear _list;
            {
                _x params ["_class", "_count"];
                if (_count > 0 && { isClass (missionConfigFile >> "CfgItems" >> _class) }) then {
                    private _i = _list lbAdd format ["%1  x%2", getText (missionConfigFile >> "CfgItems" >> _class >> "displayName"), _count];
                    _list lbSetData [_i, _class];
                    _list lbSetPicture [_i, getText (missionConfigFile >> "CfgItems" >> _class >> "icon")];
                    if (_class == _keep) then { _list lbSetCurSel _i; };
                };
            } forEach _pairs;
            if (lbSize _list == 0) then { _list lbAdd _empty; _list lbSetColor [0, [0.545, 0.596, 0.702, 1]]; };
        };
        private _mine = _display displayCtrl IDC_MINE;
        if (isNull _mine) exitWith {};
        [_mine, ((missionNamespace getVariable ["ALife_phone", createHashMap]) getOrDefault ["items", []]) select { _x isEqualTypeParams ["", 0] }, "Nothing"] call _fill;
        private _locker = missionNamespace getVariable ["ALife_lockerItems", []];
        [_display displayCtrl IDC_THEIRS, (_locker param [3, []]) select { _x isEqualTypeParams ["", 0] }, "Empty"] call _fill;
        // Title bar: "Locker items" and the space used, like Arma's header.
        (_display displayCtrl IDC_HEAD) ctrlSetStructuredText parseText format [
            "<t valign='middle'>Locker items</t><t align='right' valign='middle' color='#aeb4c0'>%1 / %2</t>",
            _locker param [5, 0], _locker param [6, 0]];
    };

    case "move": {
        private _store = _arg == "store";
        private _list = _display displayCtrl ([IDC_THEIRS, IDC_MINE] select _store);
        private _class = if (lbCurSel _list < 0) then { "" } else { _list lbData (lbCurSel _list) };
        private _qty = floor parseNumber ctrlText (_display displayCtrl IDC_QTY);
        private _id = (missionNamespace getVariable ["ALife_lockerItems", []]) param [0, ""];
        if (_class == "") exitWith { [["Pick an item in your locker.", "Pick one of your items."] select _store, "warning"] call ALife_fnc_toast; };
        if (_qty < 1) exitWith { ["Enter how many.", "warning"] call ALife_fnc_toast; };
        if (_id == "") exitWith {};
        [player, _id, _arg, [[_class, _qty]], 0] remoteExecCall ["ALife_fnc_containerTransfer", 2];
    };
};
