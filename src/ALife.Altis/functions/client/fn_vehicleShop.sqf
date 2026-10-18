/*
    ALife_fnc_vehicleShop (client)

    The vehicle shop (ui\clothingShop.hpp, ALife_VehicleShop) for a CfgShops
    placement of a CarShops shop. The selected vehicle is shown in 3D: a
    local copy (nobody else sees it) floating high above the player, slowly
    turning, in the chosen skin: the vehicle's CfgVehicles textures[] for
    the player's side if it has any (e.g. the police livery), otherwise its
    own Arma colours (TextureSources).
    Pay cash / Pay bank asks the server (ALife_fnc_shopBuyVehicle), which
    spawns the real one at the shop's spawn point.

    Modes:
        ["open", placement]
        ["onLoad", display]
        ["select"]             - show the selected vehicle
        ["colour"]             - repaint the preview
        ["rotate", degrees]
        ["buy", "cash"|"bank"]
        ["bought", [name, plate]]  (from the server)
        ["unload"]
*/

#define IDD 5600
#define IDC_TITLE 5601
#define IDC_BALANCE 5602
#define IDC_DESC 5603
#define IDC_COLOUR 5605
#define IDC_LIST 5610

params ["_mode", ["_arg", ""]];

private _display = findDisplay IDD;
private _place = missionNamespace getVariable ["ALife_vehiclePlacement", ""];
private _shop = ([_place] call ALife_fnc_shopInfo) param [2, configNull];
private _data = missionNamespace getVariable ["ALife_phone", createHashMap];
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };
private _fnc_missing = {
    private _level = _data getOrDefault [(_data getOrDefault ["faction", "civ"]) + "Level", 0];
    [_this, _data getOrDefault ["licences", []], _level] call ALife_fnc_shopAccess
};
private _fnc_selected = {
    // The selected row of the shop's vehicles[], or [].
    private _rows = getArray (_shop >> "vehicles");
    private _row = lnbCurSelRow (_display displayCtrl IDC_LIST);
    if (_row < 0 || { _row >= count _rows }) exitWith { [] };
    _rows select _row
};

