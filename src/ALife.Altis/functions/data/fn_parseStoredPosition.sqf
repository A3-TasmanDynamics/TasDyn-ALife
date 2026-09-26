/*
    File: fn_parseStoredPosition.sqf
    Author: Tasman Dynamics

    Description:
        Safely converts a loaded record's raw <faction>_position value
        (docs/DATA_CONTRACT.md: array of [key,value] pairs, e.g.
        [["x",1234.5],["y",5678.2],["z",10.5]]) into a [x,y,z] position
        array. Never throws on malformed input -- returns nil instead, so
        a caller can treat "malformed" the same as "nothing stored" rather
        than crashing (fn_spawnPlayer.sqf used to call
        `createHashMapFromArray` directly on this value with no validation
        at all, which is exactly what "Invalid number in expression"
        turned out to mean once fed anything that wasn't a clean array of
        pairs).

    Parameter(s):
        0: ANYTHING - the raw <faction>_position value from the loaded
           record (expected ARRAY, but not trusted to be one)

    Returns:
        ARRAY [x,y,z] if the input parses to a complete, valid position, or
        NIL (check with isNil, not count/emptiness) if it's missing,
        malformed, or incomplete.
*/

params ["_raw"];

if (!(_raw isEqualType []) || { count _raw == 0 }) exitWith {};

private _positionMap = _raw createHashMapFromArray;
private _x = _positionMap getOrDefault ["x", ""];
private _y = _positionMap getOrDefault ["y", ""];
private _z = _positionMap getOrDefault ["z", ""];

if !(_x isEqualType 0 && { _y isEqualType 0 } && { _z isEqualType 0 }) exitWith {};

[_x, _y, _z]
