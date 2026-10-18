/*
    ALife_fnc_clothingShop (client)

    The clothing shop (ui\clothingShop.hpp) for a CfgShops placement of a
    ClothingShops shop. A camera faces the player; picking an item puts it
    on them (on top of what they came in wearing) so they can see it. Pay
    cash / Pay bank asks the server (ALife_fnc_shopBuyClothing); when it
    answers "bought", the item stays on. Closing puts back what they came
    in wearing plus anything bought.

    Modes:
        ["open", placement]
        ["onLoad", display]
        ["category"]           - another list (uniforms, headgear, ...)
        ["select"]             - try on the selected item
        ["rotate", degrees]    - turn the camera around the player
        ["buy", "cash"|"bank"]
        ["bought", [list, class]]  (from the server)
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
private _fnc_itemName = {
    params ["_list", "_class", "_name"];
    if (_name != "") exitWith { _name };
    private _root = ["CfgWeapons", "CfgVehicles", "CfgGlasses"] select ((["backpacks", "goggles"] find _list) + 1);
    getText (configFile >> _root >> _class >> "displayName")
};
private _fnc_picture = {
    params ["_list", "_class"];
    private _root = ["CfgWeapons", "CfgVehicles", "CfgGlasses"] select ((["backpacks", "goggles"] find _list) + 1);
    getText (configFile >> _root >> _class >> "picture")
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

switch (_mode) do {
    case "open": {
        private _info = [_arg] call ALife_fnc_shopInfo;
        if (dialog || { _info isEqualTo [] } || { (_info select 0) != "clothing" }) exitWith {};
        private _missing = (_info select 2) call _fnc_missing;
        if (_missing != "") exitWith { [format ["%1 needed to use this shop.", _missing], "warning"] call ALife_fnc_toast; };
        ALife_clothingPlacement = _arg;
        ALife_clothingBase = getUnitLoadout player; // what they came in wearing (+ what they buy)
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
        {
            _x params ["_class", ["_name", ""], ["_price", 0], ["_licence", ""], ["_level", 0]];
            private _row = _list lnbAddRow [[_key, _class, _name] call _fnc_itemName, _price call _fnc_money];
            _list lnbSetData [[_row, 0], _class];
            _list lnbSetPicture [[_row, 0], [_key, _class] call _fnc_picture];
            if (([_licence, _level] call _fnc_missing) != "") then { _list lnbSetColor [[_row, 0], [1, 1, 1, 0.4]]; };
        } forEach getArray (_shop >> _key);
        // Back to what they came in wearing, then frame the right part of them.
        player setUnitLoadout ALife_clothingBase;
        call _fnc_camera;
        (_display displayCtrl IDC_DESC) ctrlSetStructuredText parseText "";
    };

    case "select": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        if (_row < 0) exitWith {};
        private _key = ALife_clothingList;
        private _entry = getArray (_shop >> _key) select (_row min (count getArray (_shop >> _key) - 1));
        _entry params ["_class", ["_name", ""], ["_price", 0], ["_licence", ""], ["_level", 0]];

        // Try it on: what they came in wearing, with this one slot swapped.
        // Containers keep what was in them.
        (_slots get _key) params ["_index", "_container"];
        private _loadout = +ALife_clothingBase;
        private _current = _loadout select _index;
        _loadout set [_index, if (_container) then { [_class, if (_current isEqualTo []) then { [] } else { _current select 1 }] } else { _class }];
        player setUnitLoadout _loadout;

        private _missing = [_licence, _level] call _fnc_missing;
        (_display displayCtrl IDC_DESC) ctrlSetStructuredText parseText format [
            "<t size='1.1' font='RobotoCondensedBold'>%1</t>  <t color='#fbbf24'>%2</t>%3",
            [_key, _class, _name] call _fnc_itemName, _price call _fnc_money,
            ["", format ["<br/><t color='#fbbf24'>%1 needed.</t>", _missing]] select (_missing != "")];
    };

    case "rotate": {
        ALife_shopAngle = (missionNamespace getVariable ["ALife_shopAngle", 0]) + _arg;
        call _fnc_camera;
    };

    case "buy": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        if (_row < 0) exitWith { ["Pick something to try on first.", "warning"] call ALife_fnc_toast; };
        [player, _place, ALife_clothingList, _list lnbData [_row, 0], _arg] remoteExec ["ALife_fnc_shopBuyClothing", 2];
    };

    case "bought": {
        // Keep it: it becomes part of what they're wearing when they leave.
        _arg params ["_key", "_class"];
        (_slots get _key) params ["_index", "_container"];
        private _current = ALife_clothingBase select _index;
        ALife_clothingBase set [_index, if (_container) then { [_class, if (_current isEqualTo []) then { [] } else { _current select 1 }] } else { _class }];
    };

    case "unload": {
        private _cam = missionNamespace getVariable ["ALife_shopCamera", objNull];
        if (!isNull _cam) then {
            _cam cameraEffect ["terminate", "back"];
            camDestroy _cam;
        };
        ALife_shopCamera = objNull;
        if (!isNil "ALife_clothingBase") then { player setUnitLoadout ALife_clothingBase; };
    };
};
