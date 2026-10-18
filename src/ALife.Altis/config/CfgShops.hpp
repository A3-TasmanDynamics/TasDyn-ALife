// Shops. A shop opens with a scroll-wheel action when you're within
// `radius` metres of one of its markers and playing its faction. Prices and
// requirements are checked by the server, never taken from the client.
//
//   displayName -- window title / action name
//   faction     -- "civ" / "cop" / "medic" (who can use it)
//   licence     -- needed to use the shop at all (CfgLicences key, "" = none)
//   level       -- minimum players.<faction>_level (cop / medic rank), 0 = any
//   markers     -- where it is (mission.sqm markers)
//   radius      -- metres from a marker
//
// Stock, any mix of these lists; entry class = classname, then
//   price       -- dollars each
//   licence     -- optional, needed for this entry on top of the shop's
//   level       -- optional, minimum rank for this entry
//
//   Items       -- virtual items (CfgItems.hpp)
//   Gear        -- Arma gear: weapons, magazines, attachments, uniforms,
//                  vests, headgear, glasses, backpacks, map/GPS/etc.
//   Vehicles    -- vehicles (CfgVehicles); appear at the shop's
//                  vehicleSpawns[] markers
//
// Everything is vanilla Arma 3 or official DLC (marked "DLC: <name>").
// Players without a DLC can buy its gear but can't drive its vehicles.
//
// Locations: shops sit on the town spawn markers for now; give each its own
// marker in Eden (and list it here) when the shops get placed properly.
// Only Items lists can be bought so far -- Gear and Vehicles are priced and
// ready for the next step (ALife_fnc_shopBuy / ALife_fnc_shop).

class CfgShops
{
    // =====================================================================
    //  Civilian
    // =====================================================================

    class general_store
    {
        displayName = "General Store";
        faction = "civ";
        licence = "";
        level = 0;
        markers[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
        radius = 30;
        class Items
        {
            class water     { price = 10; };
            class coffee    { price = 10; };
            class apple     { price = 50; };
            class tbacon    { price = 75; };
            class redgull   { price = 1500; };
            class pickaxe   { price = 750; };
            class fuelcan   { price = 850; };
            class lockpick  { price = 150; };
            class repairkit { price = 350; };
        };
        class Gear
        {
            class ItemMap      { price = 50; };
            class ItemCompass  { price = 50; };
            class ItemWatch    { price = 50; };
            class ItemRadio    { price = 100; };
            class ItemGPS      { price = 100; };
            class Binocular    { price = 150; };
            class FirstAidKit  { price = 150; };
            class Chemlight_green { price = 30; };
            class NVGoggles    { price = 2000; };
        };
    };

    class civ_clothing
    {
        displayName = "Clothing Store";
        faction = "civ";
        licence = "";
        level = 0;
        markers[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
        radius = 30;
        class Gear
        {
            // Uniforms
            class U_C_Poloshirt_blue       { price = 250; };
            class U_C_Poloshirt_burgundy   { price = 275; };
            class U_C_Poloshirt_redwhite   { price = 150; };
            class U_C_Poloshirt_salmon     { price = 175; };
            class U_C_Poloshirt_stripped   { price = 125; };
            class U_C_Poloshirt_tricolour  { price = 350; };
            class U_C_Commoner1_1          { price = 200; };
            class U_C_WorkerCoveralls      { price = 450; };
            class U_C_HunterBody_grn       { price = 1500; };
            class U_C_Journalist           { price = 1200; };
            class U_OrestesBody            { price = 1100; };
            class U_Rangemaster            { price = 1250; };
            class U_C_Scientist            { price = 900; };
            class U_C_man_sport_1_F        { price = 500; };  // DLC: Apex
            class U_C_Man_casual_1_F       { price = 600; };  // DLC: Apex
            class U_C_Mechanic_01_F        { price = 700; };  // DLC: Laws of War
            // Headgear
            class H_Cap_red                { price = 100; };
            class H_Cap_blu                { price = 100; };
            class H_Cap_grn                { price = 100; };
            class H_Cap_blk                { price = 100; };
            class H_Bandanna_gry           { price = 150; };
            class H_Hat_brown              { price = 250; };
            class H_Hat_tan                { price = 250; };
            class H_StrawHat               { price = 225; };
            class H_Booniehat_tan          { price = 300; };
            // Glasses
            class G_Shades_Black           { price = 50; };
            class G_Shades_Blue            { price = 50; };
            class G_Sport_Blackred         { price = 75; };
            class G_Squares                { price = 50; };
            class G_Aviator                { price = 100; };
            class G_Spectacles             { price = 50; };
            // Backpacks
            class B_AssaultPack_cbr        { price = 2500; };
            class B_AssaultPack_blk        { price = 2500; };
            class B_FieldPack_cbr          { price = 3000; };
            class B_Kitbag_cbr             { price = 4500; };
            class B_Carryall_cbr           { price = 6500; };
        };
    };

