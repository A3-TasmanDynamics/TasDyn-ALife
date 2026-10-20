/*
    ALife_fnc_vehicleSpawn (server)

    Puts an owned vehicle into the world: bought at a shop or taken out of
    the garage. In its skin, locked, owned (alife_owner), with its plate
    and database id, and -- from the garage -- its fuel, hit-point damage
    and trunk as they were stored. Its trunk is registered straight away
    (ALife_containers) and saved whenever it changes
    (ALife_fnc_containerTransfer). If it's destroyed, the database marks it
    destroyed and the trunk is lost.

    Arguments:
        0: STRING - classname
        1: STRING - spawn marker (position and direction), or ARRAY [position, direction]
        2: STRING - owner's Steam UID
        3: STRING - vehicles.id ("" if it isn't recorded)
        4: STRING - plate
        5: STRING - skin choice (ALife_fnc_vehicleSkin: "tex:<row>" / "src:<class>" / "")
        6: NUMBER - fuel 0-1 (default 1)
        7: ARRAY  - hit-point damage [[name, damage], ...] (default none)
        8: ARRAY  - trunk [["items", [[class, count], ...]], ["cash", n]] (default empty)

    Returns:
        OBJECT - the vehicle
*/

params ["_class", "_marker", "_uid", ["_id", ""], ["_plate", ""], ["_skin", ""], ["_fuel", 1], ["_damage", []], ["_trunk", []]];

private _pos = if (_marker isEqualType "") then { getMarkerPos _marker } else { _marker select 0 };
private _vehicle = createVehicle [_class, _pos, [], 0, "NONE"];
_vehicle setDir (if (_marker isEqualType "") then { markerDir _marker } else { _marker select 1 });
_vehicle setPosATL [_pos select 0, _pos select 1, 0.1];
{ _vehicle setObjectTextureGlobal [_forEachIndex, _x]; } forEach ([_class, _skin] call ALife_fnc_vehicleSkin);
clearItemCargoGlobal _vehicle;
clearWeaponCargoGlobal _vehicle;
clearMagazineCargoGlobal _vehicle;
clearBackpackCargoGlobal _vehicle;
_vehicle lock 2;
_vehicle setFuel ((_fuel max 0) min 1);
{
    if (_x isEqualTypeParams ["", 0] && { (_x select 0) != "" }) then { _vehicle setHitPointDamage [_x select 0, ((_x select 1) max 0) min 0.9]; };
} forEach _damage;
_vehicle setVariable ["alife_owner", _uid, true];
_vehicle setVariable ["alife_plate", _plate, true];
_vehicle setVariable ["alife_skin", _skin];
if (_id != "") then {
    _vehicle setVariable ["alife_vehicleId", _id];
    if (isNil "ALife_vehicleObjects") then { ALife_vehicleObjects = createHashMap; };
    ALife_vehicleObjects set [_id, _vehicle];
};

// Its trunk, as it was stored.
private _items = createHashMap;
private _cash = 0;
{
    if (_x isEqualTypeParams ["", []] && { (_x select 0) == "items" }) then {
        { if (_x isEqualTypeParams ["", 0] && { isClass (missionConfigFile >> "CfgItems" >> (_x select 0)) } && { (_x select 1) > 0 }) then { _items set [_x select 0, floor (_x select 1)]; }; } forEach (_x select 1);
    };
    if (_x isEqualTypeParams ["", 0] && { (_x select 0) == "cash" }) then { _cash = floor ((_x select 1) max 0); };
} forEach _trunk;
if (count _items > 0 || { _cash > 0 }) then {
    private _container = [_vehicle] call ALife_fnc_containerGet; // registers it, empty
    if (_container != "") then {
        private _c = ALife_containers get _container;
        _c set ["items", _items];
        _c set ["cash", _cash];
    };
};

// Wrecked: it's gone for good.
_vehicle addEventHandler ["Killed", {
    params ["_vehicle"];
    private _id = _vehicle getVariable ["alife_vehicleId", ""];
    if (_id == "") exitWith {};
    private _r = ["vehicle", ["status", _id, "destroyed"]] call ALife_fnc_extension;
    if (_r != "OK") then { diag_log format ["[ALife] vehicle %1 destroyed but not recorded: %2", _id, _r]; };
    ALife_vehicleObjects deleteAt _id;
    private _container = _vehicle getVariable ["alife_container", ""];
    if (_container != "") then { ALife_containers deleteAt _container; };
    diag_log format ["[ALife] vehicle %1 (%2, plate %3) destroyed", _id, typeOf _vehicle, _vehicle getVariable ["alife_plate", ""]];
}];

_vehicle
