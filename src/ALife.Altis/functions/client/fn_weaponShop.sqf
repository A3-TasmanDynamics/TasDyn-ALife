/*
    ALife_fnc_weaponShop (client)

    The weapon shop (ui\weaponShop.hpp) for one CfgShops placement of a
    WeaponShops shop (CfgWeapons.hpp): the General Store, the Gun Store, the
    Rebel Armoury, the Police Armoury, EMS Equipment.

    The middle of the screen is the selected item lying on a dark table in
    3D (a local scene high above the player, a camera looking down on it;
    drag to turn it, scroll to zoom). On the left, the shop: tabs (Weapons,
    Magazines, Attachments, Items), a search box, and the items in groups
    that open and close with a click (Tasers, Pistols, SMGs, Rifles...;
    magazines by the weapon they fit; attachments by kind), greyed out with
    what's needed when you can't buy them. On the right, the kit and your
    cart.

    The kit: a weapon picks its muzzle, optic, light / laser and bipod (only
    what fits it and this shop sells) and its magazines -- the preview shows
    it built -- and goes in the cart as one line. Bought, it comes with
    everything attached and the first magazine loaded. Anything else goes
    in with a quantity. Pay cash / Pay bank buys the whole cart at once
    (ALife_fnc_shopBuyWeapon checks everything again, including that it fits
    on you); the goods are then put on you here ("bought").

    Where they go: a weapon into its empty slot (rifle, pistol, launcher,
    binoculars), otherwise your backpack, vest or uniform; a map, compass,
    watch, radio, GPS or night vision into its empty slot, otherwise your
    inventory; the rest into your inventory. Anything that doesn't fit after
    all is put on the ground at your feet.

    Modes:
        ["open", placement]
        ["onLoad", display] / ["unload"] / ["refresh"]
        ["tab", 0-3] / ["list"] / ["select"] (a click) / ["show"] / ["preview"] / ["zoom"]
        ["kitChanged"]      - a kit dropdown / the magazine count changed
        ["add"] / ["remove"] / ["clear"] / ["cart"]
        ["checkout", "cash" | "bank"]
        ["bought", [[list, class, qty, kit], ...]]   - (from the server) put it on you

    A cart line: [list, class, qty, price each, name, kit]; kit is
    [muzzle, pointer, optic, bipod, magazine, magazines] for a weapon kit,
    [] otherwise.
*/

#define IDD 5150
#define IDC_TITLE 5151
#define IDC_BALANCE 5152
#define IDC_TAB0 5160
#define IDC_SEARCH 5164
#define IDC_LIST 5165
#define IDC_INFO 5166
#define IDC_ADD 5167
#define IDC_QTY 5168
#define IDC_QTY_LABEL 5169
#define IDC_CART_TITLE 5170
#define IDC_CART 5171
#define IDC_HINT 5180
#define IDC_KIT_TITLE 5190
#define IDC_SLOT0 5191
#define IDC_SLOT_LABEL0 5181
#define IDC_MAG 5195
#define IDC_MAG_COUNT 5196
#define IDC_MAG_LABEL 5186
#define IDC_KIT_TOTAL 5197
// The scene: how far up, the table's colour.
#define STAGE_UP 500
#define TABLE_TEXTURE "#(rgb,8,8,3)color(0.045,0.063,0.11,1)"
// The kit's attachment slots: compatibleItems slot, label; in weaponsItems order.
#define KIT_SLOTS [["MuzzleSlot", "Muzzle"], ["PointerSlot", "Light / laser"], ["CowsSlot", "Optic"], ["UnderBarrelSlot", "Bipod"]]

params ["_mode", ["_arg", ""]];

