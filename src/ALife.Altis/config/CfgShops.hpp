// Where the shops are. Each entry puts one shop on the map:
//
//   type      -- "market" (VirtualShops, CfgMarket.hpp), "weapons"
//                (WeaponShops, CfgWeapons.hpp), "clothing" (ClothingShops,
//                CfgClothing.hpp) or "vehicles" (CarShops, CfgVehicles.hpp)
//   shop      -- the shop's class there (its name, side, licence and stock)
//   markers[] -- mission.sqm markers it's at
//   radius    -- metres from a marker
//   vehicleSpawns[] -- vehicle shops: where bought vehicles appear
//
// Shops sit on the town spawn markers for now; give each its own marker in
// Eden when they get placed properly. Only markets can be bought from so far;
// weapons, clothing and vehicles are priced and ready for the next step.

class CfgShops
{
    class general_market
    {
        type = "market";
        shop = "general";
        markers[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
        radius = 30;
    };

    class general_store
    {
        type = "weapons";
        shop = "genstore";
        markers[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
        radius = 30;
    };

    class civ_clothing
    {
        type = "clothing";
        shop = "civ";
        markers[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
        radius = 30;
    };

    class gun_store
    {
        type = "weapons";
        shop = "gun";
        markers[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos" };
        radius = 30;
    };

    class rebel_market
    {
        type = "market";
        shop = "rebel";
        markers[] = { "spawn_civ_sofia" };
        radius = 30;
    };

    class rebel_armoury
    {
        type = "weapons";
        shop = "rebel";
        markers[] = { "spawn_civ_sofia" };
        radius = 30;
    };

    class rebel_clothing
    {
        type = "clothing";
        shop = "rebel";
        markers[] = { "spawn_civ_sofia" };
        radius = 30;
    };

    class rebel_vehicles
    {
        type = "vehicles";
        shop = "rebel";
        markers[] = { "spawn_civ_sofia" };
        radius = 30;
        vehicleSpawns[] = { "spawn_civ_sofia" };
    };

    class civ_cars
    {
        type = "vehicles";
        shop = "civ_car";
        markers[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
        radius = 30;
        vehicleSpawns[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
    };

    class civ_trucks
    {
        type = "vehicles";
        shop = "civ_truck";
        markers[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
        radius = 30;
        vehicleSpawns[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
    };

    class civ_air
    {
        type = "vehicles";
        shop = "civ_air";
        markers[] = { "spawn_civ_athira", "spawn_civ_pyrgos" };
        radius = 30;
        vehicleSpawns[] = { "spawn_civ_athira", "spawn_civ_pyrgos" };
    };

    class civ_boats
    {
        type = "vehicles";
        shop = "civ_ship";
        markers[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
        radius = 30;
        vehicleSpawns[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
    };

    class cop_market
    {
        type = "market";
        shop = "cop";
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
        radius = 30;
    };

    class cop_supplies
    {
        type = "weapons";
        shop = "cop_kit";
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
        radius = 30;
    };

    class cop_armoury
    {
        type = "weapons";
        shop = "cop";
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
        radius = 30;
    };

    class cop_clothing
    {
        type = "clothing";
        shop = "cop";
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
        radius = 30;
    };

    class cop_cars
    {
        type = "vehicles";
        shop = "cop_car";
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
        radius = 30;
        vehicleSpawns[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
    };

    class cop_air
    {
        type = "vehicles";
        shop = "cop_air";
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira" };
        radius = 30;
        vehicleSpawns[] = { "spawn_cop_kavala", "spawn_cop_athira" };
    };

    class cop_boats
    {
        type = "vehicles";
        shop = "cop_ship";
        markers[] = { "spawn_cop_kavala", "spawn_cop_pyrgos" };
        radius = 30;
        vehicleSpawns[] = { "spawn_cop_kavala", "spawn_cop_pyrgos" };
    };

    class medic_market
    {
        type = "market";
        shop = "medic";
        markers[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
        radius = 30;
    };

    class medic_equipment
    {
        type = "weapons";
        shop = "medic_kit";
        markers[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
        radius = 30;
    };

    class medic_clothing
    {
        type = "clothing";
        shop = "medic";
        markers[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
        radius = 30;
    };

    class medic_vehicles
    {
        type = "vehicles";
        shop = "med_car";
        markers[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
        radius = 30;
        vehicleSpawns[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
    };

    class medic_air
    {
        type = "vehicles";
        shop = "med_air";
        markers[] = { "spawn_medic_kavala", "spawn_medic_athira" };
        radius = 30;
        vehicleSpawns[] = { "spawn_medic_kavala", "spawn_medic_athira" };
    };
};
