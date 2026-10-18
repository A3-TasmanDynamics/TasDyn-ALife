// Weapon shops, in Tonic's row format (Config_Weapons) with our licence /
// rank fields instead of condition strings. Each shop:
//
//   name     -- shown in the shop window
//   side     -- "civ" / "cop" / "medic": who can use it
//   licence  -- needed to use the shop at all (CfgLicences key, "" = none)
//   level    -- minimum cop_level / medic_level to use it (0 = any)
//   items[]  -- weapons and kit (map, GPS, binoculars, NVGs, first aid...)
//   mags[]   -- magazines
//   accs[]   -- attachments
//
// Each row: {classname, "name" ("" = Arma's own), buy price, sell price,
//            "licence" ("" = none), level (0 = any)}
//
// Where each shop is: CfgShops. Everything is vanilla Arma 3 or official
// DLC (marked "DLC: <name>"); players without a DLC can still buy its gear.

class WeaponShops
{
    class genstore
    {
        name = "General Store";
        side = "civ";
        licence = "";
        level = 0;
        items[] = {
            {"ItemMap"        , "", 50, 25, "", 0},
            {"ItemCompass"    , "", 50, 25, "", 0},
            {"ItemWatch"      , "", 50, 25, "", 0},
            {"ItemRadio"      , "", 100, 50, "", 0},
            {"ItemGPS"        , "", 100, 50, "", 0},
            {"Binocular"      , "", 150, 75, "", 0},
            {"FirstAidKit"    , "", 150, 75, "", 0},
            {"Chemlight_green", "", 30, 15, "", 0},
            {"NVGoggles"      , "", 2000, 1000, "", 0}
        };
    };

    class gun
    {
        name = "Gun Store";
        side = "civ";
        licence = "gun";
        level = 0;
        items[] = {
            {"hgun_P07_F"            , "", 1500, 750, "", 0},
            {"hgun_Rook40_F"         , "", 2000, 1000, "", 0},
            {"hgun_ACPC2_F"          , "", 3500, 1750, "", 0},
            {"hgun_Pistol_heavy_02_F", "", 4500, 2250, "", 0},  // Zubr
            {"hgun_Pistol_heavy_01_F", "", 6000, 3000, "", 0},  // 4-five
            {"hgun_PDW2000_F"        , "", 15000, 7500, "", 0}
        };
        mags[] = {
            {"16Rnd_9x21_Mag"     , "", 25, 12, "", 0},
            {"9Rnd_45ACP_Mag"     , "", 45, 22, "", 0},
            {"6Rnd_45ACP_Cylinder", "", 50, 25, "", 0},
            {"11Rnd_45ACP_Mag"    , "", 60, 30, "", 0},
            {"30Rnd_9x21_Mag"     , "", 75, 37, "", 0}
        };
        accs[] = {
            {"optic_Yorris"         , "", 1000, 500, "", 0},
            {"optic_MRD"            , "", 1000, 500, "", 0},
            {"optic_ACO_grn_smg"    , "", 1500, 750, "", 0},
            {"acc_flashlight_pistol", "", 500, 250, "", 0}
        };
    };

    class rebel
    {
        name = "Rebel Armoury";
        side = "civ";
        licence = "rebel";
        level = 0;
        items[] = {
            {"arifle_AKS_F"   , "", 22000, 11000, "", 0},  // DLC: Apex
            {"arifle_TRG20_F" , "", 25000, 12500, "", 0},
            {"arifle_TRG21_F" , "", 30000, 15000, "", 0},
            {"arifle_Mk20_F"  , "", 30000, 15000, "", 0},
            {"arifle_Katiba_F", "", 35000, 17500, "", 0},
            {"arifle_AKM_F"   , "", 40000, 20000, "", 0},  // DLC: Apex
            {"srifle_DMR_01_F", "", 50000, 25000, "", 0},  // Rahim
            {"srifle_EBR_F"   , "", 60000, 30000, "", 0}   // Mk18 ABR
        };
        mags[] = {
            {"30Rnd_545x39_Mag_F"        , "", 150, 75, "", 0},
            {"30Rnd_556x45_Stanag"       , "", 125, 62, "", 0},
            {"30Rnd_65x39_caseless_green", "", 275, 137, "", 0},
            {"30Rnd_762x39_Mag_F"        , "", 300, 150, "", 0},
            {"10Rnd_762x54_Mag"          , "", 500, 250, "", 0},
            {"20Rnd_762x51_Mag"          , "", 500, 250, "", 0}
        };
        accs[] = {
            {"optic_ACO_grn"  , "", 3500, 1750, "", 0},
            {"optic_Holosight", "", 3600, 1800, "", 0},
            {"optic_MRCO"     , "", 7500, 3750, "", 0},
            {"optic_Arco"     , "", 8000, 4000, "", 0},
            {"optic_Hamr"     , "", 8500, 4250, "", 0},
            {"acc_flashlight" , "", 1000, 500, "", 0}
        };
    };