switch (_mode) do {
    case "open": {
        private _info = [_arg] call ALife_fnc_shopInfo;
        if (dialog || { _info isEqualTo [] } || { (_info select 0) != "vehicles" }) exitWith {};
        private _missing = (_info select 2) call _fnc_missing;
        if (_missing != "") exitWith { [format ["%1 needed to use this shop.", _missing], "warning"] call ALife_fnc_toast; };
        ALife_vehiclePlacement = _arg;
        ALife_shopAngle = 0;
        // The showroom: high above the player, out of everyone's way.
        ALife_showroom = (getPosASL player) vectorAdd [0, 0, 600];
        createDialog "ALife_VehicleShop";
        [player] remoteExec ["ALife_fnc_requestPhoneData", 2];
    };

    case "onLoad": {
        _display = _arg;
        (_display displayCtrl IDC_TITLE) ctrlSetText toUpper getText (_shop >> "name");
        private _list = _display displayCtrl IDC_LIST;
        lnbClear _list;
        {
            _x params ["_class", ["_name", ""], ["_price", 0], "", ["_licence", ""], ["_level", 0]];
            if (_name == "") then { _name = getText (configFile >> "CfgVehicles" >> _class >> "displayName"); };
            private _row = _list lnbAddRow [_name, _price call _fnc_money];
            _list lnbSetData [[_row, 0], _class];
            _list lnbSetPicture [[_row, 0], getText (configFile >> "CfgVehicles" >> _class >> "picture")];
            if (([_licence, _level] call _fnc_missing) != "") then { _list lnbSetColor [[_row, 0], [1, 1, 1, 0.4]]; };
        } forEach getArray (_shop >> "vehicles");

        private _cam = "camera" camCreate (ASLToAGL (ALife_showroom vectorAdd [0, 10, 3]));
        _cam cameraEffect ["internal", "back"];
        _cam camSetFov 0.7;
        showCinemaBorder false;
        ALife_shopCamera = _cam;

        // Turn the preview slowly while the shop is open.
        [] spawn {
            while { !isNull findDisplay IDD } do {
                private _veh = missionNamespace getVariable ["ALife_showroomVehicle", objNull];
                if (!isNull _veh) then { _veh setDir (getDir _veh + 0.4); };
                sleep 0.02;
            };
        };

        if ((lnbSize _list) select 0 > 0) then { _list lnbSetCurSelRow 0; }; // shows it ("select")
        [] spawn { ["refresh"] call ALife_fnc_vehicleShop; };
    };

    case "refresh": {
        if (isNull _display) exitWith {};
        (_display displayCtrl IDC_BALANCE) ctrlSetStructuredText parseText format [
            "<t color='#8b98b3'>Bank </t><t font='RobotoCondensedBold'>%1</t><t color='#8b98b3'>   Cash </t><t font='RobotoCondensedBold'>%2</t>",
            (_data getOrDefault ["bank", 0]) call _fnc_money, (_data getOrDefault ["cash", 0]) call _fnc_money];
    };

    case "select": {
        if (isNull _display) exitWith {};
        private _entry = call _fnc_selected;
        if (_entry isEqualTo []) exitWith {};
        _entry params ["_class", ["_name", ""], ["_price", 0], "", ["_licence", ""], ["_level", 0]];
        if (_name == "") then { _name = getText (configFile >> "CfgVehicles" >> _class >> "displayName"); };

        // A fresh local preview, frozen in the air.
        deleteVehicle (missionNamespace getVariable ["ALife_showroomVehicle", objNull]);
        private _veh = createVehicleLocal [_class, [0, 0, 0], [], 0, "CAN_COLLIDE"];
        _veh enableSimulation false;
        _veh allowDamage false;
        _veh setPosASL ALife_showroom;
        _veh setDir 140;
        ALife_showroomVehicle = _veh;

        // Skins: ours for this side (CfgVehicles textures[]) if there are
        // any, otherwise the vehicle's own Arma colours. Data "tex:<row>" /
        // "src:<TextureSources class>" / "" (as it comes).
        private _combo = _display displayCtrl IDC_COLOUR;
        lbClear _combo;
        private _side = _data getOrDefault ["faction", "civ"];
        {
            if ((_x select 1) in ["", _side]) then { _combo lbSetData [_combo lbAdd (_x select 0), format ["tex:%1", _forEachIndex]]; };
        } forEach getArray (missionConfigFile >> "CfgVehicles" >> _class >> "textures");
        if (lbSize _combo == 0) then {
            private _sources = "true" configClasses (configFile >> "CfgVehicles" >> _class >> "TextureSources");
            { _combo lbSetData [_combo lbAdd getText (_x >> "displayName"), "src:" + configName _x]; } forEach _sources;
        };
        if (lbSize _combo == 0) then { _combo lbSetData [_combo lbAdd "Standard", ""]; };
        _combo lbSetCurSel 0; // paints it ("colour")

        // Frame it by its size.
        private _size = ((boundingBoxReal _veh) select 2) max 2;
        ALife_showroomDistance = _size * 1.9;
        ["rotate", 0] call ALife_fnc_vehicleShop;

        // What it is.
        private _trunk = getNumber (missionConfigFile >> "CfgVehicles" >> _class >> "trunk");
        private _seats = count fullCrew [_veh, "", true];
        private _speed = getNumber (configFile >> "CfgVehicles" >> _class >> "maxSpeed");
        private _missing = [_licence, _level] call _fnc_missing;
        (_display displayCtrl IDC_DESC) ctrlSetStructuredText parseText format [
            "<t size='1.15' font='RobotoCondensedBold'>%1</t>  <t color='#fbbf24'>%2</t><br/>" +
            "<t color='#8b98b3'>Seats </t>%3<t color='#8b98b3'>    Top speed </t>%4 km/h<t color='#8b98b3'>    Trunk </t>%5%6",
            _name, _price call _fnc_money, _seats, _speed, _trunk,
            ["", format ["<br/><t color='#fbbf24'>%1 needed.</t>", _missing]] select (_missing != "")];
    };

    case "colour": {
        if (isNull _display) exitWith {};
        private _veh = missionNamespace getVariable ["ALife_showroomVehicle", objNull];
        if (isNull _veh) exitWith {};
        private _combo = _display displayCtrl IDC_COLOUR;
        private _choice = _combo lbData (lbCurSel _combo);
        private _textures = [typeOf _veh, _choice] call ALife_fnc_vehicleSkin;
        { _veh setObjectTexture [_forEachIndex, _x]; } forEach _textures;
    };

    case "rotate": {
        // Orbit the camera; the vehicle also turns on its own.
        ALife_shopAngle = (missionNamespace getVariable ["ALife_shopAngle", 0]) + _arg;
        private _cam = missionNamespace getVariable ["ALife_shopCamera", objNull];
        if (isNull _cam) exitWith {};
        private _dist = missionNamespace getVariable ["ALife_showroomDistance", 8];
        private _a = ALife_shopAngle;
        _cam camSetTarget (ASLToAGL (ALife_showroom vectorAdd [0, 0, 1]));
        _cam camSetPos (ASLToAGL (ALife_showroom vectorAdd [_dist * sin _a, _dist * cos _a, _dist * 0.35]));
        _cam camCommit 0.5;
    };

    case "buy": {
        if (isNull _display) exitWith {};
        private _entry = call _fnc_selected;
        if (_entry isEqualTo []) exitWith { ["Pick a vehicle first.", "warning"] call ALife_fnc_toast; };
        private _combo = _display displayCtrl IDC_COLOUR;
        [player, _place, _entry select 0, _combo lbData (lbCurSel _combo), _arg] remoteExec ["ALife_fnc_shopBuyVehicle", 2];
    };

    case "bought": {
        closeDialog 0;
    };

    case "unload": {
        private _cam = missionNamespace getVariable ["ALife_shopCamera", objNull];
        if (!isNull _cam) then {
            _cam cameraEffect ["terminate", "back"];
            camDestroy _cam;
        };
        ALife_shopCamera = objNull;
        deleteVehicle (missionNamespace getVariable ["ALife_showroomVehicle", objNull]);
        ALife_showroomVehicle = objNull;
    };
};
