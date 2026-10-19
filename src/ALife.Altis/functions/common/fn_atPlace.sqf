/*
    ALife_fnc_atPlace (common)

    Is a unit at a place from the config (a CfgShops placement, a
    CfgProcessing processor, a CfgGathering resource)? A place is any of:

      - its markers (markers[] or, for gathering, zones[]): inside an area
        marker (ellipse / rectangle), or within `radius` of a point marker;
      - its NPCs (npcs[], the names of units placed in Eden): within
        `radius` of one.

    Markers / NPCs that don't exist (not placed yet) are skipped.

    Arguments:
        0: OBJECT - the unit
        1: CONFIG - the place's class
        2: STRING - its marker list's name (default "markers")

    Returns:
        BOOL
*/

params [["_unit", objNull, [objNull]], ["_cfg", configNull, [configNull]], ["_key", "markers", [""]]];

if (isNull _unit || { isNull _cfg }) exitWith { false };
private _radius = getNumber (_cfg >> "radius");
if (_radius <= 0) then { _radius = 10; };

private _atMarker = (getArray (_cfg >> _key)) findIf {
    _x in allMapMarkers && {
        if (markerShape _x in ["ELLIPSE", "RECTANGLE"]) then { _unit inArea _x } else { _unit distance2D getMarkerPos _x <= _radius }
    }
};
if (_atMarker >= 0) exitWith { true };

(getArray (_cfg >> "npcs") findIf {
    private _npc = missionNamespace getVariable [_x, objNull];
    !isNull _npc && { _unit distance _npc <= _radius }
}) >= 0
