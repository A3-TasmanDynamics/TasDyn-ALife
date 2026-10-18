// Vehicles sold in shops: one class per vehicle (its Arma classname), with
// what it costs and who may buy it, per faction, and its trunk space.
// Read with missionConfigFile >> "CfgVehicles" -- the mission's own list,
// not Arma's configFile >> "CfgVehicles".
//
//   trunk        -- virtual-item space (CfgItems weight units); a vehicle
//                   not listed here falls back to CfgALife >> Trunks by type
//   class civ / cop / medic -- that faction can buy it:
//     price      -- dollars
//     licence    -- needed on top of the shop's (CfgLicences key)
//     level      -- minimum cop_level / medic_level
//
// Which shop sells what is in CfgShops (vehicles[]).

class CfgVehicles
{
    class C_Quadbike_01_F
    {
        trunk = 25;
        class civ   { price = 2500; };
    };
    class C_Hatchback_01_F
    {
        trunk = 40;
        class civ   { price = 9500; };
    };
    class C_Offroad_01_F
    {
        trunk = 65;
        class civ   { price = 12500; };
        class cop   { price = 5000; };
        class medic { price = 5000; };
    };
    class C_Offroad_02_unarmed_F  // DLC: Apex
    {
        trunk = 60;
        class civ   { price = 15000; };
    };
    class C_SUV_01_F
    {
        trunk = 50;
        class civ   { price = 30000; };
        class cop   { price = 20000; level = 1; };
        class medic { price = 10000; level = 1; };
    };
    class C_Hatchback_01_sport_F
    {
        trunk = 40;
        class civ   { price = 40000; };
        class cop   { price = 25000; level = 2; };
    };
    class C_Van_01_transport_F
    {
        trunk = 100;
        class civ   { price = 45000; };
    };
    class C_Van_02_transport_F  // DLC: Laws of War
    {
        trunk = 110;
        class civ   { price = 50000; };
    };
    class C_Van_01_box_F
    {
        trunk = 150;
        class civ   { price = 60000; };
    };
    class C_Van_02_vehicle_F  // DLC: Laws of War
    {
        trunk = 160;
        class civ   { price = 65000; };
    };
    class C_Truck_02_transport_F  // Zamak
    {
        trunk = 250;
        class civ   { price = 75000; };
    };
    class C_Truck_02_covered_F
    {
        trunk = 300;
        class civ   { price = 100000; };
    };
    class C_Van_01_fuel_F
    {
        trunk = 50;
        class civ   { price = 80000; };
    };
    class C_Truck_02_fuel_F
    {
        trunk = 100;
        class civ   { price = 120000; };
    };
    class O_Truck_03_transport_F  // Tempest
    {
        trunk = 280;
        class civ   { price = 200000; };
    };
    class O_Truck_03_covered_F
    {
        trunk = 350;
        class civ   { price = 250000; };
    };
    class B_Truck_01_box_F  // HEMTT box
    {
        trunk = 450;
        class civ   { price = 350000; };
    };
    class O_Truck_03_device_F
    {
        trunk = 300;
        class civ   { price = 450000; };
    };
    class C_Plane_Civil_01_F
    {
        trunk = 50;
        class civ   { price = 150000; };
    };
    class C_Heli_Light_01_civil_F  // M-900
    {
        trunk = 75;
        class civ   { price = 250000; };
        class medic { price = 50000; };
    };
    class B_Heli_Light_01_F  // Hummingbird
    {
        trunk = 85;
        class civ   { price = 275000; };
        class cop   { price = 75000; };
    };
    class O_Heli_Light_02_unarmed_F  // Orca
    {
        trunk = 110;
        class civ   { price = 750000; };
        class medic { price = 150000; level = 3; };
    };
    class I_Heli_Transport_02_F  // Mohawk
    {
        trunk = 150;
        class civ   { price = 1000000; };
    };
    class C_Scooter_Transport_01_F
    {
        trunk = 20;
        class civ   { price = 2500; };
    };
    class C_Rubberboat
    {
        trunk = 45;
        class civ   { price = 5000; };
    };
    class C_Boat_Civil_01_F  // Motorboat
    {
        trunk = 85;
        class civ   { price = 22000; };
    };
    class C_Boat_Transport_02_F  // DLC: Apex (RHIB)
    {
        trunk = 100;
        class civ   { price = 30000; };
    };
    class B_SDV_01_F
    {
        trunk = 45;
        class civ   { price = 150000; licence = "dive"; };
        class cop   { price = 100000; level = 3; };
    };
    class B_Quadbike_01_F
    {
        trunk = 25;
        class civ   { price = 2500; };
    };
    class B_G_Offroad_01_F
    {
        trunk = 65;
        class civ   { price = 15000; };
    };
    class O_MRAP_02_F  // Ifrit
    {
        trunk = 60;
        class civ   { price = 300000; };
    };
    class B_Heli_Light_01_stripped_F
    {
        trunk = 75;
        class civ   { price = 325000; licence = "pilot"; };
    };
    class B_GEN_Offroad_01_gen_F  // DLC: Apex (Gendarmerie offroad)
    {
        trunk = 65;
        class cop   { price = 7500; level = 1; };
    };
    class B_MRAP_01_F  // Hunter
    {
        trunk = 60;
        class cop   { price = 30000; level = 3; };
    };
    class I_MRAP_03_F  // Strider
    {
        trunk = 60;
        class cop   { price = 45000; level = 5; };
    };
    class B_Heli_Transport_01_F  // Ghost Hawk
    {
        trunk = 120;
        class cop   { price = 200000; level = 4; };
    };
    class B_Boat_Transport_01_F
    {
        trunk = 45;
        class cop   { price = 3000; };
    };
    class C_Boat_Civil_01_police_F
    {
        trunk = 85;
        class cop   { price = 20000; };
    };
    class C_Van_02_medevac_F  // DLC: Laws of War (ambulance)
    {
        trunk = 80;
        class medic { price = 15000; level = 1; };
    };
    class O_Truck_03_medical_F  // Tempest medical
    {
        trunk = 150;
        class medic { price = 40000; level = 3; };
    };
};
