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
if (_id == "") exitWith { ["That has no storage.", "warning"] remoteExecCall ["ALife_fnc_notify", owner _unit]; };

private _range = [4, 8] select ((_id select [0, 4]) == "veh_"); // vehicles are bigger
if (_unit distance _object > _range) exitWith { ["Get closer.", "warning"] remoteExecCall ["ALife_fnc_notify", owner _unit]; };

// Just cash on the ground (a money stack): pick it straight up, no menu.
private _container = ALife_containers get _id;
private _cash = _container get "cash";
if ((_container get "kind") == "ground" && { count (_container get "items") == 0 } && { _cash > 0 }) exitWith {
    [_unit, _id, "take", [], _cash] call ALife_fnc_containerTransfer; // removes the emptied stack
    private _left = (ALife_containers getOrDefault [_id, createHashMap]) getOrDefault ["cash", 0];
    if (_left < _cash) then {
        [format ["Picked up $%1.", [_cash - _left] call BIS_fnc_numberText], "success"] remoteExecCall ["ALife_fnc_notify", owner _unit];
    };
};

[_unit, _id] call ALife_fnc_containerSend;
