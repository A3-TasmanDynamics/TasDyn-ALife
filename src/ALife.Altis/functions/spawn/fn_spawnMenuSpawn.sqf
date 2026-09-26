/*
    File: fn_spawnMenuSpawn.sqf
    Author: Tasman Dynamics

    Description:
        Reads the current side/spawn-point selection and sends it to the
        server as a REQUEST -- fn_spawnPlayer.sqf re-validates everything
        server-side and can override it entirely (e.g. if the player is
        resuming from a not-alive state, this selection is ignored
        outright). Wired as dialog/spawnMenu.hpp's BtnSpawn action.

    Parameter(s):
        None

    Returns:
        Nothing
*/

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
