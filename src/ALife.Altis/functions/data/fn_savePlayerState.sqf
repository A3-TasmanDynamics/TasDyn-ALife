/*
    File: fn_savePlayerState.sqf
    Author: Tasman Dynamics

    Description:
        Saves a player's current alive/position state for whichever faction
        they're actively playing. Shared by two call sites that both need
        exactly this: initServer.sqf's HandleDisconnect (the final save on
        a clean quit) and fn_sync.sqf's periodic autosave (protects against
        data loss on an ungraceful crash, which HandleDisconnect never
        fires for at all).

        A no-op for a unit that hasn't spawned into a faction yet
        (alife_activeFaction unset) -- nothing to save.

        Server-only.

    Parameter(s):
        0: OBJECT - the player's unit
        1: STRING - uid

    Returns:
        Nothing
*/

params ["_unit", "_uid"];

if (!isServer) exitWith {};

private _activeFaction = _unit getVariable ["alife_activeFaction", ""];
if (_activeFaction == "") exitWith {};

private _pos = getPosATL _unit;
private _positionValue = str [["x", _pos select 0], ["y", _pos select 1], ["z", _pos select 2]];
private _token = (str serverTime) + "-" + _uid;

[_uid, _activeFaction + "_alive", str (alive _unit), _token] call ALife_fnc_save;
[_uid, _activeFaction + "_position", _positionValue, _token] call ALife_fnc_save;
