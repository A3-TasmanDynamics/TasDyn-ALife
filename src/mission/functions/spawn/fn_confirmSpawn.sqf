/*
    File: fn_confirmSpawn.sqf
    Author: Tasman Dynamics

    Description:
        Reads the current faction/spawn point selection and sends it to
        the server as a REQUEST. fn_spawnPlayer.sqf re-validates
        everything server-side and can override it entirely — e.g. if the
        player is resuming from a not-alive state, their selection here is
        ignored outright. Wired as spawnMenu.hpp's SpawnButton action.

    Parameter(s):
        None

    Returns:
        Nothing
*/

private _display = findDisplay 4700;
private _factionList = _display displayCtrl 4701;
private _spawnPointList = _display displayCtrl 4702;

private _factionIndex = lbCurSel _factionList;
private _spawnIndex = lbCurSel _spawnPointList;

if (_factionIndex == -1 || _spawnIndex == -1) exitWith {
    hint "Select a faction and a spawn point first.";
};

private _faction = _factionList lbData _factionIndex;
private _spawnKey = _spawnPointList lbData _spawnIndex;

closeDialog 0;

[player, getPlayerUID player, _faction, _spawnKey] remoteExec ["ALife_fnc_spawnPlayer", 2];
