/*
    ALife_fnc_vehicleLockApply (client, called by the server)

    Sets a vehicle's lock where the vehicle is local (whoever is driving
    it); ALife_fnc_vehicleLock has already checked the keys.

    Arguments:
        0: OBJECT - the vehicle
        1: NUMBER - lock state (0 unlocked, 2 locked)
*/

params [["_vehicle", objNull, [objNull]], ["_state", 2, [0]]];

if (isNull _vehicle || { !local _vehicle } || { !(_state in [0, 2]) }) exitWith {};
_vehicle lock _state;
