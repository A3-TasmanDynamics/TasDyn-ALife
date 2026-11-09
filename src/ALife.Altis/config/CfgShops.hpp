// Where the shops are. Each entry puts one shop on the map:
//
//   type      -- "market" (VirtualShops, CfgMarket.hpp), "weapons"
//                (WeaponShops, CfgWeapons.hpp), "clothing" (ClothingShops,
//                CfgClothing.hpp), "vehicles" (CarShops, CfgVehicles.hpp)
//                or "licences" (a licence office: no shop class; its own
//                name, side and licences[] -- CfgLicences keys with a price)
//   shop      -- the shop's class there (its name, side, licence and stock)
//   markers[] -- mission.sqm markers it's at
//   npcs[]    -- and / or units placed in Eden (their variable names): look at
//                one and press Windows to open the shop -- or, when the same
//                NPC serves several shops, a wheel of them (with the Garage
//                for vehicle shops, and the locker for your faction's
//                receptionist). They stand still and can't be hurt
//                (ALife_fnc_serverInit). Names used, _1 to _6 each:
//                  civ_generalstore (market + general store), civ_clothing,
//                  civ_gunstore, civ_vehicleshop (cars + trucks),
//                  civ_airshop, civ_boatshop, rebel_trader (all rebel shops),
//                  police_receptionist / medic_receptionist (every police /
//                  EMS shop, and their lockers).
//                Vehicles appear at the nearest vehicleSpawns marker within
//                CfgALife >> vehicleSpawnRange, else at a clear spot nearby.
//   Police vehicle spawn markers (empty markers in Eden, facing the way the
//   vehicle should): police_car_spawn_<town>_1..3 (kavala / athira /
//   pyrgos), police_air_spawn_<town>_1..2 (kavala / athira, on a helipad),
//   police_boat_spawn_<town>_1..2 (on the water). Any not placed are skipped;
//   the nearest clear one is used.
//   radius    -- metres from a marker / NPC
//   vehicleSpawns[] -- vehicle shops: where bought vehicles appear
//   mapMarker -- optional: a map marker with this text on each of its NPCs
//
// Shops are reached through their NPCs only (markers[] left empty): the
// Windows key on one, within its radius (the server checks it again on
// every purchase).

class CfgShops
{
    // Licence office: place civilian units in Eden named licence_vendor_1,
    // licence_vendor_2, ... (add more names here).
    class licence_office
    {
        type = "licences";
        name = "Licence Office";
        side = "civ";
        licences[] = { "driver", "truck", "pilot", "boat", "dive", "gun", "hunting", "home" };
        markers[] = {};
        npcs[] = { "licence_vendor_1", "licence_vendor_2", "licence_vendor_3" };
        radius = 5;
        mapMarker = "Licence Office";
    };

    class resource_trader
    {
        type = "market";
        shop = "resources";
        markers[] = {};
        npcs[] = { "trader_resources_1", "trader_resources_2" }; // units placed in Eden: Windows key on one
        radius = 5;
    };

    class general_market
    {
        type = "market";
        shop = "general";
        markers[] = {};
        npcs[] = { "civ_generalstore_1", "civ_generalstore_2", "civ_generalstore_3", "civ_generalstore_4", "civ_generalstore_5", "civ_generalstore_6" };
        radius = 8;
    };

    class general_store
    {
        type = "weapons";
        shop = "genstore";
        markers[] = {};
        npcs[] = { "civ_generalstore_1", "civ_generalstore_2", "civ_generalstore_3", "civ_generalstore_4", "civ_generalstore_5", "civ_generalstore_6" };
        radius = 8;
    };

    class civ_clothing
    {
        type = "clothing";
        shop = "civ";
        markers[] = {};
        npcs[] = { "civ_clothing_1", "civ_clothing_2", "civ_clothing_3", "civ_clothing_4", "civ_clothing_5", "civ_clothing_6" };
        radius = 8;
    };

    class gun_store
    {
        type = "weapons";
        shop = "gun";
        markers[] = {};
        npcs[] = { "civ_gunstore_1", "civ_gunstore_2", "civ_gunstore_3", "civ_gunstore_4", "civ_gunstore_5", "civ_gunstore_6" };
        radius = 8;
    };

    class rebel_market
    {
        type = "market";
        shop = "rebel";
        markers[] = {};
        npcs[] = { "rebel_trader_1", "rebel_trader_2", "rebel_trader_3", "rebel_trader_4", "rebel_trader_5", "rebel_trader_6" };
        radius = 8;
    };

    class rebel_armoury
    {
        type = "weapons";
        shop = "rebel";
        markers[] = {};
        npcs[] = { "rebel_trader_1", "rebel_trader_2", "rebel_trader_3", "rebel_trader_4", "rebel_trader_5", "rebel_trader_6" };
        radius = 8;
    };

    class rebel_clothing
    {
        type = "clothing";
        shop = "rebel";
        markers[] = {};
        npcs[] = { "rebel_trader_1", "rebel_trader_2", "rebel_trader_3", "rebel_trader_4", "rebel_trader_5", "rebel_trader_6" };
        radius = 8;
    };

    class rebel_vehicles
    {
        type = "vehicles";
        shop = "rebel";
        markers[] = {};
        npcs[] = { "rebel_trader_1", "rebel_trader_2", "rebel_trader_3", "rebel_trader_4", "rebel_trader_5", "rebel_trader_6" };
        radius = 8;
        vehicleSpawns[] = { "spawn_civ_sofia" };
    };

    class civ_cars
    {
        type = "vehicles";
        shop = "civ_car";
        markers[] = {};
        npcs[] = { "civ_vehicleshop_1", "civ_vehicleshop_2", "civ_vehicleshop_3", "civ_vehicleshop_4", "civ_vehicleshop_5", "civ_vehicleshop_6" };
        radius = 8;
        vehicleSpawns[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
    };

