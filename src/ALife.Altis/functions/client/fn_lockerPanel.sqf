/*
    ALife_fnc_lockerPanel (client)

    One locker for everything: while your locker is open in Arma's inventory
    (the gear crate, ALife_fnc_lockerClient), this panel is drawn inside the
    same screen, at its left edge, for your virtual items -- what you carry,
    what's in the locker, a quantity, and Store / Take (the server moves and
    saves them, ALife_fnc_containerTransfer, container kind "locker").

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
        private _x0 = safezoneX + safezoneW * 0.008;
        private _w = safezoneW * 0.15;
        private _y = safezoneY + safezoneH * 0.16;
        private _place = {
            params ["_class", "_idc", "_yy", "_h", ["_dx", 0], ["_ww", 1]];
            private _c = _display ctrlCreate [_class, _idc];
            _c ctrlSetPosition [_x0 + _w * _dx, _yy, _w * _ww, _h];
            _c ctrlCommit 0;
            _c
        };
        (["ALife_RscBackground", -1, _y, safezoneH * 0.66] call _place) ctrlSetBackgroundColor [0.059, 0.09, 0.165, 0.94];
        (["ALife_RscBackground", -1, _y, safezoneH * 0.004] call _place) ctrlSetBackgroundColor [0.984, 0.749, 0.141, 1];
        private _head = ["ALife_RscStructuredText", IDC_HEAD, _y + safezoneH * 0.012, safezoneH * 0.05, 0.05, 0.9] call _place;
        _head ctrlSetStructuredText parseText "<t font='RobotoCondensedBold' size='1.1'>LOCKER ITEMS</t><br/><t size='0.8' color='#8b98b3'>Virtual items. Gear goes in the crate.</t>";
        (["ALife_RscText", -1, _y + safezoneH * 0.07, safezoneH * 0.025, 0.05, 0.9] call _place) ctrlSetText "You carry";
        private _mine = ["ALife_RscListBox", IDC_MINE, _y + safezoneH * 0.095, safezoneH * 0.2, 0.05, 0.9] call _place;
        (["ALife_RscText", -1, _y + safezoneH * 0.305, safezoneH * 0.025, 0.05, 0.9] call _place) ctrlSetText "In your locker";
        private _theirs = ["ALife_RscListBox", IDC_THEIRS, _y + safezoneH * 0.33, safezoneH * 0.2, 0.05, 0.9] call _place;
        (["ALife_RscText", -1, _y + safezoneH * 0.545, safezoneH * 0.03, 0.05, 0.22] call _place) ctrlSetText "Qty";
        private _qty = ["ALife_RscEdit", IDC_QTY, _y + safezoneH * 0.545, safezoneH * 0.03, 0.27, 0.2] call _place;
        _qty ctrlSetText "1";
        private _store = ["ALife_RscButton", -1, _y + safezoneH * 0.59, safezoneH * 0.04, 0.05, 0.43] call _place;
        _store ctrlSetText "Store";
        _store ctrlAddEventHandler ["ButtonClick", { ["move", "store"] call ALife_fnc_lockerPanel; }];
        private _take = ["ALife_RscButton", -1, _y + safezoneH * 0.59, safezoneH * 0.04, 0.52, 0.43] call _place;
        _take ctrlSetText "Take";
        _take ctrlAddEventHandler ["ButtonClick", { ["move", "take"] call ALife_fnc_lockerPanel; }];
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
        (_display displayCtrl IDC_HEAD) ctrlSetStructuredText parseText format [
            "<t font='RobotoCondensedBold' size='1.1'>LOCKER ITEMS</t><br/><t size='0.8' color='#8b98b3'>%1 / %2 space used. Gear goes in the crate.</t>",
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
