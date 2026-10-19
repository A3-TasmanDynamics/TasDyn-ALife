//
//   name     -- shown in the shop window
//   side     -- "civ" / "cop" / "medic": who can use it
//   licence  -- needed to use the shop at all (CfgLicences key, "" = none)
//   level    -- minimum cop_level / medic_level to use it (0 = any)
//   vehicles[] -- rows: {classname, "name" ("" = Arma's own), buy price,
//                 sell price, "licence" ("" = none), level (0 = any)}
//
// Where each shop is, and where bought vehicles appear: CfgShops.

class CarShops
{
    class rebel
    {
        name = "Rebel Vehicles";
        side = "civ";
        licence = "rebel";
        level = 0;
        vehicles[] = {
            {"B_Quadbike_01_F"           , "", 2500, 1250, "", 0},  // Quad Bike
            {"B_G_Offroad_01_F"          , "", 15000, 7500, "", 0},  // Offroad
            {"O_MRAP_02_F"               , "", 300000, 150000, "", 0},  // Ifrit
            {"B_Heli_Light_01_stripped_F", "", 325000, 162500, "pilot", 0}  // MH-9 Hummingbird (Stripped)
        };
    };

    class civ_car
    {
        name = "Car Dealership";
        side = "civ";
        licence = "driver";
        level = 0;
        vehicles[] = {
            {"C_Quadbike_01_F"       , "", 2500, 1250, "", 0},  // Quad Bike
            {"C_Hatchback_01_F"      , "", 9500, 4750, "", 0},  // Hatchback
            {"C_Offroad_01_F"        , "", 12500, 6250, "", 0},  // Offroad
            {"C_Offroad_02_unarmed_F", "", 15000, 7500, "", 0},  // MB 4WD -- DLC: Apex
            {"C_SUV_01_F"            , "", 30000, 15000, "", 0},  // SUV
            {"C_Hatchback_01_sport_F", "", 40000, 20000, "", 0}  // Hatchback (Sport)
        };
    };

    class civ_truck
    {
        name = "Truck Dealership";
        side = "civ";
        licence = "truck";
        level = 0;
        vehicles[] = {
            {"C_Van_01_transport_F"  , "", 45000, 22500, "", 0},  // Truck
            {"C_Van_02_transport_F"  , "", 50000, 25000, "", 0},  // Van Transport -- DLC: Laws of War
            {"C_Van_01_box_F"        , "", 60000, 30000, "", 0},  // Truck Boxer
            {"C_Van_02_vehicle_F"    , "", 65000, 32500, "", 0},  // Van (Cargo) -- DLC: Laws of War
            {"C_Truck_02_transport_F", "", 75000, 37500, "", 0},  // Zamak Transport -- Zamak
            {"C_Truck_02_covered_F"  , "", 100000, 50000, "", 0},  // Zamak Transport (Covered)
            {"C_Van_01_fuel_F"       , "", 80000, 40000, "", 0},  // Fuel Truck
            {"C_Truck_02_fuel_F"     , "", 120000, 60000, "", 0},  // Zamak Fuel
            {"O_Truck_03_transport_F", "", 200000, 100000, "", 0},  // Tempest Transport -- Tempest
            {"O_Truck_03_covered_F"  , "", 250000, 125000, "", 0},  // Tempest Transport (Covered)
            {"B_Truck_01_box_F"      , "", 350000, 175000, "", 0},  // HEMTT box
            {"O_Truck_03_device_F"   , "", 450000, 225000, "", 0}  // Tempest (Device)
        };
    };

    class civ_air
    {
        name = "Aircraft Dealership";
        side = "civ";
        licence = "pilot";
        level = 0;
        vehicles[] = {
            {"C_Plane_Civil_01_F"       , "", 150000, 75000, "", 0},  // DLC: Apex (Caesar BTT)
            {"C_Heli_Light_01_civil_F"  , "", 250000, 125000, "", 0},  // M-900
            {"B_Heli_Light_01_F"        , "", 275000, 137500, "", 0},  // MH-9 Hummingbird -- Hummingbird
            {"O_Heli_Light_02_unarmed_F", "", 750000, 375000, "", 0},  // PO-30 Orca (Unarmed) -- Orca
            {"I_Heli_Transport_02_F"    , "", 1000000, 500000, "", 0}  // CH-49 Mohawk -- Mohawk
        };
    };

