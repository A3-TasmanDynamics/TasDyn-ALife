/*
    ALife_fnc_completeRevive (server, called by the revived player's client
    right after their respawn)

    Puts the new unit where the body was, removes the body, saves them as
    alive, and sends their gear back (ALife_fnc_reviveApplied).

    Arguments:
        0: OBJECT - the player's new unit
*/

params [["_unit", objNull, [objNull]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _uid = getPlayerUID _unit;
if (isNil "ALife_revives") then { ALife_revives = createHashMap; };
private _revive = ALife_revives getOrDefault [_uid, []];
if (_revive isEqualTo []) exitWith {
    // Nothing pending (e.g. the server restarted): fall back to the spawn menu.
    [[], "", ""] remoteExecCall ["ALife_fnc_reviveApplied", owner _unit];
};
ALife_revives deleteAt _uid;
_revive params ["_pos", "_loadout", "_body", "_medicName"];

private _faction = [side group _unit] call ALife_fnc_sideToFaction;
_unit setPosATL _pos;
_unit hideObjectGlobal false;
_unit setVariable ["alife_uid", _uid];
_unit setVariable ["alife_faction", _faction];
_unit setVariable ["alife_name", name _unit, true];
if (!isNull _body) then { deleteVehicle _body; };

private _record = ALife_sessions getOrDefault [_uid, createHashMap];
_record set [_faction + "_alive", true];
["save", [_uid, _faction + "_alive", "true", "-"]] call ALife_fnc_extension;
["save", [_uid, _faction + "_position", str [["x", _pos select 0], ["y", _pos select 1], ["z", _pos select 2]], "-"]] call ALife_fnc_extension;

diag_log format ["[ALife] %1 is back up (revived by %2)", _uid, _medicName];
[_pos, _loadout, _medicName] remoteExecCall ["ALife_fnc_reviveApplied", owner _unit];
