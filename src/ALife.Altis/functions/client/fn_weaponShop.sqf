/*
    ALife_fnc_weaponShop (client)

    A weapon shop window (ALife_WeaponShop in ui\shop.hpp) for one CfgShops
    placement of a WeaponShops shop (CfgWeapons.hpp): the General Store, the
    Gun Store, the Rebel Armoury, Police Supplies / Armoury, EMS Equipment.
    Its rows in four sections -- Weapons, Magazines, Attachments, Items --
    with their price; rows needing a licence / rank you don't have are
    greyed out. Pay cash / Pay bank ask the server (ALife_fnc_shopBuyWeapon),
    which checks everything again and has the goods put on you here
    ("bought").

    Where they go: a weapon into its empty slot (rifle, pistol, launcher,
    binoculars), otherwise your backpack, vest or uniform; a map, compass,
    watch, radio, GPS or night vision into its empty slot, otherwise your
    inventory; magazines and attachments into your inventory.

    Modes:
        ["open", placement]
        ["onLoad", display] / ["refresh"] / ["select"]
        ["buy", "cash" | "bank"]
        ["bought", [list, class, qty]]   - (from the server) put it on you
*/

#define IDD 5150
#define IDC_TITLE 5101
#define IDC_BALANCE 5102
#define IDC_DESC 5103
#define IDC_LIST 5110
#define IDC_QTY 5120

params ["_mode", ["_arg", ""]];

private _display = findDisplay IDD;
private _place = missionNamespace getVariable ["ALife_weaponShop", ""];
private _cfg = ([_place] call ALife_fnc_shopInfo) param [2, configNull]; // the WeaponShops shop
private _data = missionNamespace getVariable ["ALife_phone", createHashMap];
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };
// What this player lacks for a shop config or a row's [licence, level] ("" = nothing).
private _fnc_missing = {
    private _level = _data getOrDefault [(_data getOrDefault ["faction", "civ"]) + "Level", 0];
    [_this, _data getOrDefault ["licences", []], _level] call ALife_fnc_shopAccess
};
// A row's class in Arma's config (magazines are CfgMagazines).
private _fnc_root = { configFile >> (["CfgWeapons", "CfgMagazines"] select (_this select 0 == "mags")) >> (_this select 1) };
// Weapons in a slot of their own: rifle 1, pistol 2, launcher 4, binoculars 4096.
private _fnc_isWeapon = { (getNumber (configFile >> "CfgWeapons" >> _this >> "type")) in [1, 2, 4, 4096] };

