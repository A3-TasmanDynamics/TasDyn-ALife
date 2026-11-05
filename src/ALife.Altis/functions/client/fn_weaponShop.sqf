/*
    ALife_fnc_weaponShop (client)

    The weapon shop (ui\weaponShop.hpp) for one CfgShops placement of a
    WeaponShops shop (CfgWeapons.hpp): the General Store, the Gun Store, the
    Rebel Armoury, Police Supplies / Armoury, EMS Equipment.

    The selected item lies on a dark table in 3D filling the screen (a local
    scene high above the player: a camera looking down on it; drag to turn
    it, scroll to zoom). Along the bottom, the shop -- tabs (Weapons,
    Magazines, Attachments, Items), a search box, the items in groups that
    open and close with a click (Tasers, Pistols, SMGs, Rifles...; magazines
    by the weapon they fit; attachments by kind), greyed out with what's
    needed when you can't buy them; "Mags & attachments" lists what fits the
    selected weapon -- and your cart. Pay cash / Pay bank buys the whole cart at once
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
        ["tab", 0-3] / ["list"] / ["select"] (a click) / ["show"] / ["preview"]
        ["for"]     - the selected weapon's magazines and attachments, or back
        ["add"] / ["remove"] / ["clear"] / ["cart"]
        ["checkout", "cash" | "bank"]
        ["bought", [[list, class, qty], ...]]   - (from the server) put it on you
*/

