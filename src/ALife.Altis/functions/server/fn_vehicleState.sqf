/*
    ALife_fnc_vehicleState (server)

    What gets saved about an owned vehicle when it leaves the world (stored
    in the garage, impounded) or its trunk changes: fuel, hit-point damage
    and its trunk, as JSON-ready arrays for the extension's "vehicle" store /
    inventory ops.

    Arguments:
        0: OBJECT - the vehicle

    Returns:
        ARRAY - [fuel, damage [[hitpoint, damage], ...], trunk [["items", [[class, count], ...]], ["cash", n]]]
*/

params [["_vehicle", objNull, [objNull]]];

private _damage = [];
(getAllHitPointsDamage _vehicle) params [["_names", []], "", ["_values", []]];
{
    private _d = _values param [_forEachIndex, 0];
    if (_x != "" && { _d > 0.01 }) then { _damage pushBack [_x, (round (_d * 100)) / 100]; };
} forEach _names;

private _items = [];
private _cash = 0;
private _container = ALife_containers getOrDefault [_vehicle getVariable ["alife_container", ""], createHashMap];
if (count _container > 0) then {
    { _items pushBack [_x, _y]; } forEach (_container get "items");
    _cash = _container get "cash";
};

[(round ((fuel _vehicle) * 100)) / 100, _damage, [["items", _items], ["cash", _cash]]]
