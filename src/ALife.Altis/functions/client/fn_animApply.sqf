/*
    ALife_fnc_animApply (client, from ALife_fnc_animSync)

    Another player's animation switch, shown on this machine.

    Arguments:
        0: OBJECT - their unit
        1: STRING - the move
*/

params [["_unit", objNull, [objNull]], ["_move", "", [""]]];

if (isNull _unit || { _unit == player } || { !alive _unit }) exitWith {};
_unit switchMove _move;
