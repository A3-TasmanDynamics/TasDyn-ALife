/*
    ALife_fnc_requestSpawn (server, called by the client)

    The player picked a spawn point. The server checks it's one they're
    allowed (the client only sends a key, never a position), finds a clear
    spot nearby, shows the unit and answers with ALife_fnc_spawnApproved.

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - spawn point key (or "last")
*/

params [["_unit", objNull, [objNull]], ["_key", "", [""]]];

if (!isServer || { isNull _unit }) exitWith {};

if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {
    diag_log format ["[ALife] requestSpawn refused: client %1 asked for a unit it doesn't own (%2)", remoteExecutedOwner, _unit];
};

private _owner = owner _unit;
private _uid = getPlayerUID _unit;
private _faction = [side group _unit] call ALife_fnc_sideToFaction;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];

if ((_record getOrDefault ["status", ""]) != "OK") exitWith {
    [[], "", "Your session has expired. Please reconnect."] remoteExecCall ["ALife_fnc_spawnApproved", _owner];
};

private _options = [_record, _faction] call ALife_fnc_spawnOptions;
private _index = _options findIf { (_x select 0) == _key };
if (_index < 0) exitWith {
    diag_log format ["[ALife] %1 asked for spawn '%2' which isn't a %3 spawn point", _uid, _key, _faction];
    [[], "", "That spawn point isn't available. Pick another."] remoteExecCall ["ALife_fnc_spawnApproved", _owner];
};

(_options select _index) params ["", "_name", "", "_pos"];

private _target = [_pos select 0, _pos select 1, 0];
if (_key == "last") then {
    _target = +_pos; // exact stored position (may be upstairs)
} else {
    private _radius = getNumber (missionConfigFile >> "CfgSpawnPoints" >> _faction >> _key >> "radius") max 5;
    private _clear = _target findEmptyPosition [0, _radius, "C_man_1"];
    if (!(_clear isEqualTo []) && { !surfaceIsWater _clear }) then {
        _target = [_clear select 0, _clear select 1, 0];
    };
};

_unit setPosATL _target;
_unit hideObjectGlobal false;
_unit setVariable ["alife_uid", _uid];
_unit setVariable ["alife_faction", _faction];
_unit setVariable ["alife_name", name _unit, true]; // shown to medics reviving them
_unit setVariable ["alife_gang", _record getOrDefault ["gang_tag", ""], true]; // gang uniform textures

_record set [_faction + "_alive", true];
// Save the destination itself: the unit's own position may not have
// caught up yet on the server.
private _saved = ["save", [_uid, _faction + "_alive", "true", "-"]] call ALife_fnc_extension;
["save", [_uid, _faction + "_position", str [["x", _target select 0], ["y", _target select 1], ["z", _target select 2]], "-"]] call ALife_fnc_extension;

diag_log format ["[ALife] %1 spawned as %2 at %3 %4 (alive saved: %5)", _uid, _faction, _key, _target, _saved];

// The outfit they had (saved gear); none after dying and respawning.
private _gear = _record getOrDefault [_faction + "_gear", []];
private _loadout = [];
if (_gear isEqualType []) then {
    private _i = _gear findIf { _x isEqualType [] && { (_x param [0, ""]) isEqualTo "loadout" } };
    if (_i >= 0) then { _loadout = (_gear select _i) param [1, []]; };
};

[_target, _name, "", _loadout] remoteExecCall ["ALife_fnc_spawnApproved", _owner];