#define IDD 5150
#define IDC_TITLE 5151
#define IDC_BALANCE 5152
#define IDC_TAB0 5160
#define IDC_SEARCH 5164
#define IDC_LIST 5165
#define IDC_INFO 5166
#define IDC_QTY 5168
#define IDC_FOR 5169
#define IDC_CART_TITLE 5170
#define IDC_CART 5171
#define IDC_HINT 5180
// The scene: how far up, the table's colour.
#define STAGE_UP 500
#define TABLE_TEXTURE "#(rgb,8,8,3)color(0.045,0.063,0.11,1)"

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
// The selected catalogue entry: [tab, group, list, class, name, price, need], or [].
private _fnc_selected = {
    private _i = missionNamespace getVariable ["ALife_wsSelected", -1];
    if (_i < 0) exitWith { [] };
    (missionNamespace getVariable ["ALife_wsCatalogue", []]) param [_i, []]
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
            "<t align='center' color='#8b98b3' shadow='1'>Drag to turn  ·  Scroll to zoom  ·  Double-click to add</t>";

        // ---- The catalogue: every row with its tab and group ----------------
        private _rows = getArray (_cfg >> "items");
        private _catalogue = [];
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
        ALife_wsFor = "";
        ALife_wsSelected = -1;
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

        // Drag above the panels to turn the item, scroll to zoom.
        _display displayAddEventHandler ["MouseButtonDown", {
            params ["", "_button", "", "_y"];
            if (_button == 0 && { _y < safezoneY + safezoneH * 0.54 }) then { ALife_wsDrag = true; };
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
            if ((getMousePosition select 1) > safezoneY + safezoneH * 0.54) exitWith {}; // over the panels: the list scrolls
            ALife_wsZoom = ((ALife_wsZoom * (1 - 0.12 * _scroll)) max 0.25) min 3;
            ["zoom"] call ALife_fnc_weaponShop;
        }];
        // Search as you type.
        (_display displayCtrl IDC_SEARCH) ctrlAddEventHandler ["KeyUp", {
            private _text = toLower ctrlText (_this select 0);
            if (_text != ALife_wsSearch) then { ALife_wsSearch = _text; ["list"] call ALife_fnc_weaponShop; };
        }];

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
            "<t align='right'><t color='#8b98b3'>Bank </t><t font='RobotoCondensedBold'>%1</t><t color='#8b98b3'>   Cash </t><t font='RobotoCondensedBold'>%2</t></t>",
            (_data getOrDefault ["bank", 0]) call _fnc_money, (_data getOrDefault ["cash", 0]) call _fnc_money];
    };

    case "tab": {
        if (isNull _display) exitWith {};
        ALife_wsTab = _arg;
        ALife_wsFor = "";
        ALife_wsSelected = -1; // the first item of the tab
        for "_t" from 0 to 3 do {
            private _b = _display displayCtrl (IDC_TAB0 + _t);
            private _on = _t == _arg;
            private _has = (missionNamespace getVariable ["ALife_wsCatalogue", []]) findIf { (_x select 0) == _t } >= 0;
            _b ctrlSetBackgroundColor ([[1, 1, 1, 0.08], [0.96, 0.62, 0.04, 1]] select _on);
            _b ctrlSetTextColor ([[[1, 1, 1, 0.35], [1, 1, 1, 1]] select _has, [0.118, 0.071, 0, 1]] select _on);
        };
        ["list"] call ALife_fnc_weaponShop;
    };

    case "for": {
        // A weapon's magazines and attachments, or back to the tab.
        if (isNull _display) exitWith {};
        if ((missionNamespace getVariable ["ALife_wsFor", ""]) != "") exitWith {
            private _keep = ALife_wsSelected;
            ["tab", ALife_wsTab] call ALife_fnc_weaponShop;
            ALife_wsSelected = _keep;
            ["list"] call ALife_fnc_weaponShop;
            ["show"] call ALife_fnc_weaponShop;
        };
        private _entry = call _fnc_selected;
        if (_entry isEqualTo [] || { (_entry select 2) != "items" } || { !(((_entry select 3) call _fnc_type) in [1, 2, 4]) }) exitWith {
            ["Pick a weapon first.", "warning"] call ALife_fnc_toast;
        };
        ALife_wsFor = _entry select 3;
        ["list"] call ALife_fnc_weaponShop;
        ["show"] call ALife_fnc_weaponShop;
    };

    case "list": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _search = missionNamespace getVariable ["ALife_wsSearch", ""];
        private _tab = missionNamespace getVariable ["ALife_wsTab", 0];
        private _for = missionNamespace getVariable ["ALife_wsFor", ""];
        private _catalogue = missionNamespace getVariable ["ALife_wsCatalogue", []];
        private _open = missionNamespace getVariable ["ALife_wsOpen", createHashMap];
        // What's listed: the tab, or what fits the weapon picked for "for".
        private _shown = [];
        if (_for != "") then {
            private _mags = (compatibleMagazines _for) apply { toLower _x };
            private _accs = (compatibleItems _for) apply { toLower _x };
            {
                if (((_x select 2) == "mags" && { toLower (_x select 3) in _mags }) || { (_x select 2) == "accs" && { toLower (_x select 3) in _accs } }) then { _shown pushBack _forEachIndex; };
            } forEach _catalogue;
        } else {
            { if ((_x select 0) == _tab) then { _shown pushBack _forEachIndex; }; } forEach _catalogue;
        };
        _shown = _shown select { _search == "" || { _search in toLower ((_catalogue select _x) select 4) } };
        private _groupOf = { if (_for != "" && { ((_catalogue select _this) select 2) == "mags" }) then { "Magazines" } else { (_catalogue select _this) select 1 } };
        // Groups in the order they first come in the shop.
        private _groups = [];
        { _groups pushBackUnique (_x call _groupOf); } forEach _shown;

        ALife_wsListing = true; // (selecting rows below isn't a click)
        lnbClear _list;
        if (_for != "") then {
            private _row = _list lnbAddRow [format ["FITS THE %1", toUpper getText (configFile >> "CfgWeapons" >> _for >> "displayName")], ""];
            _list lnbSetValue [[_row, 0], -2];
            _list lnbSetColor [[_row, 0], [1, 1, 1, 1]];
        };
        {
            private _group = _x;
            private _members = _shown select { (_x call _groupOf) == _group };
            // Each group opens and closes with a click; the first one starts
            // open (all of them for a weapon's fits).
            private _key = format ["%1|%2|%3", _tab, _for, _group];
            if (isNil { _open get _key }) then { _open set [_key, _forEachIndex == 0 || { _for != "" }]; };
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
        if (_value < 0) exitWith {};
        ALife_wsSelected = _value;
        ["show"] call ALife_fnc_weaponShop;
    };

    case "show": {
        if (isNull _display) exitWith {};
        private _entry = call _fnc_selected;
        private _forBtn = _display displayCtrl IDC_FOR;
        private _for = missionNamespace getVariable ["ALife_wsFor", ""];
        if (_entry isEqualTo []) exitWith {
            (_display displayCtrl IDC_INFO) ctrlSetStructuredText parseText "";
            deleteVehicle (missionNamespace getVariable ["ALife_wsHolder", objNull]);
            _forBtn ctrlEnable (_for != "");
        };
        _entry params ["", "", "_key", "_class", "_name", "_price", "_need"];
        private _isWeapon = _key == "items" && { (_class call _fnc_type) in [1, 2, 4] };
        _forBtn ctrlSetText (["Mags & attachments", "Back to the list"] select (_for != ""));
        _forBtn ctrlEnable (_isWeapon || { _for != "" });
        // What it is: the magazines a weapon takes (that this shop sells), or what's needed.
        private _extra = "";
        if (_need != "") then {
            _extra = format ["<t color='#fbbf24'>%1 needed</t>", _need call _fnc_safe];
        } else {
            if (_isWeapon) then {
                private _fits = (compatibleMagazines _class) apply { toLower _x };
                private _sold = (missionNamespace getVariable ["ALife_wsCatalogue", []]) select { (_x select 2) == "mags" && { toLower (_x select 3) in _fits } };
                if (_sold isNotEqualTo []) then { _extra = format ["<t color='#8b98b3'>Takes </t>%1", ((_sold apply { _x select 4 }) joinString ", ") call _fnc_safe]; };
            } else {
                _extra = format ["<t color='#8b98b3'>%1</t>", (getText ([_key, _class] call _fnc_root >> "descriptionShort")) call _fnc_safe];
            };
        };
        (_display displayCtrl IDC_INFO) ctrlSetStructuredText parseText format [
            "<t font='RobotoCondensedBold'>%1</t>  <t color='#fbbf24'>%2</t>   %3", _name call _fnc_safe, _price call _fnc_money, _extra];
        ["preview", _entry] call ALife_fnc_weaponShop;
    };

    case "preview": {
        _arg params ["", "", "_key", "_class"];
        deleteVehicle (missionNamespace getVariable ["ALife_wsHolder", objNull]);
        private _holder = createVehicleLocal ["GroundWeaponHolder_Scripted", [0, 0, 0], [], 0, "CAN_COLLIDE"];
        _holder enableSimulation false;
        _holder setPosASL (ALife_wsStage vectorAdd [0, 0, 0.01]);
        _holder setDir ALife_wsAngle;
        switch (true) do {
            case (_key == "mags"): { _holder addMagazineCargo [_class, 1]; };
            case ((_class call _fnc_type) in [1, 2, 4, 4096]): { _holder addWeaponCargo [_class, 1]; };
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
        private _qty = floor parseNumber ctrlText (_display displayCtrl IDC_QTY);
        if (_qty < 1) then { _qty = 1; };
        private _cart = missionNamespace getVariable ["ALife_wsCart", []];
        private _i = _cart findIf { (_x select 0) == _key && { (_x select 1) == _class } };
        if (_i < 0) then {
            if (count _cart >= 15) exitWith { ["Your cart is full: buy what's in it first.", "warning"] call ALife_fnc_toast; };
            _cart pushBack [_key, _class, _qty min 20, _price, _name];
        } else {
            (_cart select _i) set [2, (((_cart select _i) select 2) + _qty) min 20];
        };
        ALife_wsCart = _cart;
        ["cart"] call ALife_fnc_weaponShop;
    };

    case "remove": {
        if (isNull _display) exitWith {};
        private _row = lnbCurSelRow (_display displayCtrl IDC_CART);
        private _cart = missionNamespace getVariable ["ALife_wsCart", []];
        if (_row < 0 || { _row >= count _cart }) exitWith { ["Pick something in your cart.", "warning"] call ALife_fnc_toast; };
        private _line = _cart select _row;
        if ((_line select 2) > 1) then { _line set [2, (_line select 2) - 1]; } else { _cart deleteAt _row; };
        ["cart"] call ALife_fnc_weaponShop;
        private _list = _display displayCtrl IDC_CART;
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
            _x params ["_key", "_class", "_qty", "_price", "_name"];
            private _row = _list lnbAddRow [_name, format ["x%1", _qty], (_price * _qty) call _fnc_money];
            _list lnbSetPicture [[_row, 0], getText ([_key, _class] call _fnc_root >> "picture")];
            _total = _total + _price * _qty;
        } forEach _cart;
        if (_cart isEqualTo []) then {
            _list lnbAddRow ["Empty: double-click an item to add it.", "", ""];
            _list lnbSetColor [[0, 0], [0.545, 0.596, 0.702, 1]];
        };
        (_display displayCtrl IDC_CART_TITLE) ctrlSetStructuredText parseText format [
            "<t font='RobotoCondensedBold'>YOUR CART</t><t align='right' font='RobotoCondensedBold' color='#fbbf24'>%1</t>", _total call _fnc_money];
    };

    case "checkout": {
        if (isNull _display) exitWith {};
        private _cart = missionNamespace getVariable ["ALife_wsCart", []];
        if (_cart isEqualTo []) exitWith { ["Your cart is empty.", "warning"] call ALife_fnc_toast; };
        [player, _place, _cart apply { [_x select 0, _x select 1, _x select 2] }, _arg] remoteExec ["ALife_fnc_shopBuyWeapon", 2];
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
        {
            _x params [["_key", ""], ["_class", ""], ["_qty", 0]];
            for "_n" from 1 to _qty do {
                switch (true) do {
                    case (_key == "mags"): {
                        if (player canAdd _class) then { player addMagazine _class; } else { (call _fnc_ground) addMagazineCargoGlobal [_class, 1]; };
                    };
                    case ((_class call _fnc_type) in [1, 2, 4, 4096]): {
                        private _slot = switch (_class call _fnc_type) do {
                            case 1: { primaryWeapon player };
                            case 2: { handgunWeapon player };
                            case 4: { secondaryWeapon player };
                            default { binocular player };
                        };
                        if (_slot == "") exitWith { player addWeapon _class; };
                        private _i = [player canAddItemToBackpack _class, player canAddItemToVest _class, player canAddItemToUniform _class] find true;
                        if (_i < 0) exitWith { (call _fnc_ground) addWeaponCargoGlobal [_class, 1]; };
                        ([backpackContainer player, vestContainer player, uniformContainer player] select _i) addWeaponCargoGlobal [_class, 1];
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
