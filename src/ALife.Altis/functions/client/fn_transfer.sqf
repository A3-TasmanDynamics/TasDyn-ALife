/*
    ALife_fnc_transfer (client)

    The inventory screen (ui\transfer.hpp), shared by dropped bags and
    vehicle trunks: the bag/trunk on the left, you on the right. Select one
    or more rows (Ctrl/Shift-click), enter a quantity, then Take / Store
    (Drop on the ground). "Take all" / "Store all" move everything;
    double-click a row to move all of that one. Cash is a row like any item.

    The server does the moving (ALife_fnc_containerTransfer) and checks
    everything; it answers with fresh contents for both sides.

    ALife_container = [id, title, kind, items, cash, weight, capacity]
    (id "" = the ground in front of you, before anything's dropped).

    Modes:
        ["openGround"]          - your inventory and the ground (drop things)
        ["open"] / ["onLoad", display] / ["refresh"]
        ["move", side, all]     - side "container" (take) or "player" (store)
        ["moveRow", side, row]  - double-click: all of one row
*/

#define IDD_TRANSFER   5300
#define IDC_TITLE      5301
#define IDC_WEIGHT     5302
#define IDC_C_LABEL    5310
#define IDC_C_LIST     5311
#define IDC_C_QTY      5312
#define IDC_P_LABEL    5320
#define IDC_P_LIST     5321
#define IDC_P_QTY      5322
#define IDC_P_ONE      5323
#define IDC_P_ALL      5324

params ["_mode", ["_a", ""], ["_b", false]];

private _display = uiNamespace getVariable ["ALife_transferDisplay", displayNull];
private _container = missionNamespace getVariable ["ALife_container", ["", "Ground", "ground", [], 0, 0, 1000]];
_container params ["_id", "_title", "_kind", "_items", "_cash", "_weight", "_capacity"];
private _phone = missionNamespace getVariable ["ALife_phone", createHashMap];
private _carry = getNumber (missionConfigFile >> "CfgALife" >> "carryWeight");
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };

// Fill a list: cash first, then items. Row data = class ("cash" for money), value = how many.
private _fnc_fill = {
    params ["_list", "_items", "_cash", "_empty"];
    private _keep = lbSelection _list;
    lbClear _list;
    if (_cash > 0) then {
        private _row = _list lbAdd format ["%1  Cash", _cash call _fnc_money];
        _list lbSetData [_row, "cash"];
        _list lbSetValue [_row, _cash];
        _list lbSetPicture [_row, "ui\icons\bank_ca.paa"];
    };
    {
        _x params ["_class", "_count"];
        if (_count > 0) then {
            private _cfg = missionConfigFile >> "CfgItems" >> _class;
            private _row = _list lbAdd format ["[%1]  %2", _count, getText (_cfg >> "displayName")];
            _list lbSetData [_row, _class];
            _list lbSetValue [_row, _count];
            _list lbSetPicture [_row, getText (_cfg >> "icon")];
        };
    } forEach _items;
    if (lbSize _list == 0) then {
        _list lbAdd _empty;
        _list lbSetColor [0, [0.62, 0.66, 0.72, 1]];
    } else {
        { if (_x < lbSize _list) then { _list lbSetSelected [_x, true]; }; } forEach _keep;
    };
};

switch (_mode) do {
    case "openGround": {
        ALife_container = ["", "Ground", "ground", [], 0, 0, 1000];
        if (isNull (findDisplay IDD_TRANSFER)) then { ["open"] call ALife_fnc_transfer; } else { ["refresh"] call ALife_fnc_transfer; };
    };

    case "open": {
        if (!isNull (findDisplay IDD_TRANSFER)) exitWith { ["refresh"] call ALife_fnc_transfer; };
        createDialog "ALife_Transfer";
    };

    case "onLoad": {
        uiNamespace setVariable ["ALife_transferDisplay", _a];
        ["refresh"] call ALife_fnc_transfer;
        [player] remoteExec ["ALife_fnc_requestPhoneData", 2]; // fresh player side
    };

    case "refresh": {
        if (isNull _display) exitWith {};
        private _ground = _kind == "ground";
        (_display displayCtrl IDC_TITLE) ctrlSetText (if (_ground) then { "Inventory" } else { "Vehicle Trunk - " + _title });
        (_display displayCtrl IDC_WEIGHT) ctrlSetText (if (_ground) then { "" } else { format ["Trunk: %1 / %2", _weight, _capacity] });
        (_display displayCtrl IDC_C_LABEL) ctrlSetText (if (_ground) then { ["Ground", "Dropped items"] select (_id != "") } else { "Trunk" });

        private _myItems = (_phone getOrDefault ["items", []]) select { _x isEqualTypeParams ["", 0] };
        (_display displayCtrl IDC_P_LABEL) ctrlSetText format ["Your inventory  (%1 / %2)", [_myItems] call ALife_fnc_itemsWeight, _carry];

        [_display displayCtrl IDC_C_LIST, _items, _cash, ["Empty", "Nothing here yet -- drop things from your inventory."] select (_ground && { _id == "" })] call _fnc_fill;
        [_display displayCtrl IDC_P_LIST, _myItems, _phone getOrDefault ["cash", 0], "You're not carrying anything."] call _fnc_fill;

        (_display displayCtrl IDC_P_ONE) ctrlSetText (["Store", "Drop"] select _ground);
        (_display displayCtrl IDC_P_ALL) ctrlSetText (["Store all", "Drop all"] select _ground);
    };

    // Take (container side) / Store-Drop (player side): the selected rows,
    // each up to the quantity entered -- or everything with "all".
    case "move": {
        if (isNull _display) exitWith {};
        private _side = _a;
        private _all = _b;
        private _list = _display displayCtrl ([IDC_P_LIST, IDC_C_LIST] select (_side == "container"));
        private _qty = floor parseNumber ctrlText (_display displayCtrl ([IDC_P_QTY, IDC_C_QTY] select (_side == "container")));
        private _rows = lbSelection _list;
        if (_all) then {
            _rows = [];
            for "_i" from 0 to (lbSize _list) - 1 do { _rows pushBack _i; };
        };

        private _entries = [];
        private _money = 0;
        {
            private _data = _list lbData _x;
            private _available = _list lbValue _x;
            if (_data != "") then {
                private _n = [_qty min _available, _available] select _all;
                if (_n > 0) then {
                    if (_data == "cash") then { _money = _n; } else { _entries pushBack [_data, _n]; };
                };
            };
        } forEach _rows;

        if (_entries isEqualTo [] && { _money <= 0 }) exitWith {
            hint (if (_all) then { "There's nothing to move." } else { "Select something first (Ctrl or Shift to pick several), and enter how many." });
        };
        if (_side == "container" && { _id == "" }) exitWith {};
        [player, _id, ["store", "take"] select (_side == "container"), _entries, _money] remoteExecCall ["ALife_fnc_containerTransfer", 2];
    };

    // Double-click: all of that one row.
    case "moveRow": {
        if (isNull _display || { _b < 0 }) exitWith {};
        private _side = _a;
        private _list = _display displayCtrl ([IDC_P_LIST, IDC_C_LIST] select (_side == "container"));
        private _data = _list lbData _b;
        private _available = _list lbValue _b;
        if (_data == "" || { _available <= 0 } || { _side == "container" && { _id == "" } }) exitWith {};
        [player, _id, ["store", "take"] select (_side == "container"),
            [[[_data, _available]], []] select (_data == "cash"), [0, _available] select (_data == "cash")] remoteExecCall ["ALife_fnc_containerTransfer", 2];
    };
};