    class civ_ship
    {
        name = "Boat Shop";
        side = "civ";
        licence = "boat";
        level = 0;
        vehicles[] = {
            {"C_Scooter_Transport_01_F", "", 2500, 1250, "", 0},  // DLC: Apex (water scooter)
            {"C_Rubberboat"            , "", 5000, 2500, "", 0},  // Rescue Boat
            {"C_Boat_Civil_01_F"       , "", 22000, 11000, "", 0},  // Motorboat
            {"C_Boat_Transport_02_F"   , "", 30000, 15000, "", 0},  // DLC: Apex (RHIB)
            {"B_SDV_01_F"              , "", 150000, 75000, "dive", 0}  // SDV
        };
    };

    class cop_car
    {
        name = "Police Garage";
        side = "cop";
        licence = "";
        level = 0;
        vehicles[] = {
            {"C_Offroad_01_F"        , "", 5000, 2500, "", 0},  // Offroad
            {"B_GEN_Offroad_01_gen_F", "", 7500, 3750, "", 1},  // DLC: Apex (Gendarmerie offroad)
            {"C_SUV_01_F"            , "", 20000, 10000, "", 1},  // SUV
            {"C_Hatchback_01_sport_F", "", 25000, 12500, "", 2},  // Hatchback (Sport)
            {"B_MRAP_01_F"           , "", 30000, 15000, "", 3},  // Hunter
            {"I_MRAP_03_F"           , "", 45000, 22500, "", 5}   // Strider
        };
    };

    class cop_air
    {
        name = "Police Air Unit";
        side = "cop";
        licence = "cAir";
        level = 2;
        vehicles[] = {
            {"B_Heli_Light_01_F"    , "", 75000, 37500, "", 0},  // MH-9 Hummingbird -- Hummingbird
            {"B_Heli_Transport_01_F", "", 200000, 100000, "", 4}  // UH-80 Ghost Hawk -- Ghost Hawk
        };
    };

    class cop_ship
    {
        name = "Coast Guard";
        side = "cop";
        licence = "cg";
        level = 0;
        vehicles[] = {
            {"B_Boat_Transport_01_F"   , "", 3000, 1500, "", 0},  // Assault Boat
            {"C_Boat_Civil_01_police_F", "", 20000, 10000, "", 0},  // Motorboat (Police)
            {"B_SDV_01_F"              , "", 100000, 50000, "", 3}  // SDV
        };
    };

    class med_car
    {
        name = "EMS Garage";
        side = "medic";
        licence = "";
        level = 0;
        vehicles[] = {
            {"C_Offroad_01_F"      , "", 5000, 2500, "", 0},  // Offroad
            {"C_SUV_01_F"          , "", 10000, 5000, "", 1},  // SUV
            {"C_Van_02_medevac_F"  , "", 15000, 7500, "", 1},  // Van (Ambulance) -- DLC: Laws of War (ambulance)
            {"O_Truck_03_medical_F", "", 40000, 20000, "", 3}   // Tempest medical
        };
    };

    class med_air
    {
        name = "Air Ambulance";
        side = "medic";
        licence = "mAir";
        level = 0;
        vehicles[] = {
            {"C_Heli_Light_01_civil_F"  , "", 50000, 25000, "", 0},  // M-900
            {"O_Heli_Light_02_unarmed_F", "", 150000, 75000, "", 3}  // PO-30 Orca (Unarmed) -- Orca
        };
    };
};

// Each vehicle's own skins, Tonic style: textures[] rows
//   {"name", "side" ("" = anyone), {texture per hidden selection, ...}}
// A buyer sees the rows for their side; with none, the vehicle's Arma
// colours (TextureSources). Placeholder police / EMS skins are recoloured
// vanilla (tools/textures/make_placeholders.py): drop the real PAAs in at
// the same paths under textures\vehicles\.
//
// Each vehicle's trunk space, in CfgItems weight units. A vehicle not
// listed here (map vehicles, admin-spawned ones) gets TrunkDefaults for its
// type.
// Read with missionConfigFile >> "CfgVehicles" (the mission's own list, not
// Arma's configFile >> "CfgVehicles").

// The garage (at every vehicle shop: Windows -> Garage, ALife_fnc_garage).
class CfgGarage
{
    impoundRate = 0.1;      // release fee: this share of the vehicle's shop price...
    impoundMin = 250;       // ...but at least this (dollars, from the bank)
    storeRange = 30;        // metres: how close your vehicle must be to store it
};

class CfgVehicles
{
    // Trunk space by type, for vehicles without their own `trunk` below.
    class TrunkDefaults
    {
        car = 50;
        truck = 150;
        air = 100;
        ship = 80;
        other = 30;
    };

