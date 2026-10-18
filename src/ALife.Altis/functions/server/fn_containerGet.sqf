/*
    ALife_fnc_containerGet (server)

    The container behind an object: a dropped bag, or a vehicle's trunk
    (registered the first time someone opens it).

    Containers live in ALife_containers (server only), keyed by id:
        object   - the bag / vehicle
        kind     - "ground" (dropped bag) or "vehicle"
        title    - shown in the inventory screen
        items    - HASHMAP class -> count
        cash     - money in it
        capacity - weight it holds
        created  - time it was created

    Arguments:
        0: OBJECT

    Returns:
        STRING - the container id, or "" if the object can't hold things
*/

params [["_object", objNull, [objNull]]];

if (isNull _object) exitWith { "" };
if (isNil "ALife_containers") then { ALife_containers = createHashMap; ALife_containerCount = 0; };

// A dropped bag: find its container by the object.
private _bagId = "";
{
    if ((_y get "object") isEqualTo _object) exitWith { _bagId = _x; };
} forEach ALife_containers;
if (_bagId != "") exitWith { _bagId };

private _id = _object getVariable ["alife_container", ""];
if (_id != "" && { _id in ALife_containers }) exitWith { _id };

if (!alive _object || { !(_object isKindOf "LandVehicle" || { _object isKindOf "Air" } || { _object isKindOf "Ship" }) }) exitWith { "" };

private _type = switch (true) do {
    case (_object isKindOf "Truck_F"): { "truck" };
    case (_object isKindOf "Car"): { "car" };
    case (_object isKindOf "Air"): { "air" };
    case (_object isKindOf "Ship"): { "ship" };
    default { "other" };
};

_id = "veh_" + netId _object;
ALife_containers set [_id, createHashMapFromArray [
    ["object", _object],
    ["kind", "vehicle"],
    ["title", getText (configFile >> "CfgVehicles" >> typeOf _object >> "displayName")],
    ["items", createHashMap],
    ["cash", 0],
    ["capacity", getNumber (missionConfigFile >> "CfgALife" >> "Trunks" >> _type)],
    ["created", time]
]];
_object setVariable ["alife_container", _id, true];
_id
