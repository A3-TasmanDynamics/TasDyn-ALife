/*
    ALife_fnc_animSync (server, called by the client)

    switchMove only changes an animation on the machine that runs it, so a
    player's own switch (e.g. the move out of the surrender pose) is passed
    on to everyone else here (ALife_fnc_animApply). Only listed moves, and
    only for the caller's own unit.

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - the move
*/

params [["_unit", objNull, [objNull]], ["_move", "", [""]]];

if (!isServer || { isNull _unit } || { !alive _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};
if !(_move in ["AmovPercMstpSsurWnonDnon_AmovPercMstpSnonWnonDnon", "AmovPercMstpSnonWnonDnon"]) exitWith {};

[_unit, _move] remoteExecCall ["ALife_fnc_animApply", -2];
