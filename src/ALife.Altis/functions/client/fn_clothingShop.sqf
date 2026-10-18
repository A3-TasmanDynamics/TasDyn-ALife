/*
    ALife_fnc_clothingShop (client)

    The clothing shop (ui\clothingShop.hpp) for a CfgShops placement of a
    ClothingShops shop. A camera faces the player. Picking items puts them
    in a basket -- one per slot (uniform, headgear, glasses, vest, backpack)
    -- and on the player, so they can build a whole outfit and see it before
    paying; "Keep what I'm wearing" takes a slot back out. Pay cash / Pay
    bank buys the whole basket in one go (ALife_fnc_shopBuyClothing); when
    the server answers "bought", the outfit is theirs. Closing puts back
    what they own (what they came in wearing plus anything bought).

    Modes:
        ["open", placement]
        ["onLoad", display]
        ["category"]           - show another list (uniforms, headgear, ...)
        ["select"]             - put the selected item in the basket (or take one out)
        ["rotate", degrees]    - turn the camera around the player
        ["buy", "cash"|"bank"]
        ["bought", [[list, class], ...]]  (from the server)
        ["unload"]
*/

#define IDD 5500
#define IDC_TITLE 5501
#define IDC_BALANCE 5502
#define IDC_DESC 5503
#define IDC_CATEGORY 5505
#define IDC_LIST 5510

params ["_mode", ["_arg", ""]];

private _display = findDisplay IDD;
private _place = missionNamespace getVariable ["ALife_clothingPlacement", ""];
private _shop = ([_place] call ALife_fnc_shopInfo) param [2, configNull];
private _data = missionNamespace getVariable ["ALife_phone", createHashMap];
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };
private _lists = [["uniforms", "Uniforms"], ["headgear", "Headgear"], ["goggles", "Glasses"], ["vests", "Vests"], ["backpacks", "Backpacks"]];
// Where each list lives in getUnitLoadout: [index, is a container with contents].
private _slots = createHashMapFromArray [["uniforms", [3, true]], ["vests", [4, true]], ["backpacks", [5, true]], ["headgear", [6, false]], ["goggles", [7, false]]];
private _fnc_missing = {
    private _level = _data getOrDefault [(_data getOrDefault ["faction", "civ"]) + "Level", 0];
    [_this, _data getOrDefault ["licences", []], _level] call ALife_fnc_shopAccess
};
private _fnc_root = {
    // Backpacks are vehicles, glasses are CfgGlasses, the rest are CfgWeapons.
    ["CfgWeapons", "CfgVehicles", "CfgGlasses"] select ((["backpacks", "goggles"] find _this) + 1)
};
private _fnc_row = {
    // The shop's row for [list, class], or [].
    params ["_list", "_class"];
    private _rows = getArray (_shop >> _list);
    private _i = _rows findIf { (_x select 0) == _class };
    if (_i < 0) exitWith { [] };
    _rows select _i
};
private _fnc_name = {
    params ["_list", "_class"];
    private _row = [_list, _class] call _fnc_row;
    private _name = _row param [1, ""];
    if (_name == "") then { _name = getText (configFile >> (_list call _fnc_root) >> _class >> "displayName"); };
    _name
};
// A loadout with [list, class] put on: containers keep what was in them.
private _fnc_wear = {
    params ["_loadout", "_list", "_class"];
    (_slots get _list) params ["_index", "_container"];
    private _current = _loadout select _index;
    _loadout set [_index, if (_container) then { [_class, if (_current isEqualTo []) then { [] } else { _current select 1 }] } else { _class }];
    _loadout
};
// What they own with everything in the basket on top.
private _fnc_preview = {
    private _loadout = +ALife_clothingBase;
    { _loadout = [_loadout, _x, _y] call _fnc_wear; } forEach ALife_clothingCart;
    player setUnitLoadout _loadout;
    [true] call ALife_fnc_applySkins; // the group's texture on what they're trying
};
// Point the camera at the body (or the head, for hats and glasses).
private _fnc_camera = {
    private _cam = missionNamespace getVariable ["ALife_shopCamera", objNull];
    if (isNull _cam) exitWith {};
    private _head = (missionNamespace getVariable ["ALife_clothingList", "uniforms"]) in ["headgear", "goggles"];
    private _angle = missionNamespace getVariable ["ALife_shopAngle", 0];
    private _dist = [2.6, 1.1] select _head;
    private _height = [1.0, 1.62] select _head;
    _cam camSetTarget (player modelToWorld [0, 0, _height]);
    _cam camSetPos (player modelToWorld [_dist * sin _angle, _dist * cos _angle, _height + ([0.15, 0.05] select _head)]);
    _cam camCommit 0.4;
};
// The basket: each item, its price, the total, and anything not allowed.
private _fnc_summary = {
    if (isNull _display) exitWith {};
    private _lines = [];
    private _total = 0;
    private _blocked = [];
    {
        _x params ["_key", "_label"];
        if (_key in ALife_clothingCart) then {
            private _class = ALife_clothingCart get _key;
            private _row = [_key, _class] call _fnc_row;
            private _price = _row param [2, 0];
            _total = _total + _price;
            _lines pushBack format ["<t color='#8b98b3'>%1</t>  %2<t align='right' color='#fbbf24'>%3</t>", _label, [_key, _class] call _fnc_name, _price call _fnc_money];
            private _missing = [_row param [3, ""], _row param [4, 0]] call _fnc_missing;
            if (_missing != "") then { _blocked pushBackUnique _missing; };
        };
    } forEach _lists;
    // Their things move into the new uniform / vest / backpack; one that's
    // smaller still holds them all, it's just full until they make room.
    private _full = [];
    { if ((_x select 0) > 1) then { _full pushBack (_x select 1); }; } forEach [[loadUniform player, "uniform"], [loadVest player, "vest"], [loadBackpack player, "backpack"]];
    (_display displayCtrl IDC_DESC) ctrlSetStructuredText parseText (if (_lines isEqualTo []) then {
        "<t color='#8b98b3'>Pick items to try them on. Everything you pick goes in your basket; pay for it all at the end.</t>"
    } else {
        (_lines joinString "<br/>") + format ["<br/><t font='RobotoCondensedBold' size='1.15'>Total</t><t align='right' font='RobotoCondensedBold' size='1.15' color='#fbbf24'>%1</t>", _total call _fnc_money]
        + (if (_blocked isEqualTo []) then { "" } else { format ["<br/><t color='#fca5a5'>%1 needed.</t>", _blocked joinString ", "] })
        + (if (_full isEqualTo []) then { "" } else { format ["<br/><t color='#fbbf24'>Your things move across. Your new %1 is overfull: it keeps them, but won't take more.</t>", _full joinString " / "] })
    });
};

