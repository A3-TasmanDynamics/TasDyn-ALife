/*
    ALife_fnc_vehicleTyres (common)

    Pops every tyre of a vehicle (spike strips): all its "wheel" hit points
    to fully damaged. Run where the vehicle is local.

    Arguments:
        0: OBJECT - the vehicle
*/

params [["_vehicle", objNull, [objNull]]];
if (isNull _vehicle || { !local _vehicle }) exitWith {};
{
    if ("wheel" in toLower _x) then { _vehicle setHitPointDamage [_x, 1]; };
} forEach ((getAllHitPointsDamage _vehicle) param [0, []]);