    class gun_store
    {
        displayName = "Gun Store";
        faction = "civ";
        licence = "gun";
        level = 0;
        markers[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos" };
        radius = 30;
        class Gear
        {
            class hgun_P07_F               { price = 1500; };
            class hgun_Rook40_F            { price = 2000; };
            class hgun_ACPC2_F             { price = 3500; };
            class hgun_Pistol_heavy_02_F   { price = 4500; };   // Zubr
            class hgun_Pistol_heavy_01_F   { price = 6000; };   // 4-five
            class hgun_PDW2000_F           { price = 15000; };
            class 16Rnd_9x21_Mag           { price = 25; };
            class 9Rnd_45ACP_Mag           { price = 45; };
            class 6Rnd_45ACP_Cylinder      { price = 50; };
            class 11Rnd_45ACP_Mag          { price = 60; };
            class 30Rnd_9x21_Mag           { price = 75; };
            class optic_Yorris             { price = 1000; };
            class optic_MRD                { price = 1000; };
            class optic_ACO_grn_smg        { price = 1500; };
            class acc_flashlight_pistol    { price = 500; };
        };
    };

    class rebel_market
    {
        displayName = "Rebel Outpost";
        faction = "civ";
        licence = "rebel";
        level = 0;
        markers[] = { "spawn_civ_sofia" };
        radius = 30;
        class Items
        {
            class water     { price = 10; };
            class tbacon    { price = 75; };
            class lockpick  { price = 150; };
            class zipties   { price = 500; };
            class repairkit { price = 350; };
            class fuelcan   { price = 850; };
        };
        class Gear
        {
            // Weapons
            class arifle_AKS_F             { price = 22000; };  // DLC: Apex
            class arifle_TRG20_F           { price = 25000; };
            class arifle_TRG21_F           { price = 30000; };
            class arifle_Mk20_F            { price = 30000; };
            class arifle_Katiba_F          { price = 35000; };
            class arifle_AKM_F             { price = 40000; };  // DLC: Apex
            class srifle_DMR_01_F          { price = 50000; };  // Rahim
            class srifle_EBR_F             { price = 60000; };  // Mk18 ABR
            class 30Rnd_545x39_Mag_F       { price = 150; };
            class 30Rnd_556x45_Stanag      { price = 125; };
            class 30Rnd_65x39_caseless_green { price = 275; };
            class 30Rnd_762x39_Mag_F       { price = 300; };
            class 10Rnd_762x54_Mag         { price = 500; };
            class 20Rnd_762x51_Mag         { price = 500; };
            class optic_ACO_grn            { price = 3500; };
            class optic_Holosight          { price = 3600; };
            class optic_MRCO               { price = 7500; };
            class optic_Arco               { price = 8000; };
            class optic_Hamr               { price = 8500; };
            class acc_flashlight           { price = 1000; };
            // Clothing
            class U_IG_Guerilla1_1         { price = 5000; };
            class U_IG_Guerilla2_1         { price = 5500; };
            class U_IG_Guerilla3_1         { price = 5500; };
            class U_I_G_resistanceLeader_F { price = 11500; };
            class U_I_G_Story_Protagonist_F { price = 7500; };
            class U_O_SpecopsUniform_ocamo { price = 17500; };
            class V_BandollierB_cbr        { price = 4500; };
            class V_HarnessO_brn           { price = 7500; };
            class V_TacVest_khk            { price = 12500; };
            class V_PlateCarrierIA1_dgtl   { price = 25000; };
            class H_Shemag_olive           { price = 850; };
            class H_ShemagOpen_khk         { price = 800; };
            class H_Bandanna_khk           { price = 500; };
            class H_Booniehat_khk          { price = 650; };
            class G_Balaclava_blk          { price = 2000; };
            class G_Bandanna_blk           { price = 1500; };
            class B_FieldPack_oli          { price = 3000; };
            class B_Kitbag_cbr             { price = 4500; };
            class B_Carryall_oli           { price = 6500; };
        };
        class Vehicles
        {
            class B_Quadbike_01_F          { price = 2500; };
            class B_G_Offroad_01_F         { price = 15000; };
            class O_MRAP_02_F              { price = 300000; };  // Ifrit
            class B_Heli_Light_01_stripped_F { price = 325000; licence = "pilot"; };
        };
        vehicleSpawns[] = { "spawn_civ_sofia" };
    };

