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
                                client from fn_playerJoin.sqf), closing the
                                connection loading screen first
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
        ["setProgress", 100, "Ready."] call ALife_fnc_loadingScreen;
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

        // Icon + title "card" rows (ALife_RscListNBox, a 2D listbox) rather
        // than plain single-line text -- column 0 is a narrow icon slot
        // (a generic location pin; no per-point custom icon asset exists
        // yet), column 1 is the display name. The key is stored as
        // invisible row data on column 0, same role lbSetData played on
        // the old plain listbox.
        private _listCtrl = _display displayCtrl 4720;
        lnbClear _listCtrl;
        _listCtrl lnbSetColumnsPos [0, 0.22];

        {
            _x params ["_key", "_displayName"];
            private _index = _listCtrl lnbAddRow ["", _displayName];
            _listCtrl lnbSetPicture [[_index, 0], "\A3\ui_f\data\map\mapcontrol\waypoint_ca.paa"];
            _listCtrl lnbSetData [[_index, 0], _key];
        } forEach ([_faction] call ALife_fnc_getSpawnPoints);

        private _infoCtrl = _display displayCtrl 4740;
        _infoCtrl ctrlSetStructuredText parseText "<t size='1.1' color='#8b949e'>Select a spawn point from the list.</t>";

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

        private _key = _ctrl lnbData [_index, 0];
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
        _infoCtrl ctrlSetStructuredText parseText format ["<t size='2.2' color='#ffffff'>%1</t><br/><t size='1' color='#8b949e'>Click Spawn to deploy here.</t>", toUpper _displayName];
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

        // Don't reveal the world yet -- ALife_fnc_spawnPlayer's remoteExec
        // below is fire-and-forget, and the player's unit is still sitting
        // wherever mission.sqm placed their Editor slot until the server
        // actually runs setPosATL. Closing straight to the 3D world here
        // was the exact cause of "spawning straight into the playable":
        // there's a real, visible window between this closing and the
        // server's positioning actually landing. Tonic's own framework
        // keeps a persistent black overlay up through this entire gap and
        // only reveals the world once the server confirms positioning is
        // done (docs/TONIC_REFERENCE.md §2/§4) -- same idea here, just with
        // our loading screen instead of a raw cutText.
        closeDialog 0;
        ["open"] call ALife_fnc_loadingScreen;
        ["setProgress", 95, "Spawning you in..."] call ALife_fnc_loadingScreen;

        // Faction isn't sent -- fn_spawnPlayer.sqf derives it authoritatively
        // from `side _unit` server-side rather than trusting a client value.
        // fn_spawnPlayer.sqf remoteExecs ALife_fnc_loadingScreen's "close"
        // mode back to this same client once positioning actually lands.
        [player, getPlayerUID player, _spawnKey] remoteExec ["ALife_fnc_spawnPlayer", 2];
    };

    case "onUnload": {
        {
            deleteMarkerLocal _x;
        } forEach (allMapMarkers select { _x find "alife_spawn_marker" == 0 });
    };
};
