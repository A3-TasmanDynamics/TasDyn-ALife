/*
    File: fn_spawnMenu.sqf
    Author: Tasman Dynamics

    Description:
        Everything about the spawn dialog lives in this one file, dispatched
        by mode -- opening it (the server->client entry point from
        initPlayerServer.sqf), its onLoad/onUnload handlers, and every
        control action (side button, spawn list, Spawn button). These were
        five separate one-function files; combined here since they're all
        tightly coupled to the same dialog and its two pieces of selection
        state (never meaningfully called independently of each other or of
        that dialog being open).

        Selection state lives on the display itself (setVariable), not a
        mission-namespace global -- it only matters while this dialog is
        open, and disappears with it.

    Parameter(s):
        0: STRING - mode:
           "open"           -- create the dialog (remoteExec'd to a specific
                                client from initPlayerServer.sqf)
           "onLoad"         -- dialog onLoad: default to civ, center the map
           "selectSide"     -- side button action, 1: STRING side ("civ"/"cop"/"medic")
           "selectLocation" -- spawn list onLBSelChanged, 1: CONTROL, 2: SCALAR index
           "spawn"          -- Spawn button action: send the request to the server
           "onUnload"       -- dialog onUnload: clean up the map marker

    Returns:
        Nothing
*/

params ["_mode"];
private _args = _this select [1, (count _this) - 1];

switch (_mode) do {
    case "open": {
        closeDialog 0;
        createDialog "ALife_SpawnMenu";
    };

    case "onLoad": {
        disableSerialization;
        private _display = findDisplay 4700;
        if (isNull _display) exitWith {};

        ["selectSide", "civ"] call ALife_fnc_spawnMenu;

        private _mapCtrl = _display displayCtrl 4730;
        _mapCtrl ctrlMapAnimAdd [0, 0.15, [14000, 15000, 0]];
        ctrlMapAnimCommit _mapCtrl;
    };

    case "selectSide": {
        _args params [["_side", "civ", [""]]];

        disableSerialization;
        private _display = findDisplay 4700;
        if (isNull _display) exitWith {};

        _display setVariable ["alife_spawn_side", _side];
        _display setVariable ["alife_spawn_point", []];

        private _btnCiv = _display displayCtrl 4710;
        private _btnCop = _display displayCtrl 4711;
        private _btnMedic = _display displayCtrl 4712;

        _btnCiv ctrlSetBackgroundColor [0.2, 0.5, 0.2, 1];
        _btnCop ctrlSetBackgroundColor [0.2, 0.2, 0.6, 1];
        _btnMedic ctrlSetBackgroundColor [0.6, 0.2, 0.2, 1];

        switch (_side) do {
            case "civ": { _btnCiv ctrlSetBackgroundColor [0.3, 0.75, 0.3, 1]; };
            case "cop": { _btnCop ctrlSetBackgroundColor [0.3, 0.3, 0.85, 1]; };
            case "medic": { _btnMedic ctrlSetBackgroundColor [0.85, 0.3, 0.3, 1]; };
        };

        private _listCtrl = _display displayCtrl 4720;
        lbClear _listCtrl;

        {
            _x params ["_key", "_displayName"];
            private _index = _listCtrl lbAdd _displayName;
            _listCtrl lbSetData [_index, _key];
        } forEach ([_side] call ALife_fnc_getSpawnPoints);

        {
            deleteMarkerLocal _x;
        } forEach (allMapMarkers select { _x find "alife_spawn_marker" == 0 });

        private _infoCtrl = _display displayCtrl 4740;
        _infoCtrl ctrlSetStructuredText parseText "Select a spawn point from the list.";
    };

    case "selectLocation": {
        _args params ["_ctrl", "_index"];

        disableSerialization;
        private _display = findDisplay 4700;
        if (isNull _display) exitWith {};

        if (_index == -1) exitWith {};

        private _key = _ctrl lbData _index;
        private _side = _display getVariable ["alife_spawn_side", "civ"];

        private _match = ([_side] call ALife_fnc_getSpawnPoints) select { (_x select 0) == _key };
        if (_match isEqualTo []) exitWith {
            _display setVariable ["alife_spawn_point", []];
        };

        (_match select 0) params ["_pointKey", "_displayName", "_position"];
        _display setVariable ["alife_spawn_point", [_pointKey]];

        {
            deleteMarkerLocal _x;
        } forEach (allMapMarkers select { _x find "alife_spawn_marker" == 0 });

        private _marker = createMarkerLocal ["alife_spawn_marker_0", _position];
        _marker setMarkerTypeLocal "mil_objective";
        _marker setMarkerColorLocal "ColorGreen";
        _marker setMarkerTextLocal _displayName;
        _marker setMarkerSizeLocal [0.8, 0.8];

        private _mapCtrl = _display displayCtrl 4730;
        _mapCtrl ctrlMapAnimAdd [0.3, 0.05, _position];
        ctrlMapAnimCommit _mapCtrl;

        private _infoCtrl = _display displayCtrl 4740;
        _infoCtrl ctrlSetStructuredText parseText format ["<t color='#55ff55'>%1</t><br/>Click Spawn to deploy here.", _displayName];
    };

    case "spawn": {
        disableSerialization;
        private _display = findDisplay 4700;
        if (isNull _display) exitWith {};

        private _side = _display getVariable ["alife_spawn_side", ""];
        private _point = _display getVariable ["alife_spawn_point", []];

        if (_side == "" || { _point isEqualTo [] }) exitWith {
            hint "Select a faction and a spawn point first.";
        };

        private _spawnKey = _point select 0;

        closeDialog 0;

        [player, getPlayerUID player, _side, _spawnKey] remoteExec ["ALife_fnc_spawnPlayer", 2];
    };

    case "onUnload": {
        {
            deleteMarkerLocal _x;
        } forEach (allMapMarkers select { _x find "alife_spawn_marker" == 0 });
    };
};