switch (_mode) do {
    case "open": {
        private _info = [_arg] call ALife_fnc_shopInfo;
        if (dialog || { _info isEqualTo [] } || { (_info select 0) != "clothing" }) exitWith {};
        private _missing = (_info select 2) call _fnc_missing;
        if (_missing != "") exitWith { [format ["%1 needed to use this shop.", _missing], "warning"] call ALife_fnc_toast; };
        ALife_clothingPlacement = _arg;
        ALife_clothingBase = getUnitLoadout player; // what they own: came in wearing + bought here
        ALife_clothingCart = createHashMap;        // list -> class, not paid for yet
        ALife_shopAngle = 0;
        ALife_clothingList = "uniforms";
        createDialog "ALife_ClothingShop";
        [player] remoteExec ["ALife_fnc_requestPhoneData", 2];
    };

    case "onLoad": {
        _display = _arg;
        (_display displayCtrl IDC_TITLE) ctrlSetText toUpper getText (_shop >> "name");
        private _combo = _display displayCtrl IDC_CATEGORY;
        lbClear _combo;
        {
            _x params ["_key", "_label"];
            if (getArray (_shop >> _key) isNotEqualTo []) then {
                private _i = _combo lbAdd _label;
                _combo lbSetData [_i, _key];
            };
        } forEach _lists;

        private _cam = "camera" camCreate (player modelToWorld [0, 3, 1.5]);
        _cam cameraEffect ["internal", "back"];
        _cam camSetFov 0.6;
        showCinemaBorder false;
        ALife_shopCamera = _cam;
        call _fnc_camera;

        if (lbSize _combo > 0) then { _combo lbSetCurSel 0; }; // fills the list ("category")
        call _fnc_summary;
        [] spawn { ["refresh"] call ALife_fnc_clothingShop; };
    };

    case "refresh": {
        if (isNull _display) exitWith {};
        (_display displayCtrl IDC_BALANCE) ctrlSetStructuredText parseText format [
            "<t color='#8b98b3'>Bank </t><t font='RobotoCondensedBold'>%1</t><t color='#8b98b3'>   Cash </t><t font='RobotoCondensedBold'>%2</t>",
            (_data getOrDefault ["bank", 0]) call _fnc_money, (_data getOrDefault ["cash", 0]) call _fnc_money];
    };

    case "category": {
        if (isNull _display) exitWith {};
        private _combo = _display displayCtrl IDC_CATEGORY;
        private _key = _combo lbData (lbCurSel _combo);
        ALife_clothingList = _key;
        private _list = _display displayCtrl IDC_LIST;
        lnbClear _list;
        // Row 0: take this slot out of the basket.
        _list lnbAddRow ["Keep what I'm wearing", ""];
        _list lnbSetData [[0, 0], ""];
        _list lnbSetColor [[0, 0], [0.545, 0.596, 0.702, 1]];
        private _picked = ALife_clothingCart getOrDefault [_key, ""];
        {
            _x params ["_class", "", ["_price", 0], ["_licence", ""], ["_level", 0]];
            private _row = _list lnbAddRow [[_key, _class] call _fnc_name, _price call _fnc_money];
            _list lnbSetData [[_row, 0], _class];
            _list lnbSetPicture [[_row, 0], getText (configFile >> (_key call _fnc_root) >> _class >> "picture")];
            if (([_licence, _level] call _fnc_missing) != "") then { _list lnbSetColor [[_row, 0], [1, 1, 1, 0.4]]; };
            if (_class == _picked) then { _list lnbSetColor [[_row, 0], [0.984, 0.749, 0.141, 1]]; }; // in the basket
        } forEach getArray (_shop >> _key);
        call _fnc_camera;
    };

    case "select": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        if (_row < 0) exitWith {};
        private _key = ALife_clothingList;
        private _class = _list lnbData [_row, 0];
        // The basket holds at most one item per slot; row 0 takes it out.
        if (_class == "") then { ALife_clothingCart deleteAt _key; } else { ALife_clothingCart set [_key, _class]; };
        // Mark the basket item in the list.
        for "_r" from 1 to ((lnbSize _list) select 0) - 1 do {
            private _rowClass = _list lnbData [_r, 0];
            private _rowData = [_key, _rowClass] call _fnc_row;
            private _colour = [[1, 1, 1, 1], [1, 1, 1, 0.4]] select ((([_rowData param [3, ""], _rowData param [4, 0]] call _fnc_missing)) != "");
            if (_rowClass == _class) then { _colour = [0.984, 0.749, 0.141, 1]; };
            _list lnbSetColor [[_r, 0], _colour];
        };
        call _fnc_preview;
        call _fnc_summary;
    };

    case "rotate": {
        ALife_shopAngle = (missionNamespace getVariable ["ALife_shopAngle", 0]) + _arg;
        call _fnc_camera;
    };

    case "buy": {
        if (isNull _display) exitWith {};
        if (count ALife_clothingCart == 0) exitWith { ["Your basket is empty: pick something to try on first.", "warning"] call ALife_fnc_toast; };
        private _items = (keys ALife_clothingCart) apply { [_x, ALife_clothingCart get _x] };
        [player, _place, _items, _arg] remoteExec ["ALife_fnc_shopBuyClothing", 2];
    };

    case "bought": {
        // Paid for: the basket becomes part of what they own.
        {
            _x params ["_key", "_class"];
            ALife_clothingBase = [ALife_clothingBase, _key, _class] call _fnc_wear;
            if ((ALife_clothingCart getOrDefault [_key, ""]) == _class) then { ALife_clothingCart deleteAt _key; };
        } forEach _arg;
        call _fnc_preview;
        call _fnc_summary;
        if (!isNull _display) then { ["category"] call ALife_fnc_clothingShop; };
    };

    case "unload": {
        private _cam = missionNamespace getVariable ["ALife_shopCamera", objNull];
        if (!isNull _cam) then {
            _cam cameraEffect ["terminate", "back"];
            camDestroy _cam;
        };
        ALife_shopCamera = objNull;
        // Anything still in the basket wasn't paid for: back to what they own.
        if (!isNil "ALife_clothingBase") then {
            player setUnitLoadout ALife_clothingBase;
            [true] call ALife_fnc_applySkins;
        };
        ALife_clothingCart = createHashMap;
    };
};