    class civ_trucks
    {
        type = "vehicles";
        shop = "civ_truck";
        markers[] = {};
        npcs[] = { "civ_vehicleshop_1", "civ_vehicleshop_2", "civ_vehicleshop_3", "civ_vehicleshop_4", "civ_vehicleshop_5", "civ_vehicleshop_6" };
        radius = 8;
        vehicleSpawns[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
    };

    class civ_air
    {
        type = "vehicles";
        shop = "civ_air";
        markers[] = {};
        npcs[] = { "civ_airshop_1", "civ_airshop_2", "civ_airshop_3", "civ_airshop_4", "civ_airshop_5", "civ_airshop_6" };
        radius = 8;
        vehicleSpawns[] = { "spawn_civ_athira", "spawn_civ_pyrgos" };
    };

    class civ_boats
    {
        type = "vehicles";
        shop = "civ_ship";
        markers[] = {};
        npcs[] = { "civ_boatshop_1", "civ_boatshop_2", "civ_boatshop_3", "civ_boatshop_4", "civ_boatshop_5", "civ_boatshop_6" };
        radius = 8;
        vehicleSpawns[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
    };

    class cop_market
    {
        type = "market";
        shop = "cop";
        markers[] = {};
        npcs[] = { "police_receptionist_1", "police_receptionist_2", "police_receptionist_3", "police_receptionist_4", "police_receptionist_5", "police_receptionist_6" };
        radius = 8;
    };

    class cop_supplies
    {
        type = "weapons";
        shop = "cop_kit";
        markers[] = {};
        npcs[] = { "police_receptionist_1", "police_receptionist_2", "police_receptionist_3", "police_receptionist_4", "police_receptionist_5", "police_receptionist_6" };
        radius = 8;
    };

    class cop_armoury
    {
        type = "weapons";
        shop = "cop";
        markers[] = {};
        npcs[] = { "police_receptionist_1", "police_receptionist_2", "police_receptionist_3", "police_receptionist_4", "police_receptionist_5", "police_receptionist_6" };
        radius = 8;
    };

    class cop_clothing
    {
        type = "clothing";
        shop = "cop";
        markers[] = {};
        npcs[] = { "police_receptionist_1", "police_receptionist_2", "police_receptionist_3", "police_receptionist_4", "police_receptionist_5", "police_receptionist_6" };
        radius = 8;
    };

    class cop_cars
    {
        type = "vehicles";
        shop = "cop_car";
        markers[] = {};
        npcs[] = { "police_shop_kavala", "police_shop_athira", "police_shop_pyrgos" };
        radius = 8;
        vehicleSpawns[] = { "police_car_spawn_kavala_1", "police_car_spawn_kavala_2", "police_car_spawn_kavala_3", "police_car_spawn_athira_1", "police_car_spawn_athira_2", "police_car_spawn_athira_3", "police_car_spawn_pyrgos_1", "police_car_spawn_pyrgos_2", "police_car_spawn_pyrgos_3" };
    };

    class cop_air
    {
        type = "vehicles";
        shop = "cop_air";
        markers[] = {};
        npcs[] = { "police_air_kavala", "police_air_athira" };
        radius = 8;
        vehicleSpawns[] = { "police_air_spawn_kavala_1", "police_air_spawn_kavala_2", "police_air_spawn_athira_1", "police_air_spawn_athira_2" };
    };

    class cop_boats
    {
        type = "vehicles";
        shop = "cop_ship";
        markers[] = {};
        npcs[] = { "police_ship_kavala", "police_ship_athira", "police_ship_pyrgos" };
        radius = 8;
        vehicleSpawns[] = { "police_boat_spawn_kavala_1", "police_boat_spawn_kavala_2", "police_boat_spawn_athira_1", "police_boat_spawn_athira_2", "police_boat_spawn_pyrgos_1", "police_boat_spawn_pyrgos_2" };
    };

    class medic_market
    {
        type = "market";
        shop = "medic";
        markers[] = {};
        npcs[] = { "medic_receptionist_1", "medic_receptionist_2", "medic_receptionist_3", "medic_receptionist_4", "medic_receptionist_5", "medic_receptionist_6" };
        radius = 8;
    };

    class medic_equipment
    {
        type = "weapons";
        shop = "medic_kit";
        markers[] = {};
        npcs[] = { "medic_receptionist_1", "medic_receptionist_2", "medic_receptionist_3", "medic_receptionist_4", "medic_receptionist_5", "medic_receptionist_6" };
        radius = 8;
    };

    class medic_clothing
    {
        type = "clothing";
        shop = "medic";
        markers[] = {};
        npcs[] = { "medic_receptionist_1", "medic_receptionist_2", "medic_receptionist_3", "medic_receptionist_4", "medic_receptionist_5", "medic_receptionist_6" };
        radius = 8;
    };

    class medic_vehicles
    {
        type = "vehicles";
        shop = "med_car";
        markers[] = {};
        npcs[] = { "medic_receptionist_1", "medic_receptionist_2", "medic_receptionist_3", "medic_receptionist_4", "medic_receptionist_5", "medic_receptionist_6" };
        radius = 8;
        vehicleSpawns[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
    };

    class medic_air
    {
        type = "vehicles";
        shop = "med_air";
        markers[] = {};
        npcs[] = { "medic_receptionist_1", "medic_receptionist_2", "medic_receptionist_3", "medic_receptionist_4", "medic_receptionist_5", "medic_receptionist_6" };
        radius = 8;
        vehicleSpawns[] = { "spawn_medic_kavala", "spawn_medic_athira" };
    };
};