    class cop_kit
    {
        name = "Police Supplies";
        side = "cop";
        licence = "";
        level = 0;
        items[] = {
            {"ItemMap"    , "", 50, 25, "", 0},
            {"ItemCompass", "", 50, 25, "", 0},
            {"ItemWatch"  , "", 50, 25, "", 0},
            {"ItemRadio"  , "", 100, 50, "", 0},
            {"ItemGPS"    , "", 100, 50, "", 0},
            {"Binocular"  , "", 150, 75, "", 0},
            {"Rangefinder", "", 500, 250, "", 2},
            {"FirstAidKit", "", 150, 75, "", 0},
            {"ToolKit"    , "", 250, 125, "", 0},
            {"NVGoggles"  , "", 1200, 600, "", 1}
        };
    };

    class cop
    {
        name = "Police Armoury";
        side = "cop";
        licence = "";
        level = 0;
        items[] = {
            {"hgun_P07_snds_F", "Taser", 2000, 1000, "", 0},
            {"hgun_P07_F"     , "", 1500, 750, "", 0},
            {"SMG_05_F"       , "", 12000, 6000, "", 1},  // DLC: Apex (Protector)
            {"SMG_02_F"       , "", 15000, 7500, "", 2},  // Sting
            {"arifle_MXC_F"   , "", 30000, 15000, "", 3},
            {"arifle_MX_F"    , "", 35000, 17500, "", 4},
            {"arifle_MXM_F"   , "", 45000, 22500, "", 5}
        };
        mags[] = {
            {"16Rnd_9x21_Mag"          , "", 25, 12, "", 0},
            {"30Rnd_9x21_Mag_SMG_02"   , "", 75, 37, "", 1},
            {"30Rnd_65x39_caseless_mag", "", 130, 65, "", 3}
        };
        accs[] = {
            {"muzzle_snds_L"        , "", 650, 325, "", 0},
            {"acc_flashlight"       , "", 750, 375, "", 1},
            {"acc_flashlight_pistol", "", 500, 250, "", 0},
            {"optic_Holosight_blk_F", "", 1200, 600, "", 1},
            {"optic_MRCO"           , "", 2500, 1250, "", 3},
            {"optic_Arco"           , "", 2500, 1250, "", 3},
            {"optic_Hamr"           , "", 3500, 1750, "", 4}
        };
    };

    class medic_kit
    {
        name = "EMS Equipment";
        side = "medic";
        licence = "";
        level = 0;
        items[] = {
            {"ItemMap"        , "", 50, 25, "", 0},
            {"ItemCompass"    , "", 50, 25, "", 0},
            {"ItemWatch"      , "", 50, 25, "", 0},
            {"ItemRadio"      , "", 100, 50, "", 0},
            {"ItemGPS"        , "", 100, 50, "", 0},
            {"Binocular"      , "", 150, 75, "", 0},
            {"FirstAidKit"    , "", 50, 25, "", 0},
            {"Medikit"        , "", 500, 250, "", 0},
            {"ToolKit"        , "", 250, 125, "", 0},
            {"Chemlight_green", "", 30, 15, "", 0},
            {"NVGoggles"      , "", 1200, 600, "", 1}
        };
    };
};
