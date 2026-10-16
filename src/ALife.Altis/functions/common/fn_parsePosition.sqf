/*
    ALife_fnc_parsePosition

    Turns a stored position ([["x",1],["y",2],["z",3]]) into [x,y,z].

    Arguments:
        0: ANYTHING - the stored value (not trusted to be well-formed)

    Returns:
        ARRAY [x,y,z], or nothing (check with isNil) if there's no valid
        position stored.
*/

params ["_raw"];

if (!(_raw isEqualType []) || { _raw isEqualTo [] }) exitWith {};
if ((_raw findIf { !(_x isEqualTypeParams ["", 0]) }) >= 0) exitWith {};

private _map = createHashMapFromArray _raw;
private _px = _map getOrDefault ["x", ""];
private _py = _map getOrDefault ["y", ""];
private _pz = _map getOrDefault ["z", 0];

if (!(_px isEqualType 0) || { !(_py isEqualType 0) } || { !(_pz isEqualType 0) }) exitWith {};
if (_px <= 0 || { _py <= 0 } || { _px > worldSize } || { _py > worldSize }) exitWith {};

[_px, _py, _pz max 0]
