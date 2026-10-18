// Vehicle shops, in Tonic's row format (Config_Vehicles) with our licence /
// rank fields instead of condition strings. Each shop:
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
            {"B_Quadbike_01_F"           , "", 2500, 1250, "", 0},
            {"B_G_Offroad_01_F"          , "", 15000, 7500, "", 0},
            {"O_MRAP_02_F"               , "", 300000, 150000, "", 0},  // Ifrit
            {"B_Heli_Light_01_stripped_F", "", 325000, 162500, "pilot", 0}
        };
    };

    class civ_car
    {
        name = "Car Dealership";
        side = "civ";
        licence = "driver";
        level = 0;
        vehicles[] = {
            {"C_Quadbike_01_F"       , "", 2500, 1250, "", 0},
            {"C_Hatchback_01_F"      , "", 9500, 4750, "", 0},
            {"C_Offroad_01_F"        , "", 12500, 6250, "", 0},
            {"C_Offroad_02_unarmed_F", "", 15000, 7500, "", 0},  // DLC: Apex
            {"C_SUV_01_F"            , "", 30000, 15000, "", 0},
            {"C_Hatchback_01_sport_F", "", 40000, 20000, "", 0}
        };
    };

    class civ_truck
    {
        name = "Truck Dealership";
        side = "civ";
        licence = "truck";
        level = 0;
        vehicles[] = {
            {"C_Van_01_transport_F"  , "", 45000, 22500, "", 0},
            {"C_Van_02_transport_F"  , "", 50000, 25000, "", 0},  // DLC: Laws of War
            {"C_Van_01_box_F"        , "", 60000, 30000, "", 0},
            {"C_Van_02_vehicle_F"    , "", 65000, 32500, "", 0},  // DLC: Laws of War
            {"C_Truck_02_transport_F", "", 75000, 37500, "", 0},  // Zamak
            {"C_Truck_02_covered_F"  , "", 100000, 50000, "", 0},
            {"C_Van_01_fuel_F"       , "", 80000, 40000, "", 0},
            {"C_Truck_02_fuel_F"     , "", 120000, 60000, "", 0},
            {"O_Truck_03_transport_F", "", 200000, 100000, "", 0},  // Tempest
            {"O_Truck_03_covered_F"  , "", 250000, 125000, "", 0},
            {"B_Truck_01_box_F"      , "", 350000, 175000, "", 0},  // HEMTT box
            {"O_Truck_03_device_F"   , "", 450000, 225000, "", 0}
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
            {"B_Heli_Light_01_F"        , "", 275000, 137500, "", 0},  // Hummingbird
            {"O_Heli_Light_02_unarmed_F", "", 750000, 375000, "", 0},  // Orca
            {"I_Heli_Transport_02_F"    , "", 1000000, 500000, "", 0}   // Mohawk
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
            {"C_Rubberboat"            , "", 5000, 2500, "", 0},
            {"C_Boat_Civil_01_F"       , "", 22000, 11000, "", 0},  // Motorboat
            {"C_Boat_Transport_02_F"   , "", 30000, 15000, "", 0},  // DLC: Apex (RHIB)
            {"B_SDV_01_F"              , "", 150000, 75000, "dive", 0}
        };
    };

    class cop_car
    {
        name = "Police Garage";
        side = "cop";
        licence = "";
        level = 0;
        vehicles[] = {
            {"C_Offroad_01_F"        , "", 5000, 2500, "", 0},
            {"B_GEN_Offroad_01_gen_F", "", 7500, 3750, "", 1},  // DLC: Apex (Gendarmerie offroad)
            {"C_SUV_01_F"            , "", 20000, 10000, "", 1},
            {"C_Hatchback_01_sport_F", "", 25000, 12500, "", 2},
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
            {"B_Heli_Light_01_F"    , "", 75000, 37500, "", 0},  // Hummingbird
            {"B_Heli_Transport_01_F", "", 200000, 100000, "", 4}   // Ghost Hawk
        };
    };

    class cop_ship
    {
        name = "Coast Guard";
        side = "cop";
        licence = "cg";
        level = 0;
        vehicles[] = {
            {"B_Boat_Transport_01_F"   , "", 3000, 1500, "", 0},
            {"C_Boat_Civil_01_police_F", "", 20000, 10000, "", 0},
            {"B_SDV_01_F"              , "", 100000, 50000, "", 3}
        };
    };

    class med_car
    {
        name = "EMS Garage";
        side = "medic";
        licence = "";
        level = 0;
        vehicles[] = {
            {"C_Offroad_01_F"      , "", 5000, 2500, "", 0},
            {"C_SUV_01_F"          , "", 10000, 5000, "", 1},
            {"C_Van_02_medevac_F"  , "", 15000, 7500, "", 1},  // DLC: Laws of War (ambulance)
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
            {"O_Heli_Light_02_unarmed_F", "", 150000, 75000, "", 3}   // Orca
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
// Each vehicle's trunk space (Tonic's vItemSpace), in CfgItems weight units.
// A vehicle not listed falls back to CfgALife >> Trunks by type.
// Read with missionConfigFile >> "CfgVehicles" (the mission's own list, not
// Arma's configFile >> "CfgVehicles").

class CfgVehicles
{
    class B_Quadbike_01_F              { trunk = 25; };
    class B_G_Offroad_01_F             { trunk = 65; };
    class O_MRAP_02_F                  { trunk = 60; };
    class B_Heli_Light_01_stripped_F   { trunk = 75; };
    class C_Quadbike_01_F              { trunk = 25; };
    class C_Hatchback_01_F             { trunk = 40; };
    class C_Offroad_01_F
    {
        trunk = 65;
        textures[] = {
            {"Police", "cop", { "textures\vehicles\police_offroad.paa", "textures\vehicles\police_offroad.paa" }},
            {"EMS", "medic", { "textures\vehicles\ems_offroad.paa", "textures\vehicles\ems_offroad.paa" }}
        };
    };
    class C_Offroad_02_unarmed_F       { trunk = 60; };
    class C_SUV_01_F
    {
        trunk = 50;
        textures[] = {
            {"Police", "cop", { "textures\vehicles\police_suv.paa" }},
            {"EMS", "medic", { "textures\vehicles\ems_suv.paa" }}
        };
    };
    class C_Hatchback_01_sport_F
    {
        trunk = 40;
        textures[] = { {"Police", "cop", { "textures\vehicles\police_hatchback_sport.paa" }} };
    };
    class C_Van_01_transport_F         { trunk = 100; };
    class C_Van_02_transport_F         { trunk = 110; };
    class C_Van_01_box_F               { trunk = 150; };
    class C_Van_02_vehicle_F           { trunk = 160; };
    class C_Truck_02_transport_F       { trunk = 250; };
    class C_Truck_02_covered_F         { trunk = 300; };
    class C_Van_01_fuel_F              { trunk = 50; };
    class C_Truck_02_fuel_F            { trunk = 100; };
    class O_Truck_03_transport_F       { trunk = 280; };
    class O_Truck_03_covered_F         { trunk = 350; };
    class B_Truck_01_box_F             { trunk = 450; };
    class O_Truck_03_device_F          { trunk = 300; };
    class C_Plane_Civil_01_F           { trunk = 50; };
    class C_Heli_Light_01_civil_F
    {
        trunk = 75;
        textures[] = { {"EMS", "medic", { "textures\vehicles\ems_m900.paa" }} };
    };
    class B_Heli_Light_01_F
    {
        trunk = 85;
        textures[] = { {"Police", "cop", { "textures\vehicles\police_hummingbird.paa" }} };
    };
    class O_Heli_Light_02_unarmed_F
    {
        trunk = 110;
        textures[] = { {"EMS", "medic", { "textures\vehicles\ems_orca.paa" }} };
    };
    class I_Heli_Transport_02_F        { trunk = 150; };
    class C_Scooter_Transport_01_F     { trunk = 20; };
    class C_Rubberboat                 { trunk = 45; };
    class C_Boat_Civil_01_F            { trunk = 85; };
    class C_Boat_Transport_02_F        { trunk = 100; };
    class B_SDV_01_F                   { trunk = 45; };
    class B_GEN_Offroad_01_gen_F       { trunk = 65; };
    class B_MRAP_01_F                  { trunk = 60; };
    class I_MRAP_03_F                  { trunk = 60; };
    class B_Heli_Transport_01_F        { trunk = 120; };
    class B_Boat_Transport_01_F        { trunk = 45; };
    class C_Boat_Civil_01_police_F     { trunk = 85; };
    class C_Van_02_medevac_F           { trunk = 80; };
    class O_Truck_03_medical_F         { trunk = 150; };
};
