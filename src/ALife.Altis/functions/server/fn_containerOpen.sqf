/*
    ALife_fnc_containerOpen (server, called by the client with the Windows key)

    Opens a dropped bag or a vehicle's trunk for a player standing next to
    it.

    Arguments:
        0: OBJECT - the player
        1: OBJECT - the bag / vehicle they're looking at
*/

params [["_unit", objNull, [objNull]], ["_object", objNull, [objNull]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};
if (!alive _unit || { (_unit getVariable ["alife_faction", ""]) == "" }) exitWith {};

private _id = [_object] call ALife_fnc_containerGet;
if (_id == "") exitWith { ["That has no storage."] remoteExecCall ["ALife_fnc_notify", owner _unit]; };

private _range = [4, 8] select ((_id select [0, 4]) == "veh_"); // vehicles are bigger
if (_unit distance _object > _range) exitWith { ["Get closer."] remoteExecCall ["ALife_fnc_notify", owner _unit]; };

[_unit, _id] call ALife_fnc_containerSend;
