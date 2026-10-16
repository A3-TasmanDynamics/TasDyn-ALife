/*
    ALife_fnc_itemsWeight

    Total weight of a list of virtual items (CfgItems >> weight).

    Arguments:
        0: ARRAY - [[class, count], ...]

    Returns:
        NUMBER
*/

params [["_items", []]];

private _total = 0;
{
    if (_x isEqualTypeParams ["", 0]) then {
        _total = _total + (_x select 1) * getNumber (missionConfigFile >> "CfgItems" >> (_x select 0) >> "weight");
    };
} forEach _items;
_total
