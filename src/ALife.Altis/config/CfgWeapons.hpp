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
            {"ItemMap"        , "", 50, 25, "", 0},  // Map
            {"ItemCompass"    , "", 50, 25, "", 0},  // Compass
            {"ItemWatch"      , "", 50, 25, "", 0},  // Watch
            {"ItemRadio"      , "", 100, 50, "", 0},  // Radio
            {"ItemGPS"        , "", 100, 50, "", 0},  // GPS
            {"Binocular"      , "", 150, 75, "", 0},  // Binoculars
            {"FirstAidKit"    , "", 150, 75, "", 0},  // First Aid Kit
            {"Chemlight_green", "", 30, 15, "", 0},  // Chemlight (Green)
            {"NVGoggles"      , "", 2000, 1000, "", 0}  // NV Goggles (Brown)
        };
    };

    class gun
    {
        name = "Gun Store";
        side = "civ";
        licence = "gun";
        level = 0;
        items[] = {
            {"hgun_P07_F"            , "", 1500, 750, "", 0},  // P07 9 mm
            {"hgun_Rook40_F"         , "", 2000, 1000, "", 0},  // Rook-40 9 mm
            {"hgun_ACPC2_F"          , "", 3500, 1750, "", 0},  // ACP-C2 .45 ACP
            {"hgun_Pistol_heavy_02_F", "", 4500, 2250, "", 0},  // Zubr .45 ACP -- Zubr
            {"hgun_Pistol_heavy_01_F", "", 6000, 3000, "", 0},  // 4-five .45 ACP -- 4-five
            {"hgun_PDW2000_F"        , "", 15000, 7500, "", 0}  // PDW2000 9 mm
        };
        mags[] = {
            {"16Rnd_9x21_Mag"     , "", 25, 12, "", 0},  // 9 mm 16Rnd Mag
            {"9Rnd_45ACP_Mag"     , "", 45, 22, "", 0},  // .45 ACP 9Rnd Mag
            {"6Rnd_45ACP_Cylinder", "", 50, 25, "", 0},  // .45 ACP 6Rnd Cylinder
            {"11Rnd_45ACP_Mag"    , "", 60, 30, "", 0},  // .45 ACP 11Rnd Mag
            {"30Rnd_9x21_Mag"     , "", 75, 37, "", 0}  // 9 mm 30Rnd Mag
        };
        accs[] = {
            {"optic_Yorris"         , "", 1000, 500, "", 0},  // Yorris J2
            {"optic_MRD"            , "", 1000, 500, "", 0},  // MRD
            {"optic_ACO_grn_smg"    , "", 1500, 750, "", 0},  // ACO SMG (Green)
            {"acc_flashlight_pistol", "", 500, 250, "", 0}  // Pistol Flashlight
        };
    };

    class rebel
    {
        name = "Rebel Armoury";
        side = "civ";
        licence = "rebel";
        level = 0;
        items[] = {
            {"arifle_AKS_F"   , "", 22000, 11000, "", 0},  // AKS-74U 5.45 mm -- DLC: Apex
            {"arifle_TRG20_F" , "", 25000, 12500, "", 0},  // TRG-20 5.56 mm
            {"arifle_TRG21_F" , "", 30000, 15000, "", 0},  // TRG-21 5.56 mm
            {"arifle_Mk20_F"  , "", 30000, 15000, "", 0},  // Mk20 5.56 mm (Camo)
            {"arifle_Katiba_F", "", 35000, 17500, "", 0},  // Katiba 6.5 mm
            {"arifle_AKM_F"   , "", 40000, 20000, "", 0},  // AKM 7.62 mm -- DLC: Apex
            {"srifle_DMR_01_F", "", 50000, 25000, "", 0},  // Rahim 7.62 mm -- Rahim
            {"srifle_EBR_F"   , "", 60000, 30000, "", 0}  // Mk18 ABR 7.62 mm -- Mk18 ABR
        };
        mags[] = {
            {"30Rnd_545x39_Mag_F"        , "", 150, 75, "", 0},  // 5.45 mm 30Rnd Reload Tracer (Yellow) Mag
            {"30Rnd_556x45_Stanag"       , "", 125, 62, "", 0},  // 5.56 mm 30rnd Reload Tracer (Yellow) Mag
            {"30Rnd_65x39_caseless_green", "", 275, 137, "", 0},  // 6.5 mm 30Rnd Caseless Mag
            {"30Rnd_762x39_Mag_F"        , "", 300, 150, "", 0},  // 7.62 mm 30Rnd AKM Reload Tracer (Yellow) Mag
            {"10Rnd_762x54_Mag"          , "", 500, 250, "", 0},  // 7.62 mm 10Rnd Mag
            {"20Rnd_762x51_Mag"          , "", 500, 250, "", 0}  // 7.62 mm 20Rnd Mag
        };
        accs[] = {
            {"optic_ACO_grn"  , "", 3500, 1750, "", 0},  // ACO (Green)
            {"optic_Holosight", "", 3600, 1800, "", 0},  // Mk17 Holosight
            {"optic_MRCO"     , "", 7500, 3750, "", 0},  // MRCO
            {"optic_Arco"     , "", 8000, 4000, "", 0},  // ARCO
            {"optic_Hamr"     , "", 8500, 4250, "", 0},  // RCO
            {"acc_flashlight" , "", 1000, 500, "", 0}  // Flashlight
        };
    };

    class cop_kit
    {
        name = "Police Supplies";
        side = "cop";
        licence = "";
        level = 0;
        items[] = {
            {"ItemMap"    , "", 50, 25, "", 0},  // Map
            {"ItemCompass", "", 50, 25, "", 0},  // Compass
            {"ItemWatch"  , "", 50, 25, "", 0},  // Watch
            {"ItemRadio"  , "", 100, 50, "", 0},  // Radio
            {"ItemGPS"    , "", 100, 50, "", 0},  // GPS
            {"Binocular"  , "", 150, 75, "", 0},  // Binoculars
            {"Rangefinder", "", 500, 250, "", 2},  // Rangefinder
            {"FirstAidKit", "", 150, 75, "", 0},  // First Aid Kit
            {"NVGoggles"  , "", 1200, 600, "", 1}  // NV Goggles (Brown)
        };
    };

    class cop
    {
        name = "Police Armoury";
        side = "cop";
        licence = "";
        level = 0;
        items[] = {
            {"hgun_P07_snds_F", "Taser", 2000, 1000, "", 0},  // P07 9 mm
            {"hgun_P07_F"     , "", 1500, 750, "", 0},  // P07 9 mm
            {"SMG_05_F"       , "", 12000, 6000, "", 1},  // Protector 9 mm -- DLC: Apex (Protector)
            {"SMG_02_F"       , "", 15000, 7500, "", 2},  // Sting 9 mm -- Sting
            {"arifle_MXC_F"   , "", 30000, 15000, "", 3},  // MXC 6.5 mm
            {"arifle_MX_F"    , "", 35000, 17500, "", 4},  // MX 6.5 mm
            {"arifle_MXM_F"   , "", 45000, 22500, "", 5}  // MXM 6.5 mm
        };
        mags[] = {
            {"16Rnd_9x21_Mag"          , "", 25, 12, "", 0},  // 9 mm 16Rnd Mag
            {"30Rnd_9x21_Mag_SMG_02"   , "", 75, 37, "", 1},  // 9 mm 30Rnd Mag
            {"30Rnd_65x39_caseless_mag", "", 130, 65, "", 3}  // 6.5 mm 30Rnd Sand Mag
        };
        accs[] = {
            {"muzzle_snds_L"        , "", 650, 325, "", 0},  // Sound Suppressor (9 mm)
            {"acc_flashlight"       , "", 750, 375, "", 1},  // Flashlight
            {"acc_flashlight_pistol", "", 500, 250, "", 0},  // Pistol Flashlight
            {"optic_Holosight_blk_F", "", 1200, 600, "", 1},  // Mk17 Holosight (Black)
            {"optic_MRCO"           , "", 2500, 1250, "", 3},  // MRCO
            {"optic_Arco"           , "", 2500, 1250, "", 3},  // ARCO
            {"optic_Hamr"           , "", 3500, 1750, "", 4}  // RCO
        };
    };

    class medic_kit
    {
        name = "EMS Equipment";
        side = "medic";
        licence = "";
        level = 0;
        items[] = {
            {"ItemMap"        , "", 50, 25, "", 0},  // Map
            {"ItemCompass"    , "", 50, 25, "", 0},  // Compass
            {"ItemWatch"      , "", 50, 25, "", 0},  // Watch
            {"ItemRadio"      , "", 100, 50, "", 0},  // Radio
            {"ItemGPS"        , "", 100, 50, "", 0},  // GPS
            {"Binocular"      , "", 150, 75, "", 0},  // Binoculars
            {"FirstAidKit"    , "", 50, 25, "", 0},  // First Aid Kit
            {"Chemlight_green", "", 30, 15, "", 0},  // Chemlight (Green)
            {"NVGoggles"      , "", 1200, 600, "", 1}  // NV Goggles (Brown)
        };
    };
};
