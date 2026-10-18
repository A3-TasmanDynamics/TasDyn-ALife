// Shops: where they are, who can use them and what they sell. Prices and
// per-item rules live with the things themselves:
//   items[]    -> CfgMarket (+ CfgItems)    vehicles[] -> CfgVehicles
//   weapons[]  -> CfgWeapons                clothing[] -> CfgClothing
//
//   displayName   -- window title / action name
//   faction       -- "civ" / "cop" / "medic" (who can use it); also picks the
//                    price block (class civ / cop / medic) in the configs above
//   licence       -- needed to use the shop at all (CfgLicences key, "" = none)
//   level         -- minimum cop_level / medic_level to use it, 0 = any
//   markers[]     -- where it is (mission.sqm markers)
//   radius        -- metres from a marker
//   vehicleSpawns[] -- vehicle shops: where bought vehicles appear
//
// Shops sit on the town spawn markers for now; give each its own marker in
// Eden (and list it here) when the shops get placed properly. Only items[]
// can be bought so far -- vehicles, weapons and clothing are priced and
// ready for the next step (ALife_fnc_shopBuy / ALife_fnc_shop).

class CfgShops
{
    class general_store
    {
        displayName = "General Store";
        faction = "civ";
        licence = "";
        level = 0;
        markers[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
        radius = 30;
        items[] = {
            "water", "coffee", "apple", "tbacon",
            "redgull", "pickaxe", "fuelcan", "lockpick",
            "repairkit"
        };
        weapons[] = {
            "ItemMap", "ItemCompass", "ItemWatch", "ItemRadio",
            "ItemGPS", "Binocular", "FirstAidKit", "Chemlight_green",
            "NVGoggles"
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
        clothing[] = {
            "U_C_Poloshirt_blue", "U_C_Poloshirt_burgundy", "U_C_Poloshirt_redwhite", "U_C_Poloshirt_salmon",
            "U_C_Poloshirt_stripped", "U_C_Poloshirt_tricolour", "U_C_Commoner1_1", "U_C_WorkerCoveralls",
            "U_C_HunterBody_grn", "U_C_Journalist", "U_OrestesBody", "U_Rangemaster",
            "U_C_Scientist", "U_C_man_sport_1_F", "U_C_Man_casual_1_F", "U_C_Mechanic_01_F",
            "H_Cap_red", "H_Cap_blu", "H_Cap_grn", "H_Cap_blk",
            "H_Bandanna_gry", "H_Hat_brown", "H_Hat_tan", "H_StrawHat",
            "H_Booniehat_tan", "G_Shades_Black", "G_Shades_Blue", "G_Sport_Blackred",
            "G_Squares", "G_Aviator", "G_Spectacles", "B_AssaultPack_cbr",
            "B_AssaultPack_blk", "B_FieldPack_cbr", "B_Kitbag_cbr", "B_Carryall_cbr"
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
        weapons[] = {
            "hgun_P07_F", "hgun_Rook40_F", "hgun_ACPC2_F", "hgun_Pistol_heavy_02_F",
            "hgun_Pistol_heavy_01_F", "hgun_PDW2000_F", "16Rnd_9x21_Mag", "9Rnd_45ACP_Mag",
            "6Rnd_45ACP_Cylinder", "11Rnd_45ACP_Mag", "30Rnd_9x21_Mag", "optic_Yorris",
            "optic_MRD", "optic_ACO_grn_smg", "acc_flashlight_pistol"
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
        vehicleSpawns[] = { "spawn_civ_sofia" };
        items[] = {
            "water", "tbacon", "lockpick", "zipties",
            "repairkit", "fuelcan"
        };
        vehicles[] = {
            "B_Quadbike_01_F", "B_G_Offroad_01_F", "O_MRAP_02_F", "B_Heli_Light_01_stripped_F"
        };
        weapons[] = {
            "arifle_AKS_F", "arifle_TRG20_F", "arifle_TRG21_F", "arifle_Mk20_F",
            "arifle_Katiba_F", "arifle_AKM_F", "srifle_DMR_01_F", "srifle_EBR_F",
            "30Rnd_545x39_Mag_F", "30Rnd_556x45_Stanag", "30Rnd_65x39_caseless_green", "30Rnd_762x39_Mag_F",
            "10Rnd_762x54_Mag", "20Rnd_762x51_Mag", "optic_ACO_grn", "optic_Holosight",
            "optic_MRCO", "optic_Arco", "optic_Hamr", "acc_flashlight"
        };
        clothing[] = {
            "U_IG_Guerilla1_1", "U_IG_Guerilla2_1", "U_IG_Guerilla3_1", "U_I_G_resistanceLeader_F",
            "U_I_G_Story_Protagonist_F", "U_O_SpecopsUniform_ocamo", "V_BandollierB_cbr", "V_HarnessO_brn",
            "V_TacVest_khk", "V_PlateCarrierIA1_dgtl", "H_Shemag_olive", "H_ShemagOpen_khk",
            "H_Bandanna_khk", "H_Booniehat_khk", "G_Balaclava_blk", "G_Bandanna_blk",
            "B_FieldPack_oli", "B_Kitbag_cbr", "B_Carryall_oli"
        };
    };

    class civ_cars
    {
        displayName = "Car Dealership";
        faction = "civ";
        licence = "driver";
        level = 0;
        markers[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
        radius = 30;
        vehicleSpawns[] = { "spawn_civ_kavala", "spawn_civ_athira", "spawn_civ_pyrgos", "spawn_civ_sofia" };
        vehicles[] = {
            "C_Quadbike_01_F", "C_Hatchback_01_F", "C_Offroad_01_F", "C_Offroad_02_unarmed_F",
            "C_SUV_01_F", "C_Hatchback_01_sport_F"
        };
    };

    class civ_trucks
    {
        displayName = "Truck Dealership";
        faction = "civ";
        licence = "truck";
        level = 0;
        markers[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
        radius = 30;
        vehicleSpawns[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
        vehicles[] = {
            "C_Van_01_transport_F", "C_Van_02_transport_F", "C_Van_01_box_F", "C_Van_02_vehicle_F",
            "C_Truck_02_transport_F", "C_Truck_02_covered_F", "C_Van_01_fuel_F", "C_Truck_02_fuel_F",
            "O_Truck_03_transport_F", "O_Truck_03_covered_F", "B_Truck_01_box_F", "O_Truck_03_device_F"
        };
    };

    class civ_air
    {
        displayName = "Aircraft Dealership";
        faction = "civ";
        licence = "pilot";
        level = 0;
        markers[] = { "spawn_civ_athira", "spawn_civ_pyrgos" };
        radius = 30;
        vehicleSpawns[] = { "spawn_civ_athira", "spawn_civ_pyrgos" };
        vehicles[] = {
            "C_Plane_Civil_01_F", "C_Heli_Light_01_civil_F", "B_Heli_Light_01_F", "O_Heli_Light_02_unarmed_F",
            "I_Heli_Transport_02_F"
        };
    };

    class civ_boats
    {
        displayName = "Boat Shop";
        faction = "civ";
        licence = "boat";
        level = 0;
        markers[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
        radius = 30;
        vehicleSpawns[] = { "spawn_civ_kavala", "spawn_civ_pyrgos" };
        vehicles[] = {
            "C_Scooter_Transport_01_F", "C_Rubberboat", "C_Boat_Civil_01_F", "C_Boat_Transport_02_F",
            "B_SDV_01_F"
        };
    };

    class cop_market
    {
        displayName = "Police Supplies";
        faction = "cop";
        licence = "";
        level = 0;
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
        radius = 30;
        items[] = {
            "water", "coffee", "donut", "repairkit",
            "fuelcan", "spikestrip"
        };
        weapons[] = {
            "ItemMap", "ItemCompass", "ItemWatch", "ItemRadio",
            "ItemGPS", "Binocular", "Rangefinder", "FirstAidKit",
            "ToolKit", "NVGoggles"
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
        weapons[] = {
            "hgun_P07_snds_F", "hgun_P07_F", "SMG_05_F", "SMG_02_F",
            "arifle_MXC_F", "arifle_MX_F", "arifle_MXM_F", "16Rnd_9x21_Mag",
            "30Rnd_9x21_Mag_SMG_02", "30Rnd_65x39_caseless_mag", "muzzle_snds_L", "acc_flashlight",
            "acc_flashlight_pistol", "optic_Holosight_blk_F", "optic_MRCO", "optic_Arco",
            "optic_Hamr"
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
        clothing[] = {
            "U_Rangemaster", "U_B_GEN_Soldier_F", "U_B_GEN_Commander_F", "V_Rangemaster_belt",
            "V_TacVest_blk_POLICE", "V_TacVest_gen_F", "V_PlateCarrier1_blk", "H_Cap_police",
            "H_MilCap_gen_F", "H_Beret_gen_F", "G_Aviator", "G_Shades_Black",
            "B_AssaultPack_blk", "B_FieldPack_blk", "B_Carryall_blk"
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
        vehicleSpawns[] = { "spawn_cop_kavala", "spawn_cop_athira", "spawn_cop_pyrgos" };
        vehicles[] = {
            "C_Offroad_01_F", "B_GEN_Offroad_01_gen_F", "C_SUV_01_F", "C_Hatchback_01_sport_F",
            "B_MRAP_01_F", "I_MRAP_03_F"
        };
    };

    class cop_air
    {
        displayName = "Police Air Unit";
        faction = "cop";
        licence = "cAir";
        level = 2;
        markers[] = { "spawn_cop_kavala", "spawn_cop_athira" };
        radius = 30;
        vehicleSpawns[] = { "spawn_cop_kavala", "spawn_cop_athira" };
        vehicles[] = {
            "B_Heli_Light_01_F", "B_Heli_Transport_01_F"
        };
    };

    class cop_boats
    {
        displayName = "Coast Guard";
        faction = "cop";
        licence = "cg";
        level = 0;
        markers[] = { "spawn_cop_kavala", "spawn_cop_pyrgos" };
        radius = 30;
        vehicleSpawns[] = { "spawn_cop_kavala", "spawn_cop_pyrgos" };
        vehicles[] = {
            "B_Boat_Transport_01_F", "C_Boat_Civil_01_police_F", "B_SDV_01_F"
        };
    };

    class medic_supplies
    {
        displayName = "Medical Supplies";
        faction = "medic";
        licence = "";
        level = 0;
        markers[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
        radius = 30;
        items[] = {
            "medkit", "water", "coffee", "repairkit",
            "fuelcan"
        };
        weapons[] = {
            "ItemMap", "ItemCompass", "ItemWatch", "ItemRadio",
            "ItemGPS", "Binocular", "FirstAidKit", "Medikit",
            "ToolKit", "Chemlight_green", "NVGoggles"
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
        clothing[] = {
            "U_C_Paramedic_01_F", "U_Rangemaster", "V_Safety_yellow_F", "V_Safety_orange_F",
            "H_Cap_blk", "H_Cap_marshal", "G_Aviator", "B_AssaultPack_blk",
            "B_Messenger_Gray_F", "B_FieldPack_blk"
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
        vehicleSpawns[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
        vehicles[] = {
            "C_Offroad_01_F", "C_SUV_01_F", "C_Van_02_medevac_F", "O_Truck_03_medical_F"
        };
    };

    class medic_air
    {
        displayName = "Air Ambulance";
        faction = "medic";
        licence = "mAir";
        level = 0;
        markers[] = { "spawn_medic_kavala", "spawn_medic_athira" };
        radius = 30;
        vehicleSpawns[] = { "spawn_medic_kavala", "spawn_medic_athira" };
        vehicles[] = {
            "C_Heli_Light_01_civil_F", "O_Heli_Light_02_unarmed_F"
        };
    };
};
