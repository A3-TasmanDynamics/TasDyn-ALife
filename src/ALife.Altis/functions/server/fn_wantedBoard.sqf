/*
    ALife_fnc_wantedBoard (server, called by the client: the phone's Wanted app)

    Police only: everyone on the wanted list ([[uid, name, bounty, crimes],
    ...], highest bounty first), and for one player their crimes, sent to
    the officer's ALife_fnc_wanted.

    Arguments:
        0: OBJECT - the officer
        1: STRING - a player's uid for their crimes, or "" for the board
*/

params [["_unit", objNull, [objNull]], ["_uid", "", [""]]];
if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};
if ((_unit getVariable ["alife_faction", ""]) != "cop") exitWith {};
if (_uid == "") exitWith {
    private _raw = ["wanted", ["board"]] call ALife_fnc_extension;
    if (_raw select [0, 6] == "ERROR:") exitWith { diag_log format ["[ALife] wanted board failed: %1", _raw]; };
    ["board", parseSimpleArray _raw] remoteExecCall ["ALife_fnc_wanted", owner _unit];
};
private _raw = ["wanted", ["list", _uid]] call ALife_fnc_extension;
if (_raw select [0, 6] == "ERROR:") exitWith {};
["crimes", [_uid, parseSimpleArray _raw]] remoteExecCall ["ALife_fnc_wanted", owner _unit];
