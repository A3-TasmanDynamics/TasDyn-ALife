/*
    File: fn_spawnMenuOpen.sqf
    Author: Tasman Dynamics

    Description:
        Runs once when the spawn dialog's display is created -- wired as
        dialog/spawnMenu.hpp's ALife_SpawnMenu onLoad. Defaults to the
        civilian side and centers the map preview on Altis before the
        player has picked anything.

    Parameter(s):
        None

    Returns:
        Nothing
*/

disableSerialization;

private _display = findDisplay 4700;
if (isNull _display) exitWith {};

["civ"] call ALife_fnc_spawnMenuSelectSide;

private _mapCtrl = _display displayCtrl 4730;
_mapCtrl ctrlMapAnimAdd [0, 0.15, [14000, 15000, 0]];
ctrlMapAnimCommit _mapCtrl;