    // ---- Civilian vehicles ----

    class civ_cars
    {
        displayName = "Car Dealership";
        faction = "civ";
        licence = "driver";
        level = 0;
        markers[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
        radius = 30;
        class Vehicles
        {
            class C_Quadbike_01_F          { price = 2500; };
            class C_Hatchback_01_F         { price = 9500; };
            class C_Offroad_01_F           { price = 12500; };
            class C_Offroad_02_unarmed_F   { price = 15000; };  // DLC: Apex
            class C_SUV_01_F               { price = 30000; };
            class C_Hatchback_01_sport_F   { price = 40000; };
        };
        vehicleSpawns[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
    };

    class civ_trucks
    {
        displayName = "Truck Dealership";
        faction = "civ";
        licence = "truck";
        level = 0;
        markers[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
        radius = 30;
        class Vehicles
        {
            class C_Van_01_transport_F     { price = 45000; };
            class C_Van_02_transport_F     { price = 50000; };  // DLC: Laws of War
            class C_Van_01_box_F           { price = 60000; };
            class C_Van_02_vehicle_F       { price = 65000; };  // DLC: Laws of War
            class C_Truck_02_transport_F   { price = 75000; };  // Zamak
            class C_Truck_02_covered_F     { price = 100000; };
            class C_Van_01_fuel_F          { price = 80000; };
            class C_Truck_02_fuel_F        { price = 120000; };
            class O_Truck_03_transport_F   { price = 200000; }; // Tempest
            class O_Truck_03_covered_F     { price = 250000; };
            class B_Truck_01_box_F         { price = 350000; }; // HEMTT box
            class O_Truck_03_device_F      { price = 450000; };
        };
        vehicleSpawns[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
    };

    class civ_air
    {
        displayName = "Aircraft Dealership";
        faction = "civ";
        licence = "pilot";
        level = 0;
        markers[] = { "spawn_civ_athira", "spawn_civ_pyrgos" };
        radius = 30;
        class Vehicles
        {
            class C_Plane_Civil_01_F       { price = 150000; }; // DLC: Apex (Caesar BTT)
            class C_Heli_Light_01_civil_F  { price = 250000; }; // M-900
            class B_Heli_Light_01_F        { price = 275000; }; // Hummingbird
            class O_Heli_Light_02_unarmed_F { price = 750000; }; // Orca
            class I_Heli_Transport_02_F    { price = 1000000; }; // Mohawk
        };
        vehicleSpawns[] = { "spawn_civ_athira", "spawn_civ_pyrgos" };
    };

    class civ_boats
    {
        displayName = "Boat Shop";
        faction = "civ";
        licence = "boat";
        level = 0;
        markers[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
        radius = 30;
        class Vehicles
        {
            class C_Scooter_Transport_01_F { price = 2500; };   // DLC: Apex (water scooter)
            class C_Rubberboat             { price = 5000; };
            class C_Boat_Civil_01_F        { price = 22000; };  // Motorboat
            class C_Boat_Transport_02_F    { price = 30000; };  // DLC: Apex (RHIB)
            class B_SDV_01_F               { price = 150000; licence = "dive"; };
        };
        vehicleSpawns[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
    };

    // =====================================================================
    //  Police -- level = players.cop_level (0 = public slots, see CfgALife)
    // =====================================================================

    class cop_market
    {
        displayName = "Police Supplies";
        faction = "cop";
        licence = "";
        level = 0;
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
        radius = 30;
        class Items
        {
            class water      { price = 10; };
            class coffee     { price = 10; };
            class donut      { price = 25; };
            class repairkit  { price = 350; };
            class fuelcan    { price = 850; };
            class spikestrip { price = 2500; level = 1; };
        };
        class Gear
        {
            class ItemMap      { price = 50; };
            class ItemCompass  { price = 50; };
            class ItemWatch    { price = 50; };
            class ItemRadio    { price = 100; };
            class ItemGPS      { price = 100; };
            class Binocular    { price = 150; };
            class Rangefinder  { price = 500; level = 2; };
            class FirstAidKit  { price = 150; };
            class ToolKit      { price = 250; };
            class NVGoggles    { price = 1200; level = 1; };
        };
    };

    class cop_armoury
    {
        displayName = "Police Armoury";
        faction = "cop";
        licence = "";
        level = 0;
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
        radius = 30;
        class Gear
        {
            class hgun_P07_snds_F          { price = 2000; };             // taser (classic Altis Life)
            class hgun_P07_F               { price = 1500; };
            class SMG_05_F                 { price = 12000; level = 1; }; // DLC: Apex (Protector)
            class SMG_02_F                 { price = 15000; level = 2; }; // Sting
            class arifle_MXC_F             { price = 30000; level = 3; };
            class arifle_MX_F              { price = 35000; level = 4; };
            class arifle_MXM_F             { price = 45000; level = 5; };
            class 16Rnd_9x21_Mag           { price = 25; };
            class 30Rnd_9x21_Mag_SMG_02    { price = 75; level = 1; };
            class 30Rnd_65x39_caseless_mag { price = 130; level = 3; };
            class muzzle_snds_L            { price = 650; };
            class acc_flashlight           { price = 750; level = 1; };
            class acc_flashlight_pistol    { price = 500; };
            class optic_Holosight_blk_F    { price = 1200; level = 1; };
            class optic_MRCO               { price = 2500; level = 3; };
            class optic_Arco               { price = 2500; level = 3; };
            class optic_Hamr               { price = 3500; level = 4; };
        };
    };

    class cop_clothing
    {
        displayName = "Police Uniforms";
        faction = "cop";
        licence = "";
        level = 0;
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
        radius = 30;
        class Gear
        {
            class U_Rangemaster            { price = 25; };
            class U_B_GEN_Soldier_F        { price = 150; level = 1; };   // DLC: Apex (Gendarmerie)
            class U_B_GEN_Commander_F      { price = 250; level = 3; };   // DLC: Apex
            class V_Rangemaster_belt       { price = 800; };
            class V_TacVest_blk_POLICE     { price = 1500; level = 1; };
            class V_TacVest_gen_F          { price = 2000; level = 2; };  // DLC: Apex
            class V_PlateCarrier1_blk      { price = 3500; level = 4; };
            class H_Cap_police             { price = 25; };
            class H_MilCap_gen_F           { price = 50; level = 1; };    // DLC: Apex
            class H_Beret_gen_F            { price = 75; level = 3; };    // DLC: Apex
            class G_Aviator                { price = 50; };
            class G_Shades_Black           { price = 50; };
            class B_AssaultPack_blk        { price = 500; };
            class B_FieldPack_blk          { price = 1000; level = 1; };
            class B_Carryall_blk           { price = 1500; level = 3; };
        };
    };

    class cop_cars
    {
        displayName = "Police Garage";
        faction = "cop";
        licence = "";
        level = 0;
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
        radius = 30;
        class Vehicles
        {
            class C_Offroad_01_F           { price = 5000; };
            class B_GEN_Offroad_01_gen_F   { price = 7500; level = 1; };  // DLC: Apex (Gendarmerie offroad)
            class C_SUV_01_F               { price = 20000; level = 1; };
            class C_Hatchback_01_sport_F   { price = 25000; level = 2; };
            class B_MRAP_01_F              { price = 30000; level = 3; }; // Hunter
            class I_MRAP_03_F              { price = 45000; level = 5; }; // Strider
        };
        vehicleSpawns[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
    };

    class cop_air
    {
        displayName = "Police Air Unit";
        faction = "cop";
        licence = "cAir";
        level = 2;
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira" };
        radius = 30;
        class Vehicles
        {
            class B_Heli_Light_01_F        { price = 75000; };            // Hummingbird
            class B_Heli_Transport_01_F    { price = 200000; level = 4; };// Ghost Hawk
        };
        vehicleSpawns[] = { "spawn_cop_kavala", "spawn_cop_athira" };
    };

    class cop_boats
    {
        displayName = "Coast Guard";
        faction = "cop";
        licence = "cg";
        level = 0;
        markers[] = { "spawn_cop_kavala", "spawn_cop_pyrgos" };
        radius = 30;
        class Vehicles
        {
            class B_Boat_Transport_01_F    { price = 3000; };
            class C_Boat_Civil_01_police_F { price = 20000; };
            class B_SDV_01_F               { price = 100000; level = 3; };
        };
        vehicleSpawns[] = { "spawn_cop_kavala", "spawn_cop_pyrgos" };
    };

    // =====================================================================
    //  EMS -- level = players.medic_level
    // =====================================================================

    class medic_supplies
    {
        displayName = "Medical Supplies";
        faction = "medic";
        licence = "";
        level = 0;
        markers[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
        radius = 30;
        class Items
        {
            class medkit    { price = 250; };
            class water     { price = 10; };
            class coffee    { price = 10; };
            class repairkit { price = 350; };
            class fuelcan   { price = 850; };
        };
        class Gear
        {
            class ItemMap      { price = 50; };
            class ItemCompass  { price = 50; };
            class ItemWatch    { price = 50; };
            class ItemRadio    { price = 100; };
            class ItemGPS      { price = 100; };
            class Binocular    { price = 150; };
            class FirstAidKit  { price = 50; };
            class Medikit      { price = 500; };
            class ToolKit      { price = 250; };
            class Chemlight_green { price = 30; };
            class NVGoggles    { price = 1200; level = 1; };
        };
    };

    class medic_clothing
    {
        displayName = "EMS Uniforms";
        faction = "medic";
        licence = "";
        level = 0;
        markers[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
        radius = 30;
        class Gear
        {
            class U_C_Paramedic_01_F       { price = 25; };   // DLC: Laws of War
            class U_Rangemaster            { price = 25; };
            class V_Safety_yellow_F        { price = 100; };  // DLC: Laws of War
            class V_Safety_orange_F        { price = 100; };  // DLC: Laws of War
            class H_Cap_blk                { price = 25; };
            class H_Cap_marshal            { price = 25; level = 2; };
            class G_Aviator                { price = 50; };
            class B_AssaultPack_blk        { price = 500; };
            class B_Messenger_Gray_F       { price = 750; };  // DLC: Contact
            class B_FieldPack_blk          { price = 1000; level = 1; };
        };
    };

    class medic_vehicles
    {
        displayName = "EMS Garage";
        faction = "medic";
        licence = "";
        level = 0;
        markers[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
        radius = 30;
        class Vehicles
        {
            class C_Offroad_01_F           { price = 5000; };
            class C_SUV_01_F               { price = 10000; level = 1; };
            class C_Van_02_medevac_F       { price = 15000; level = 1; }; // DLC: Laws of War (ambulance)
            class O_Truck_03_medical_F     { price = 40000; level = 3; }; // Tempest medical
        };
        vehicleSpawns[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
    };

    class medic_air
    {
        displayName = "Air Ambulance";
        faction = "medic";
        licence = "mAir";
        level = 0;
        markers[] = { "spawn_medic_kavala", "spawn_medic_athira" };
        radius = 30;
        class Vehicles
        {
            class C_Heli_Light_01_civil_F  { price = 50000; };            // M-900
            class O_Heli_Light_02_unarmed_F { price = 150000; level = 3; }; // Orca
        };
        vehicleSpawns[] = { "spawn_medic_kavala", "spawn_medic_athira" };
    };
};
