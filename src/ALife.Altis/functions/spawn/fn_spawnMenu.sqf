/*
    File: fn_spawnMenu.sqf
    Author: Tasman Dynamics

    Description:
        Everything about the spawn dialog lives in this one file, dispatched
        by mode -- opening it (the server->client entry point from
        fn_playerJoin.sqf), its onLoad/onUnload handlers, and every
        control action (spawn list, Spawn button). These were originally
        several separate one-function files; combined here since they're all
        tightly coupled to the same dialog and its selection state (never
        meaningfully called independently of each other or of that dialog
        being open).

        Faction is NOT chosen here -- it's derived from the player's actual
        Arma side (`side player`, assigned by which Editor-placed playable
        slot they connected as) via ALife_fnc_sideToFaction, matching
        Tonic's AsYetUntitled/Framework rather than a custom side-picker.
        See docs/TONIC_REFERENCE.md §3. This dialog only ever picks WHERE to
        spawn within that side.

        Selection state lives on the display itself (setVariable), not a
        mission-namespace global -- it only matters while this dialog is
        open, and disappears with it.

    Parameter(s):
        0: STRING - mode:
           "open"           -- create the dialog (remoteExec'd to a specific
                                client from fn_playerJoin.sqf), clearing
                                initPlayerLocal.sqf's welcome screen first
           "onLoad"         -- dialog onLoad: resolve faction from side
                                player, populate the spawn list, center map
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
        cutText ["", "BLACK OUT"];
        closeDialog 0;
        createDialog "ALife_SpawnMenu";
    };

    case "onLoad": {
        disableSerialization;
        private _display = findDisplay 4700;
        if (isNull _display) exitWith {};

        private _faction = [side player] call ALife_fnc_sideToFaction;
        _display setVariable ["alife_spawn_side", _faction];
        _display setVariable ["alife_spawn_point", []];

        private _listCtrl = _display displayCtrl 4720;
        lbClear _listCtrl;

        {
            _x params ["_key", "_displayName"];
            private _index = _listCtrl lbAdd _displayName;
            _listCtrl lbSetData [_index, _key];
        } forEach ([_faction] call ALife_fnc_getSpawnPoints);

        private _infoCtrl = _display displayCtrl 4740;
        _infoCtrl ctrlSetStructuredText parseText "Select a spawn point from the list.";

        private _mapCtrl = _display displayCtrl 4730;
        _mapCtrl ctrlMapAnimAdd [0, 0.15, [14000, 15000, 0]];
        ctrlMapAnimCommit _mapCtrl;
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
            hint "Select a spawn point first.";
        };

        private _spawnKey = _point select 0;

        closeDialog 0;

        // Faction isn't sent -- fn_spawnPlayer.sqf derives it authoritatively
        // from `side _unit` server-side rather than trusting a client value.
        [player, getPlayerUID player, _spawnKey] remoteExec ["ALife_fnc_spawnPlayer", 2];
    };

    case "onUnload": {
        {
            deleteMarkerLocal _x;
        } forEach (allMapMarkers select { _x find "alife_spawn_marker" == 0 });
    };
};