private _display = findDisplay IDD;
private _place = missionNamespace getVariable ["ALife_weaponShop", ""];
private _cfg = ([_place] call ALife_fnc_shopInfo) param [2, configNull]; // the WeaponShops shop
private _data = missionNamespace getVariable ["ALife_phone", createHashMap];
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };
private _fnc_safe = { (_this splitString "<>&") joinString "" };
// What this player lacks for a shop config or a row's [licence, level] ("" = nothing).
private _fnc_missing = {
    private _level = _data getOrDefault [(_data getOrDefault ["faction", "civ"]) + "Level", 0];
    [_this, _data getOrDefault ["licences", []], _level] call ALife_fnc_shopAccess
};
// A row's class in Arma's config (magazines are CfgMagazines).
private _fnc_root = { configFile >> (["CfgWeapons", "CfgMagazines"] select ((_this select 0) == "mags")) >> (_this select 1) };
// Weapons with a slot of their own: rifle 1, pistol 2, launcher 4, binoculars 4096.
private _fnc_type = { getNumber (configFile >> "CfgWeapons" >> _this >> "type") };
private _fnc_isGun = { (_this call _fnc_type) in [1, 2, 4] };
private _catalogue = missionNamespace getVariable ["ALife_wsCatalogue", []];
// The selected catalogue entry: [tab, group, list, class, name, price, need], or [].
private _fnc_selected = {
    private _i = missionNamespace getVariable ["ALife_wsSelected", -1];
    if (_i < 0) exitWith { [] };
    _catalogue param [_i, []]
};
// A catalogue entry by list and class, or [].
private _fnc_entry = {
    params ["_key", "_class"];
    private _i = _catalogue findIf { (_x select 2) == _key && { (_x select 3) == _class } };
    if (_i < 0) exitWith { [] };
    _catalogue select _i
};
// The current kit's price: the weapon, its attachments and magazines.
private _fnc_kitPrice = {
    params ["_weapon", "_kit"];
    _kit params [["_muzzle", ""], ["_pointer", ""], ["_optic", ""], ["_bipod", ""], ["_mag", ""], ["_mags", 0]];
    private _total = (["items", _weapon] call _fnc_entry) param [5, 0];
    { if (_x != "") then { _total = _total + ((["accs", _x] call _fnc_entry) param [5, 0]); }; } forEach [_muzzle, _pointer, _optic, _bipod];
    if (_mag != "") then { _total = _total + ((["mags", _mag] call _fnc_entry) param [5, 0]) * _mags; };
    _total
};
private _fnc_camera = {
    private _cam = missionNamespace getVariable ["ALife_wsCamera", objNull];
    if (isNull _cam) exitWith {};
    private _zoom = missionNamespace getVariable ["ALife_wsZoom", 1];
    _cam camSetTarget (ASLToAGL ALife_wsStage);
    _cam camSetPos (ASLToAGL (ALife_wsStage vectorAdd [0, -0.22 * _zoom, _zoom]));
    _cam camCommit 0.15;
};

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
        (_display displayCtrl IDC_HINT) ctrlSetStructuredText parseText
            "<t align='center' color='#8b98b3' shadow='1'>Drag to turn  ·  Scroll to zoom  ·  Double-click an item to add it</t>";

        // ---- The catalogue: every row with its tab and group ----------------
        private _rows = getArray (_cfg >> "items");
        _catalogue = [];
        private _weapons = []; // [name, compatible magazines] for grouping magazines
        {
            _x params [["_class", ""], ["_name", ""], ["_price", 0], ["", 0], ["_licence", ""], ["_level", 0]];
            private _type = _class call _fnc_type;
            if (_name == "") then { _name = getText (configFile >> "CfgWeapons" >> _class >> "displayName"); };
            private _need = [_licence, _level] call _fnc_missing;
            if (_type in [1, 2, 4]) then {
                private _group = switch (true) do {
                    case ("taser" in toLower _name): { "Tasers" };
                    case (_type == 2): { "Pistols" };
                    case (_type == 4): { "Launchers" };
                    case (_class isKindOf ["Rifle_Short_Base_F", configFile >> "CfgWeapons"]): { "SMGs" };
                    case (_class isKindOf ["Rifle_Long_Base_F", configFile >> "CfgWeapons"]): { "Marksman & MGs" };
                    default { "Rifles" };
                };
                _catalogue pushBack [0, _group, "items", _class, _name, _price, _need];
                _weapons pushBack [_name, (compatibleMagazines _class) apply { toLower _x }];
            } else {
                _catalogue pushBack [3, "Equipment", "items", _class, _name, _price, _need];
            };
        } forEach _rows;
        {
            _x params [["_class", ""], ["_name", ""], ["_price", 0], ["", 0], ["_licence", ""], ["_level", 0]];
            if (_name == "") then { _name = getText (configFile >> "CfgMagazines" >> _class >> "displayName"); };
            private _i = _weapons findIf { toLower _class in (_x select 1) };
            _catalogue pushBack [1, ["Other", "For " + ((_weapons select _i) select 0)] select (_i >= 0), "mags", _class, _name, _price, [_licence, _level] call _fnc_missing];
        } forEach getArray (_cfg >> "mags");
        {
            _x params [["_class", ""], ["_name", ""], ["_price", 0], ["", 0], ["_licence", ""], ["_level", 0]];
            if (_name == "") then { _name = getText (configFile >> "CfgWeapons" >> _class >> "displayName"); };
            private _group = switch (getNumber (configFile >> "CfgWeapons" >> _class >> "ItemInfo" >> "type")) do {
                case 201: { "Optics" };
                case 101: { "Muzzle" };
                case 301: { "Lights & lasers" };
                case 302: { "Bipods" };
                default { "Other" };
            };
            _catalogue pushBack [2, _group, "accs", _class, _name, _price, [_licence, _level] call _fnc_missing];
        } forEach getArray (_cfg >> "accs");
        ALife_wsCatalogue = _catalogue;
        ALife_wsCart = [];
        ALife_wsSearch = "";
        ALife_wsSelected = -1;
        ALife_wsKit = [];
        ALife_wsOpen = createHashMap; // which groups are open

        // ---- The scene: a dark table under a camera looking down ------------
        ALife_wsStage = (getPosASL player) vectorAdd [0, 0, STAGE_UP];
        ALife_wsAngle = 90;
        ALife_wsZoom = 1;
        private _table = [];
        // A flat panel; two, facing up and down, so one always shows.
        {
            private _p = createVehicleLocal ["UserTexture10m_F", [0, 0, 0], [], 0, "CAN_COLLIDE"];
            _p setObjectTexture [0, TABLE_TEXTURE];
            _p setPosASL ALife_wsStage;
            _p setVectorDirAndUp [[0, 0, _x], [0, 1, 0]];
            // Centre it under the stage, whatever its model's origin.
            private _off = (_p modelToWorldWorld (boundingCenter _p)) vectorDiff ALife_wsStage;
            _p setPosASL (ALife_wsStage vectorDiff [_off select 0, _off select 1, (_off select 2) + 0.01 + 0.01 * _forEachIndex]);
            _table pushBack _p;
        } forEach [1, -1];
        // Lit by the sun by day; at night a soft light from above (off by day).
        private _light = "#lightpoint" createVehicleLocal (ASLToAGL (ALife_wsStage vectorAdd [0, -1, 4]));
        _light setLightBrightness 0.12;
        _light setLightColor [0.9, 0.92, 1];
        _light setLightAmbient [0.35, 0.36, 0.4];
        _light setLightDayLight false;
        ALife_wsScene = _table + [_light];
        private _cam = "camera" camCreate (ASLToAGL (ALife_wsStage vectorAdd [0, 0, 1]));
        _cam cameraEffect ["internal", "back"];
        _cam camSetFov 0.55;
        showCinemaBorder false;
        ALife_wsCamera = _cam;
        call _fnc_camera;

        // Drag between the panels to turn the item, scroll there to zoom.
        _display displayAddEventHandler ["MouseButtonDown", {
            params ["", "_button", "_x", "_y"];
            if (_button == 0 && { _x > safezoneX + safezoneW * 0.28 } && { _x < safezoneX + safezoneW * 0.72 } && { _y < safezoneY + safezoneH * 0.88 }) then { ALife_wsDrag = true; };
        }];
        _display displayAddEventHandler ["MouseButtonUp", { ALife_wsDrag = false; }];
        _display displayAddEventHandler ["MouseMoving", {
            params ["", "_dx"];
            if !(missionNamespace getVariable ["ALife_wsDrag", false]) exitWith {};
            ALife_wsAngle = ALife_wsAngle + _dx * 120;
            private _holder = missionNamespace getVariable ["ALife_wsHolder", objNull];
            if (!isNull _holder) then { _holder setDir ALife_wsAngle; };
        }];
        _display displayAddEventHandler ["MouseZChanged", {
            params ["", "_scroll"];
            getMousePosition params ["_x", "_y"];
            if !(_x > safezoneX + safezoneW * 0.28 && { _x < safezoneX + safezoneW * 0.72 } && { _y < safezoneY + safezoneH * 0.88 }) exitWith {}; // over a panel: it scrolls
            ALife_wsZoom = ((ALife_wsZoom * (1 - 0.12 * _scroll)) max 0.25) min 3;
            ["zoom"] call ALife_fnc_weaponShop;
        }];
        // Search as you type; the magazine count as you type.
        (_display displayCtrl IDC_SEARCH) ctrlAddEventHandler ["KeyUp", {
            private _text = toLower ctrlText (_this select 0);
            if (_text != ALife_wsSearch) then { ALife_wsSearch = _text; ["list"] call ALife_fnc_weaponShop; };
        }];
        (_display displayCtrl IDC_MAG_COUNT) ctrlAddEventHandler ["KeyUp", { ["kitChanged"] call ALife_fnc_weaponShop; }];
        for "_i" from 0 to 3 do {
            (_display displayCtrl (IDC_SLOT0 + _i)) ctrlAddEventHandler ["LBSelChanged", { ["kitChanged"] call ALife_fnc_weaponShop; }];
        };
        (_display displayCtrl IDC_MAG) ctrlAddEventHandler ["LBSelChanged", { ["kitChanged"] call ALife_fnc_weaponShop; }];

        // Open on the first tab with anything in it.
        private _first = [0, 1, 2, 3] findIf { private _t = _x; _catalogue findIf { (_x select 0) == _t } >= 0 };
        ["tab", _first max 0] call ALife_fnc_weaponShop;
        ["cart"] call ALife_fnc_weaponShop;
        [] spawn { ["refresh"] call ALife_fnc_weaponShop; }; // after the dialog has finished opening
    };

    case "unload": {
        private _cam = missionNamespace getVariable ["ALife_wsCamera", objNull];
        if (!isNull _cam) then {
            _cam cameraEffect ["terminate", "back"];
            camDestroy _cam;
        };
        ALife_wsCamera = objNull;
        { deleteVehicle _x; } forEach ((missionNamespace getVariable ["ALife_wsScene", []]) + [missionNamespace getVariable ["ALife_wsHolder", objNull]]);
        ALife_wsScene = [];
        ALife_wsHolder = objNull;
        ALife_wsDrag = false;
    };

    case "refresh": {
        if (isNull _display) exitWith {};
        (_display displayCtrl IDC_BALANCE) ctrlSetStructuredText parseText format [
            "<t color='#8b98b3'>Bank </t><t font='RobotoCondensedBold'>%1</t><t color='#8b98b3'>   Cash </t><t font='RobotoCondensedBold'>%2</t>",
            (_data getOrDefault ["bank", 0]) call _fnc_money, (_data getOrDefault ["cash", 0]) call _fnc_money];
    };

    case "tab": {
        if (isNull _display) exitWith {};
        ALife_wsTab = _arg;
        ALife_wsSelected = -1; // the first item of the tab
        for "_t" from 0 to 3 do {
            private _b = _display displayCtrl (IDC_TAB0 + _t);
            private _on = _t == _arg;
            private _has = _catalogue findIf { (_x select 0) == _t } >= 0;
            _b ctrlSetBackgroundColor ([[1, 1, 1, 0.08], [0.96, 0.62, 0.04, 1]] select _on);
            _b ctrlSetTextColor ([[[1, 1, 1, 0.35], [1, 1, 1, 1]] select _has, [0.118, 0.071, 0, 1]] select _on);
        };
        ["list"] call ALife_fnc_weaponShop;
    };

    case "list": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _search = missionNamespace getVariable ["ALife_wsSearch", ""];
        private _tab = missionNamespace getVariable ["ALife_wsTab", 0];
        private _open = missionNamespace getVariable ["ALife_wsOpen", createHashMap];
        private _shown = [];
        { if ((_x select 0) == _tab && { _search == "" || { _search in toLower (_x select 4) } }) then { _shown pushBack _forEachIndex; }; } forEach _catalogue;
        // Groups in the order they first come in the shop.
        private _groups = [];
        { _groups pushBackUnique ((_catalogue select _x) select 1); } forEach _shown;

        ALife_wsListing = true; // (selecting rows below isn't a click)
        lnbClear _list;
        {
            private _group = _x;
            private _members = _shown select { ((_catalogue select _x) select 1) == _group };
            // Each group opens and closes with a click; the first one starts open.
            private _key = format ["%1|%2", _tab, _group];
            if (isNil { _open get _key }) then { _open set [_key, _forEachIndex == 0]; };
            private _isOpen = (_open get _key) || { _search != "" };
            private _head = _list lnbAddRow [format ["%1   %2  (%3)", ["+", "-"] select _isOpen, toUpper _group, count _members], ""];
            _list lnbSetValue [[_head, 0], -1];
            _list lnbSetData [[_head, 0], _key];
            _list lnbSetColor [[_head, 0], [0.984, 0.749, 0.141, 1]];
            if (_isOpen) then {
                {
                    (_catalogue select _x) params ["", "", "_key", "_class", "_name", "_price", "_need"];
                    private _row = _list lnbAddRow ["      " + _name, _price call _fnc_money];
                    _list lnbSetValue [[_row, 0], _x];
                    _list lnbSetPicture [[_row, 0], getText ([_key, _class] call _fnc_root >> "picture")];
                    if (_need != "") then { _list lnbSetColor [[_row, 0], [1, 1, 1, 0.4]]; _list lnbSetColor [[_row, 1], [1, 1, 1, 0.4]]; };
                } forEach _members;
            };
        } forEach _groups;
        ALife_wsOpen = _open;
        if (_shown isEqualTo []) then {
            private _row = _list lnbAddRow [["Nothing here.", "Nothing matches your search."] select (_search != ""), ""];
            _list lnbSetValue [[_row, 0], -2];
            _list lnbSetColor [[_row, 0], [0.545, 0.596, 0.702, 1]];
        };
        // Keep the selected item highlighted; on a new tab, the first item.
        private _rows = (lnbSize _list) select 0;
        private _row = -1;
        for "_r" from 0 to _rows - 1 do { if ((_list lnbValue [_r, 0]) == (missionNamespace getVariable ["ALife_wsSelected", -1])) exitWith { _row = _r; }; };
        private _changed = false;
        if (_row < 0 && { (missionNamespace getVariable ["ALife_wsSelected", -1]) < 0 }) then {
            for "_r" from 0 to _rows - 1 do { if ((_list lnbValue [_r, 0]) >= 0) exitWith { _row = _r; }; };
            if (_row >= 0) then { ALife_wsSelected = _list lnbValue [_row, 0]; _changed = true; };
        };
        _list lnbSetCurSelRow _row;
        ALife_wsListing = false;
        if (_changed) then { ["show"] call ALife_fnc_weaponShop; };
    };

    case "select": {
        // A click in the list: a group header opens / closes it, an item shows it.
        if (isNull _display || { missionNamespace getVariable ["ALife_wsListing", false] }) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        if (_row < 0) exitWith {};
        private _value = _list lnbValue [_row, 0];
        if (_value == -1) exitWith {
            private _key = _list lnbData [_row, 0];
            private _open = missionNamespace getVariable ["ALife_wsOpen", createHashMap];
            _open set [_key, !(_open getOrDefault [_key, false])];
            ["list"] call ALife_fnc_weaponShop;
        };
        if (_value < 0 || { _value == (missionNamespace getVariable ["ALife_wsSelected", -1]) }) exitWith {};
        ALife_wsSelected = _value;
        ["show"] call ALife_fnc_weaponShop;
    };

    case "show": {
        // The selected item: its info, and on the right its kit (a weapon) or a quantity.
        if (isNull _display) exitWith {};
        private _entry = call _fnc_selected;
        private _slotCtrls = [0, 1, 2, 3] apply { _display displayCtrl (IDC_SLOT0 + _x) };
        private _slotLabels = [0, 1, 2, 3] apply { _display displayCtrl (IDC_SLOT_LABEL0 + _x) };
        private _kitCtrls = _slotCtrls + _slotLabels + [_display displayCtrl IDC_MAG, _display displayCtrl IDC_MAG_COUNT, _display displayCtrl IDC_MAG_LABEL];
        private _qtyCtrls = [_display displayCtrl IDC_QTY, _display displayCtrl IDC_QTY_LABEL];
        if (_entry isEqualTo []) exitWith {
            { _x ctrlShow false; } forEach (_kitCtrls + _qtyCtrls);
            (_display displayCtrl IDC_INFO) ctrlSetStructuredText parseText "";
            (_display displayCtrl IDC_KIT_TITLE) ctrlSetStructuredText parseText "<t color='#8b98b3'>Pick something on the left.</t>";
            (_display displayCtrl IDC_KIT_TOTAL) ctrlSetStructuredText parseText "";
            deleteVehicle (missionNamespace getVariable ["ALife_wsHolder", objNull]);
        };
        _entry params ["", "", "_key", "_class", "_name", "_price", "_need"];
        private _gun = _key == "items" && { _class call _fnc_isGun };
        { _x ctrlShow _gun; } forEach _kitCtrls;
        { _x ctrlShow !_gun; } forEach _qtyCtrls;
        (_display displayCtrl IDC_ADD) ctrlSetText (["Add to cart", "Add kit to cart"] select _gun);

        // Info under the list: what it is, or what's needed.
        private _info = if (_need != "") then { format ["<t color='#fbbf24'>%1 needed.</t>", _need call _fnc_safe] } else {
            format ["<t color='#8b98b3'>%1</t>", (getText ([_key, _class] call _fnc_root >> "descriptionShort")) call _fnc_safe]
        };
        (_display displayCtrl IDC_INFO) ctrlSetStructuredText parseText _info;

        if (_gun) then {
            // The kit: each slot lists what fits this weapon and the shop sells.
            ALife_wsKit = ["", "", "", "", "", 0];
            {
                _x params ["_slot", "_label"];
                private _combo = _slotCtrls select _forEachIndex;
                (_slotLabels select _forEachIndex) ctrlSetText _label;
                lbClear _combo;
                _combo lbSetData [_combo lbAdd "None", ""];
                private _fits = (compatibleItems [_class, _slot]) apply { toLower _x };
                {
                    _x params ["", "", "_k", "_c", "_n", "_p", "_need"];
                    if (_k == "accs" && { toLower _c in _fits }) then {
                        private _i = _combo lbAdd format ["%1  +%2%3", _n, _p call _fnc_money, ["", format ["  (%1)", _need]] select (_need != "")];
                        _combo lbSetData [_i, _c];
                        _combo lbSetPicture [_i, getText (configFile >> "CfgWeapons" >> _c >> "picture")];
                        if (_need != "") then { _combo lbSetColor [_i, [1, 1, 1, 0.4]]; };
                    };
                } forEach _catalogue;
                _combo ctrlEnable (lbSize _combo > 1);
                _combo lbSetCurSel 0;
            } forEach KIT_SLOTS;
            private _magCombo = _display displayCtrl IDC_MAG;
            lbClear _magCombo;
            _magCombo lbSetData [_magCombo lbAdd "No magazines", ""];
            private _fits = (compatibleMagazines _class) apply { toLower _x };
            {
                _x params ["", "", "_k", "_c", "_n", "_p", "_need"];
                if (_k == "mags" && { toLower _c in _fits }) then {
                    private _i = _magCombo lbAdd format ["%1  %2 each%3", _n, _p call _fnc_money, ["", format ["  (%1)", _need]] select (_need != "")];
                    _magCombo lbSetData [_i, _c];
                    if (_need != "") then { _magCombo lbSetColor [_i, [1, 1, 1, 0.4]]; };
                };
            } forEach _catalogue;
            // Two of the first magazine by default.
            _magCombo lbSetCurSel ([0, 1] select (lbSize _magCombo > 1));
            (_display displayCtrl IDC_MAG_COUNT) ctrlSetText (["0", "2"] select (lbSize _magCombo > 1));
            ["kitChanged"] call ALife_fnc_weaponShop;
        } else {
            (_display displayCtrl IDC_QTY) ctrlSetText "1";
            (_display displayCtrl IDC_KIT_TITLE) ctrlSetStructuredText parseText format [
                "<t size='1.2' font='RobotoCondensedBold'>%1</t><br/><t color='#fbbf24'>%2</t>", _name call _fnc_safe, _price call _fnc_money];
            (_display displayCtrl IDC_KIT_TOTAL) ctrlSetStructuredText parseText "";
            ["preview", [_key, _class, []]] call ALife_fnc_weaponShop;
        };
    };

    case "kitChanged": {
        // A kit dropdown or the magazine count: the kit, its price and the preview.
        if (isNull _display) exitWith {};
        private _entry = call _fnc_selected;
        if (_entry isEqualTo [] || { !((_entry select 3) call _fnc_isGun) }) exitWith {};
        private _weapon = _entry select 3;
        private _pick = { private _c = _display displayCtrl _this; if (lbCurSel _c < 0) then { "" } else { _c lbData (lbCurSel _c) } };
        private _mag = IDC_MAG call _pick;
        private _mags = if (_mag == "") then { 0 } else { ((floor parseNumber ctrlText (_display displayCtrl IDC_MAG_COUNT)) max 0) min 10 };
        ALife_wsKit = [(IDC_SLOT0) call _pick, (IDC_SLOT0 + 1) call _pick, (IDC_SLOT0 + 2) call _pick, (IDC_SLOT0 + 3) call _pick, _mag, _mags];
        private _total = [_weapon, ALife_wsKit] call _fnc_kitPrice;
        private _parts = count ((ALife_wsKit select [0, 4]) select { _x != "" });
        (_display displayCtrl IDC_KIT_TITLE) ctrlSetStructuredText parseText format [
            "<t size='1.2' font='RobotoCondensedBold'>%1</t><br/><t color='#8b98b3'>%2</t>", (_entry select 4) call _fnc_safe,
            format ["Weapon %1%2", (_entry select 5) call _fnc_money, ["", "  ·  attachments and magazines are added to it"] select (_parts > 0 || { _mags > 0 })]];
        (_display displayCtrl IDC_KIT_TOTAL) ctrlSetStructuredText parseText format [
            "<t color='#8b98b3'>Kit total</t><t align='right' size='1.15' font='RobotoCondensedBold' color='#fbbf24'>%1</t>", _total call _fnc_money];
        ["preview", ["items", _weapon, ALife_wsKit]] call ALife_fnc_weaponShop;
    };

    case "preview": {
        // [list, class, kit]: the item on the table (a weapon with its kit on).
        _arg params ["_key", "_class", ["_kit", []]];
        deleteVehicle (missionNamespace getVariable ["ALife_wsHolder", objNull]);
        private _holder = createVehicleLocal ["GroundWeaponHolder_Scripted", [0, 0, 0], [], 0, "CAN_COLLIDE"];
        _holder enableSimulation false;
        _holder setPosASL (ALife_wsStage vectorAdd [0, 0, 0.01]);
        _holder setDir ALife_wsAngle;
        switch (true) do {
            case (_key == "mags"): { _holder addMagazineCargo [_class, 1]; };
            case (_class call _fnc_isGun): {
                _kit params [["_muzzle", ""], ["_pointer", ""], ["_optic", ""], ["_bipod", ""], ["_mag", ""], ["_mags", 0]];
                private _loaded = if (_mag != "" && { _mags > 0 }) then { [_mag, getNumber (configFile >> "CfgMagazines" >> _mag >> "count")] } else { [] };
                _holder addWeaponWithAttachmentsCargo [[_class, _muzzle, _pointer, _optic, _loaded, [], _bipod], 1];
            };
            case ((_class call _fnc_type) == 4096): { _holder addWeaponCargo [_class, 1]; };
            default { _holder addItemCargo [_class, 1]; };
        };
        ALife_wsHolder = _holder;
        // Framed by its size: long guns further away than pistols and kit.
        ALife_wsZoom = switch (true) do {
            case (_key != "items"): { 0.45 };
            case ((_class call _fnc_type) == 4): { 2 };
            case ((_class call _fnc_type) == 2): { 0.6 };
            case ((_class call _fnc_type) == 1): { [1.45, 1] select (_class isKindOf ["Rifle_Short_Base_F", configFile >> "CfgWeapons"]) };
            default { 0.5 };
        };
        call _fnc_camera;
    };

    case "zoom": {
        call _fnc_camera;
    };

    case "add": {
        if (isNull _display) exitWith {};
        // A double-click on a group header only opens / closes it.
        if (_arg isEqualTo "list" && { ((_display displayCtrl IDC_LIST) lnbValue [lnbCurSelRow (_display displayCtrl IDC_LIST), 0]) < 0 }) exitWith {};
        private _entry = call _fnc_selected;
        if (_entry isEqualTo []) exitWith { ["Pick something first.", "warning"] call ALife_fnc_toast; };
        _entry params ["", "", "_key", "_class", "_name", "_price", "_need"];
        if (_need != "") exitWith { [format ["%1 needed for %2.", _need, _name], "warning"] call ALife_fnc_toast; };
        private _cart = missionNamespace getVariable ["ALife_wsCart", []];
        private _kit = [];
        private _qty = 1;
        if (_key == "items" && { _class call _fnc_isGun }) then {
            _kit = +(missionNamespace getVariable ["ALife_wsKit", []]);
            // Every part of the kit must be allowed too.
            private _blocked = "";
            {
                private _e = [["accs", "accs", "accs", "accs", "mags"] select _forEachIndex, _x] call _fnc_entry;
                if (_x != "" && { _blocked == "" } && { (_e param [6, ""]) != "" }) then { _blocked = format ["%1 needed for %2.", _e select 6, _e select 4]; };
            } forEach (_kit select [0, 5]);
            if (_blocked != "") exitWith { [_blocked, "warning"] call ALife_fnc_toast; _kit = nil; };
            _price = [_class, _kit] call _fnc_kitPrice;
            private _parts = count ((_kit select [0, 4]) select { _x != "" });
            if (_parts > 0 || { (_kit param [5, 0]) > 0 }) then {
                _name = format ["%1 kit", _name];
            };
        } else {
            _qty = ((floor parseNumber ctrlText (_display displayCtrl IDC_QTY)) max 1) min 20;
        };
        if (isNil "_kit") exitWith {};
        private _i = _cart findIf { (_x select 0) == _key && { (_x select 1) == _class } && { (_x select 5) isEqualTo _kit } };
        if (_i < 0) then {
            if (count _cart >= 15) exitWith { ["Your cart is full: buy what's in it first.", "warning"] call ALife_fnc_toast; };
            _cart pushBack [_key, _class, _qty, _price, _name, _kit];
        } else {
            (_cart select _i) set [2, (((_cart select _i) select 2) + _qty) min 20];
        };
        ALife_wsCart = _cart;
        ["cart"] call ALife_fnc_weaponShop;
    };

    case "remove": {
        if (isNull _display) exitWith {};
        // (Rows carry their cart line: a kit has a second row of what's on it.)
        private _list = _display displayCtrl IDC_CART;
        private _row = lnbCurSelRow _list;
        private _cart = missionNamespace getVariable ["ALife_wsCart", []];
        private _i = if (_row < 0) then { -1 } else { _list lnbValue [_row, 0] };
        if (_i < 0 || { _i >= count _cart }) exitWith { ["Pick something in your cart.", "warning"] call ALife_fnc_toast; };
        private _line = _cart select _i;
        if ((_line select 2) > 1) then { _line set [2, (_line select 2) - 1]; } else { _cart deleteAt _i; };
        ["cart"] call ALife_fnc_weaponShop;
        if ((lnbSize _list) select 0 > 0) then { _list lnbSetCurSelRow (_row min (((lnbSize _list) select 0) - 1)); };
    };

    case "clear": {
        ALife_wsCart = [];
        ["cart"] call ALife_fnc_weaponShop;
    };

    case "cart": {
        if (isNull _display) exitWith {};
        private _cart = missionNamespace getVariable ["ALife_wsCart", []];
        private _list = _display displayCtrl IDC_CART;
        lnbClear _list;
        private _total = 0;
        {
            _x params ["_key", "_class", "_qty", "_price", "_name", "_kit"];
            private _line = _forEachIndex;
            private _row = _list lnbAddRow [_name, format ["x%1", _qty], (_price * _qty) call _fnc_money];
            _list lnbSetValue [[_row, 0], _line];
            _list lnbSetPicture [[_row, 0], getText ([_key, _class] call _fnc_root >> "picture")];
            // A kit: what's on it, as a tooltip-style second line.
            if (_kit isNotEqualTo []) then {
                private _parts = ((_kit select [0, 4]) select { _x != "" }) apply { getText (configFile >> "CfgWeapons" >> _x >> "displayName") };
                if ((_kit select 5) > 0) then { _parts pushBack format ["%1x %2", _kit select 5, getText (configFile >> "CfgMagazines" >> (_kit select 4) >> "displayName")]; };
                if (_parts isNotEqualTo []) then {
                    private _sub = _list lnbAddRow ["      + " + (_parts joinString ", "), "", ""];
                    _list lnbSetValue [[_sub, 0], _line];
                    _list lnbSetColor [[_sub, 0], [0.545, 0.596, 0.702, 1]];
                };
            };
            _total = _total + _price * _qty;
        } forEach _cart;
        if (_cart isEqualTo []) then {
            _list lnbAddRow ["Empty: pick something and add it.", "", ""];
            _list lnbSetValue [[0, 0], -1];
            _list lnbSetColor [[0, 0], [0.545, 0.596, 0.702, 1]];
        };
        (_display displayCtrl IDC_CART_TITLE) ctrlSetStructuredText parseText format [
            "<t font='RobotoCondensedBold'>YOUR CART</t><t align='right' font='RobotoCondensedBold' color='#fbbf24'>%1</t>", _total call _fnc_money];
    };

    case "checkout": {
        if (isNull _display) exitWith {};
        private _cart = missionNamespace getVariable ["ALife_wsCart", []];
        if (_cart isEqualTo []) exitWith { ["Your cart is empty.", "warning"] call ALife_fnc_toast; };
        [player, _place, _cart apply { [_x select 0, _x select 1, _x select 2, _x select 5] }, _arg] remoteExec ["ALife_fnc_shopBuyWeapon", 2];
    };

    case "bought": {
        // Paid: put it all on, the rest at your feet.
        private _ground = objNull;
        private _fnc_ground = {
            if (isNull _ground) then {
                _ground = createVehicle ["GroundWeaponHolder", getPosATL player, [], 0, "CAN_COLLIDE"];
            };
            _ground
        };
        private _fnc_mag = { if (player canAdd _this) then { player addMagazine _this; } else { (call _fnc_ground) addMagazineCargoGlobal [_this, 1]; }; };
        {
            _x params [["_key", ""], ["_class", ""], ["_qty", 0], ["_kit", []]];
            for "_n" from 1 to _qty do {
                switch (true) do {
                    case (_key == "mags"): { _class call _fnc_mag; };
                    case (_class call _fnc_isGun): {
                        _kit params [["_muzzle", ""], ["_pointer", ""], ["_optic", ""], ["_bipod", ""], ["_mag", ""], ["_mags", 0]];
                        private _type = _class call _fnc_type;
                        private _slot = [primaryWeapon player, handgunWeapon player, secondaryWeapon player] select ([1, 2, 4] find _type);
                        if (_slot == "") then {
                            // Into its slot: the first magazine goes in with it (added first,
                            // Arma loads it), then the attachments, then the other magazines.
                            if (_mags > 0) then { player addMagazine _mag; };
                            player addWeapon _class;
                            {
                                if (_x != "") then {
                                    switch (_type) do {
                                        case 1: { player addPrimaryWeaponItem _x; };
                                        case 2: { player addHandgunItem _x; };
                                        default { player addSecondaryWeaponItem _x; };
                                    };
                                };
                            } forEach [_muzzle, _pointer, _optic, _bipod];
                            for "_m" from 2 to _mags do { _mag call _fnc_mag; };
                        } else {
                            // Slot taken: built, into the first container it fits.
                            private _loaded = if (_mags > 0) then { [_mag, getNumber (configFile >> "CfgMagazines" >> _mag >> "count")] } else { [] };
                            private _built = [[_class, _muzzle, _pointer, _optic, _loaded, [], _bipod], 1];
                            private _i = [player canAddItemToBackpack _class, player canAddItemToVest _class, player canAddItemToUniform _class] find true;
                            private _into = if (_i < 0) then { call _fnc_ground } else { [backpackContainer player, vestContainer player, uniformContainer player] select _i };
                            _into addWeaponWithAttachmentsCargoGlobal _built;
                            for "_m" from 2 to _mags do { _mag call _fnc_mag; };
                        };
                    };
                    case ((_class call _fnc_type) == 4096): {
                        if (binocular player == "") then { player addWeapon _class; } else {
                            if (player canAdd _class) then { player addItem _class; } else { (call _fnc_ground) addWeaponCargoGlobal [_class, 1]; };
                        };
                    };
                    default {
                        // Map, compass, watch, radio, GPS, night vision: its slot if that's empty.
                        private _sim = getText (configFile >> "CfgWeapons" >> _class >> "simulation");
                        if (_sim in ["ItemMap", "ItemCompass", "ItemWatch", "ItemRadio", "ItemGPS", "NVGoggles"]
                            && { (assignedItems player) findIf { getText (configFile >> "CfgWeapons" >> _x >> "simulation") == _sim } < 0 }) exitWith { player linkItem _class; };
                        if (player canAdd _class) then { player addItem _class; } else { (call _fnc_ground) addItemCargoGlobal [_class, 1]; };
                    };
                };
            };
        } forEach _arg;
        if (!isNull _ground) then { ["Some of it didn't fit on you: it's on the ground at your feet.", "warning"] call ALife_fnc_toast; };
        ALife_wsCart = [];
        ["cart"] call ALife_fnc_weaponShop;
    };
};
