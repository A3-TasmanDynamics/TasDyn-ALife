/*
    ALife_fnc_vehicleSpawnPoint (server)

    Where a vehicle bought or taken out of the garage appears: the shop's
    nearest vehicleSpawns marker when one is within CfgALife >>
    vehicleSpawnRange of the player (it must be clear), otherwise -- a shop
    NPC placed away from the markers -- a clear spot near the player (open
    water for boats).

    Arguments:
        0: OBJECT - the player
        1: CONFIG - the CfgShops placement
        2: STRING - the vehicle's class

    Returns:
        ARRAY - [spot, error]: spot is a marker name or [position, direction]
                for ALife_fnc_vehicleSpawn, "" when there's nowhere (error
                says why)
*/

params ["_unit", "_place", ["_class", "", [""]]];

private _spawns = (getArray (_place >> "vehicleSpawns") select { _x in allMapMarkers }) apply { [_unit distance2D getMarkerPos _x, _x] };
_spawns sort true;
private _range = getNumber (missionConfigFile >> "CfgALife" >> "vehicleSpawnRange") max 50;
if (_spawns isNotEqualTo [] && { ((_spawns select 0) select 0) <= _range }) exitWith {
    private _marker = (_spawns select 0) select 1;
    if ((nearestObjects [getMarkerPos _marker, ["LandVehicle", "Air", "Ship"], 7]) isNotEqualTo []) exitWith {
        ["", "The spawn point is blocked. Move the vehicle that's on it and try again."]
    };
    [_marker, ""]
};

// No spawn point here: a clear spot in front of them.
private _water = _class isKindOf "Ship";
private _from = _unit getPos [15, getDir _unit];
private _spot = [];
{
    private _p = _from findEmptyPosition [0, _x, _class];
    if (_p isNotEqualTo [] && { (surfaceIsWater _p) isEqualTo _water }) exitWith { _spot = _p; };
} forEach [20, 45, 80];
if (_spot isEqualTo []) exitWith {
    ["", ["There's no clear spot for it nearby. Try somewhere more open.", "There's no open water for it nearby."] select _water]
};
[[[_spot select 0, _spot select 1, 0], getDir _unit], ""]
