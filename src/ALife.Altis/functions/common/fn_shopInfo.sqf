/*
    ALife_fnc_shopInfo (common)

    A shop placement (CfgShops) and the shop it puts on the map, which lives
    in its type's config in Tonic's format:

        "market"   -> VirtualShops  (CfgMarket.hpp)
        "weapons"  -> WeaponShops   (CfgWeapons.hpp)
        "clothing" -> ClothingShops (CfgClothing.hpp)
        "vehicles" -> CarShops      (CfgVehicles.hpp)

    Arguments:
        0: STRING - CfgShops class

    Returns:
        ARRAY - [type, placement config, shop config], or [] if either is
        missing
*/

params [["_placement", "", [""]]];

private _place = missionConfigFile >> "CfgShops" >> _placement;
if (!isClass _place) exitWith { [] };

private _type = getText (_place >> "type");
private _index = ["market", "weapons", "clothing", "vehicles"] find _type;
if (_index < 0) exitWith { [] };
private _root = ["VirtualShops", "WeaponShops", "ClothingShops", "CarShops"] select _index;

private _shop = missionConfigFile >> _root >> getText (_place >> "shop");
if (!isClass _shop) exitWith { [] };

[_type, _place, _shop]
