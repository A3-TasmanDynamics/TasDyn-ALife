/*
    ALife_fnc_savePlayer (server)

    Saves a spawned player's alive state and position for the faction
    they're playing. Does nothing for a player still in the spawn menu.

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - Steam UID (optional; read from the unit if empty)

    Returns:
        BOOL - true if both saves succeeded
*/

params ["_unit", ["_uid", ""]];

if (!isServer || { isNull _unit }) exitWith { false };

private _faction = _unit getVariable ["alife_faction", ""];
if (_faction == "") exitWith { false };
if (_uid == "") then { _uid = _unit getVariable ["alife_uid", ""]; };
if (_uid == "") exitWith { false };

private _pos = getPosATL _unit;
private _position = str [["x", _pos select 0], ["y", _pos select 1], ["z", _pos select 2]];

private _aliveResult = ["save", [_uid, _faction + "_alive", str (alive _unit), "-"]] call ALife_fnc_extension;
private _posResult = ["save", [_uid, _faction + "_position", _position, "-"]] call ALife_fnc_extension;

private _ok = _aliveResult == "OK" && { _posResult == "OK" };
if (!_ok) then {
    diag_log format ["[ALife] save failed for %1 (%2): alive=%3 position=%4", _uid, _faction, _aliveResult, _posResult];
};
_ok