switch (_mode) do {
    case "open": {
        private _info = [_arg] call ALife_fnc_shopInfo;
        if (!isNull _display || { _info isEqualTo [] } || { (_info select 0) != "weapons" }) exitWith {};
        private _missing = (_info select 2) call _fnc_missing;
        if (_missing != "") exitWith { [format ["%1 needed to use this shop.", _missing], "warning"] call ALife_fnc_toast; };
        ALife_weaponShop = _arg;
        if (dialog) then { closeDialog 0; };
        createDialog "ALife_WeaponShop";
        [player] remoteExec ["ALife_fnc_requestPhoneData", 2];
    };

    case "onLoad": {
        _display = _arg;
        (_display displayCtrl IDC_TITLE) ctrlSetText toUpper getText (_cfg >> "name");
        private _list = _display displayCtrl IDC_LIST;
        lnbClear _list;
        // Sections: items[] holds both weapons and kit.
        private _rows = getArray (_cfg >> "items");
        {
            _x params ["_title", "_key", "_entries"];
            if (_entries isNotEqualTo []) then {
                private _head = _list lnbAddRow [toUpper _title, "", ""];
                _list lnbSetColor [[_head, 0], [0.984, 0.749, 0.141, 1]];
                {
                    _x params [["_class", ""], ["_name", ""], ["_price", 0], ["", 0], ["_licence", ""], ["_level", 0]];
                    private _root = [_key, _class] call _fnc_root;
                    if (_name == "") then { _name = getText (_root >> "displayName"); };
                    private _need = [_licence, _level] call _fnc_missing;
                    private _row = _list lnbAddRow ["   " + _name, _price call _fnc_money, _need];
                    _list lnbSetData [[_row, 0], _class];
                    _list lnbSetData [[_row, 1], _key];
                    _list lnbSetPicture [[_row, 0], getText (_root >> "picture")];
                    if (_need != "") then {
                        _list lnbSetColor [[_row, 0], [1, 1, 1, 0.4]];
                        _list lnbSetColor [[_row, 2], [0.984, 0.749, 0.141, 0.8]];
                    };
                } forEach _entries;
            };
        } forEach [
            ["Weapons", "items", _rows select { ((_x param [0, ""]) call _fnc_isWeapon) && { getNumber (configFile >> "CfgWeapons" >> (_x select 0) >> "type") != 4096 } }],
            ["Magazines", "mags", getArray (_cfg >> "mags")],
            ["Attachments", "accs", getArray (_cfg >> "accs")],
            ["Items", "items", _rows select { !((_x param [0, ""]) call _fnc_isWeapon) || { getNumber (configFile >> "CfgWeapons" >> (_x select 0) >> "type") == 4096 } }]
        ];
        // The first thing you can buy.
        for "_r" from 0 to ((lnbSize _list) select 0) - 1 do {
            if ((_list lnbData [_r, 0]) != "") exitWith { _list lnbSetCurSelRow _r; };
        };
        [] spawn { ["refresh"] call ALife_fnc_weaponShop; }; // after the dialog has finished opening
    };

    case "refresh": {
        if (isNull _display) exitWith {};
        (_display displayCtrl IDC_BALANCE) ctrlSetStructuredText parseText format [
            "<t align='right'><t color='#8b98b3'>Bank </t><t font='RobotoCondensedBold'>%1</t><t color='#8b98b3'>   Cash </t><t font='RobotoCondensedBold'>%2</t></t>",
            (_data getOrDefault ["bank", 0]) call _fnc_money, (_data getOrDefault ["cash", 0]) call _fnc_money];
        ["select"] call ALife_fnc_weaponShop;
    };

    case "select": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        if (_row < 0) exitWith {};
        private _class = _list lnbData [_row, 0];
        private _key = _list lnbData [_row, 1];
        if (_class == "") exitWith { (_display displayCtrl IDC_DESC) ctrlSetStructuredText parseText ""; };
        private _root = [_key, _class] call _fnc_root;
        private _text = getText (_root >> "descriptionShort");
        // A weapon: the magazines this shop sells for it.
        if (_key == "items" && { _class call _fnc_isWeapon }) then {
            private _fits = (compatibleMagazines _class) apply { toLower _x };
            private _sold = (getArray (_cfg >> "mags")) select { toLower (_x select 0) in _fits };
            if (_sold isNotEqualTo []) then {
                _text = format ["Takes: %1", (_sold apply { [(_x select 1), getText (configFile >> "CfgMagazines" >> (_x select 0) >> "displayName")] select ((_x select 1) == "") }) joinString ", "];
            };
        };
        (_display displayCtrl IDC_DESC) ctrlSetStructuredText parseText format ["<t size='0.95' color='#8b98b3'>%1</t>%2",
            (_text splitString "<>") joinString "",
            ["", format ["<br/><t size='0.95' color='#fbbf24'>%1 needed.</t>", _list lnbText [_row, 2]]] select ((_list lnbText [_row, 2]) != "")];
    };

    case "buy": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        private _class = if (_row < 0) then { "" } else { _list lnbData [_row, 0] };
        if (_class == "") exitWith { ["Pick something first.", "warning"] call ALife_fnc_toast; };
        private _key = _list lnbData [_row, 1];
        private _qty = floor parseNumber ctrlText (_display displayCtrl IDC_QTY);
        if (_key == "items" && { _class call _fnc_isWeapon }) then { _qty = 1; };
        if (_qty < 1 || { _qty > 20 }) exitWith { ["Buy between 1 and 20 at a time.", "warning"] call ALife_fnc_toast; };
        [player, _place, _key, _class, _qty, _arg] remoteExec ["ALife_fnc_shopBuyWeapon", 2];
    };

    case "bought": {
        _arg params [["_key", ""], ["_class", ""], ["_qty", 0]];
        if (_class == "" || { _qty < 1 }) exitWith {};
        if (_key == "mags") exitWith { player addMagazines [_class, _qty]; };
        if (_key == "items" && { _class call _fnc_isWeapon }) exitWith {
            private _slot = switch (getNumber (configFile >> "CfgWeapons" >> _class >> "type")) do {
                case 1: { primaryWeapon player };
                case 2: { handgunWeapon player };
                case 4: { secondaryWeapon player };
                default { binocular player };
            };
            if (_slot == "") exitWith { player addWeapon _class; };
            // Slot taken: into the first container it fits.
            private _i = [player canAddItemToBackpack _class, player canAddItemToVest _class, player canAddItemToUniform _class] find true;
            if (_i < 0) exitWith { player addWeapon _class; }; // (the server checked there's room)
            ([backpackContainer player, vestContainer player, uniformContainer player] select _i) addWeaponCargoGlobal [_class, 1];
        };
        // Map, compass, watch, radio, GPS, night vision: its slot if that's empty.
        private _sim = getText (configFile >> "CfgWeapons" >> _class >> "simulation");
        for "_n" from 1 to _qty do {
            if (_sim in ["ItemMap", "ItemCompass", "ItemWatch", "ItemRadio", "ItemGPS", "NVGoggles"]
                && { (assignedItems player) findIf { getText (configFile >> "CfgWeapons" >> _x >> "simulation") == _sim } < 0 }) then {
                player linkItem _class;
            } else {
                player addItem _class;
            };
        };
    };
};
