/*
    ALife_fnc_vehicleLock (server, called by the client: U near your vehicle)

    Locks or unlocks a vehicle for its owner (alife_owner = their Steam UID)
    or someone they gave a key to (alife_keys).
    `lock` only works where the vehicle is local, so the change is applied
    there (the server, or whoever is driving).

    Arguments:
        0: OBJECT - the player's unit
        1: OBJECT - the vehicle
*/

params [["_unit", objNull, [objNull]], ["_vehicle", objNull, [objNull]]];

if (!isServer || { isNull _unit } || { isNull _vehicle }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};
if (!alive _unit || { !alive _vehicle } || { _unit distance _vehicle > 15 }) exitWith {};
private _uid = getPlayerUID _unit;
if ((_vehicle getVariable ["alife_owner", ""]) != _uid && { !(_uid in (_vehicle getVariable ["alife_keys", []])) }) exitWith {
    ["You don't have the keys to this vehicle.", "warning"] remoteExecCall ["ALife_fnc_notify", owner _unit];
};

private _locked = locked _vehicle >= 2;
private _state = [2, 0] select _locked; // locked -> unlock, otherwise lock
if (local _vehicle) then {
    _vehicle lock _state;
} else {
    [_vehicle, _state] remoteExecCall ["ALife_fnc_vehicleLockApply", _vehicle];
};
[["Locked.", "Unlocked."] select _locked, "success"] remoteExecCall ["ALife_fnc_notify", owner _unit];