    class B_Quadbike_01_F              { trunk = 25; };  // Quad Bike
    class B_G_Offroad_01_F             { trunk = 65; };  // Offroad
    class O_MRAP_02_F                  { trunk = 60; };  // Ifrit
    class B_Heli_Light_01_stripped_F   { trunk = 75; };  // MH-9 Hummingbird (Stripped)
    class C_Quadbike_01_F              { trunk = 25; };  // Quad Bike
    class C_Hatchback_01_F             { trunk = 40; };  // Hatchback
    class C_Offroad_01_F  // Offroad
    {
        trunk = 65;
        textures[] = {
            {"Police", "cop", { "textures\vehicles\police_offroad.paa", "textures\vehicles\police_offroad.paa" }},
            {"EMS", "medic", { "textures\vehicles\ems_offroad.paa", "textures\vehicles\ems_offroad.paa" }}
        };
    };
    class C_Offroad_02_unarmed_F       { trunk = 60; };  // MB 4WD
    class C_SUV_01_F  // SUV
    {
        trunk = 50;
        textures[] = {
            {"Police", "cop", { "textures\vehicles\police_suv.paa" }},
            {"EMS", "medic", { "textures\vehicles\ems_suv.paa" }}
        };
    };
    class C_Hatchback_01_sport_F  // Hatchback (Sport)
    {
        trunk = 40;
        textures[] = { {"Police", "cop", { "textures\vehicles\police_hatchback_sport.paa" }} };
    };
    class C_Van_01_transport_F         { trunk = 100; };  // Truck
    class C_Van_02_transport_F         { trunk = 110; };  // Van Transport
    class C_Van_01_box_F               { trunk = 150; };  // Truck Boxer
    class C_Van_02_vehicle_F           { trunk = 160; };  // Van (Cargo)
    class C_Truck_02_transport_F       { trunk = 250; };  // Zamak Transport
    class C_Truck_02_covered_F         { trunk = 300; };  // Zamak Transport (Covered)
    class C_Van_01_fuel_F              { trunk = 50; };  // Fuel Truck
    class C_Truck_02_fuel_F            { trunk = 100; };  // Zamak Fuel
    class O_Truck_03_transport_F       { trunk = 280; };  // Tempest Transport
    class O_Truck_03_covered_F         { trunk = 350; };  // Tempest Transport (Covered)
    class B_Truck_01_box_F             { trunk = 450; };  // HEMTT Box
    class O_Truck_03_device_F          { trunk = 300; };  // Tempest (Device)
    class C_Plane_Civil_01_F           { trunk = 50; };  // Caesar BTT
    class C_Heli_Light_01_civil_F  // M-900
    {
        trunk = 75;
        textures[] = { {"EMS", "medic", { "textures\vehicles\ems_m900.paa" }} };
    };
    class B_Heli_Light_01_F  // MH-9 Hummingbird
    {
        trunk = 85;
        textures[] = { {"Police", "cop", { "textures\vehicles\police_hummingbird.paa" }} };
    };
    class O_Heli_Light_02_unarmed_F  // PO-30 Orca (Unarmed)
    {
        trunk = 110;
        textures[] = { {"EMS", "medic", { "textures\vehicles\ems_orca.paa" }} };
    };
    class I_Heli_Transport_02_F        { trunk = 150; };  // CH-49 Mohawk
    class C_Scooter_Transport_01_F     { trunk = 20; };  // Water Scooter
    class C_Rubberboat                 { trunk = 45; };  // Rescue Boat
    class C_Boat_Civil_01_F            { trunk = 85; };  // Motorboat
    class C_Boat_Transport_02_F        { trunk = 100; };  // RHIB
    class B_SDV_01_F                   { trunk = 45; };  // SDV
    class B_GEN_Offroad_01_gen_F       { trunk = 65; };  // Offroad
    class B_MRAP_01_F                  { trunk = 60; };  // Hunter
    class I_MRAP_03_F                  { trunk = 60; };  // Strider
    class B_Heli_Transport_01_F        { trunk = 120; };  // UH-80 Ghost Hawk
    class B_Boat_Transport_01_F        { trunk = 45; };  // Assault Boat
    class C_Boat_Civil_01_police_F     { trunk = 85; };  // Motorboat (Police)
    class C_Van_02_medevac_F           { trunk = 80; };  // Van (Ambulance)
    class O_Truck_03_medical_F         { trunk = 150; };  // Tempest Medical
};
