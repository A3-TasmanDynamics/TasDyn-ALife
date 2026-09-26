/*
    File: fn_spawnMenuSelectLocation.sqf
    Author: Tasman Dynamics

    Description:
        Handles a spawn point selection in the list -- stores the pick,
        drops a marker on the map preview, and pans to it. Wired as
        dialog/spawnMenu.hpp's SpawnList onLBSelChanged (called with the
        control and the newly-selected index).

    Parameter(s):
        0: CONTROL - the spawn point listbox
        1: SCALAR  - the newly-selected index

    Returns:
        Nothing
*/

params ["_ctrl", "_index"];

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
