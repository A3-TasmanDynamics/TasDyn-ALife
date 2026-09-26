/*
    File: fn_spawnMenuClose.sqf
    Author: Tasman Dynamics

    Description:
        Cleans up the map preview marker left over from whatever spawn
        point was selected. Wired as dialog/spawnMenu.hpp's onUnload --
        runs whether the dialog closed via Spawn or Cancel, so the marker
        can't survive past the dialog it belongs to.

    Parameter(s):
        None

    Returns:
        Nothing
*/

{
    deleteMarkerLocal _x;
} forEach (allMapMarkers select { _x find "alife_spawn_marker" == 0 });
