/*
    File: fn_getSpawnPoints.sqf
    Author: Tasman Dynamics

    Description:
        Reads spawn point definitions for a faction from CfgSpawnPoints
        (config/spawn_config.hpp) — the source of truth for where players
        can spawn, not a database table. See that file for why.

        Each entry's position resolves from its `marker` field (via
        getMarkerPos — the marker itself is placed in Eden) if set,
        otherwise falls back to its raw `position[]` array.

        Runs identically on client or server — config data is shared
        mission state, no privileged data here, unlike load/save.

    Parameter(s):
        0: STRING - faction ("civ" / "cop" / "medic")

    Returns:
        ARRAY of ARRAY - one entry per matching spawn point:
        [configClassName, displayName, resolvedPosition, gangKey]
        e.g. [["civ_kavala", "Kavala", [1234,5678,0], ""]]
        Empty array if none are defined for that faction.
*/

params ["_faction"];

private _allEntries = "true" configClasses (configFile >> "CfgSpawnPoints");
private _matching = _allEntries select { getText (_x >> "faction") == _faction };

_matching apply {
    private _entry = _x;
    private _marker = getText (_entry >> "marker");
    private _resolvedPosition = if (_marker != "") then {
        getMarkerPos _marker
    } else {
        getArray (_entry >> "position")
    };

    [
        configName _entry,
        getText (_entry >> "displayName"),
        _resolvedPosition,
        getText (_entry >> "gang")
    ]
}
