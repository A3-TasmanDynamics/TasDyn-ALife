/*
    ALife_fnc_requestPhoneData (server; called by the client, and by other
    server functions after a change)

    Sends a player what their phone shows -- items, money, levels -- from
    their loaded record (ALife_fnc_phoneData on their machine).

    Arguments:
        0: OBJECT - the player's unit
*/

params [["_unit", objNull, [objNull]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
if ((_record getOrDefault ["status", ""]) != "OK") exitWith {};

private _faction = [side group _unit] call ALife_fnc_sideToFaction;

[
    _faction,
    _record getOrDefault [_faction + "_items", []],
    _record getOrDefault [_faction + "_cash", 0],
    _record getOrDefault [_faction + "_bank", 0],
    _record getOrDefault ["cop_level", 0],
    _record getOrDefault ["medic_level", 0],
    _record getOrDefault ["staff_rank_key", ""],
    _record getOrDefault ["staff_level", 0]
] remoteExecCall ["ALife_fnc_phoneData", owner _unit];
