/*
    ALife_fnc_isAtm

    Whether an object is an ATM -- any ATM on the map, by its model or class
    (CfgALife >> atmModels / atmClasses), so nothing needs placing.

    Arguments:
        0: OBJECT

    Returns:
        BOOL
*/

params [["_object", objNull, [objNull]]];

if (isNull _object) exitWith { false };
private _cfg = missionConfigFile >> "CfgALife";
(typeOf _object) in getArray (_cfg >> "atmClasses")
    || { (toLower ((getModelInfo _object) select 0)) in (getArray (_cfg >> "atmModels") apply { toLower _x }) }
