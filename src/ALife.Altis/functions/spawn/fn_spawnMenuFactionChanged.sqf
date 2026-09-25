/*
    File: fn_spawnMenuFactionChanged.sqf
    Author: Tasman Dynamics

    Description:
        Repopulates the spawn point list for whichever faction is now
        selected in the faction listbox. Wired as spawnMenu.hpp's
        FactionList onLBSelChanged.

    Parameter(s):
        None

    Returns:
        Nothing
*/

private _display = findDisplay 4700;
private _factionList = _display displayCtrl 4701;
private _spawnPointList = _display displayCtrl 4702;

private _selectedIndex = lbCurSel _factionList;
if (_selectedIndex == -1) exitWith {};

private _faction = _factionList lbData _selectedIndex;
private _spawnPoints = [_faction] call ALife_fnc_getSpawnPoints;

lbClear _spawnPointList;
{
    _x params ["_key", "_displayName"];
    private _index = _spawnPointList lbAdd _displayName;
    _spawnPointList lbSetData [_index, _key];
} forEach _spawnPoints;
