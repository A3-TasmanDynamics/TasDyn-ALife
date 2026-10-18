// Clothing shops, in Tonic's row format (Config_Clothing) with our licence
// / rank fields instead of condition strings. Each shop:
//
//   name     -- shown in the shop window
//   side     -- "civ" / "cop" / "medic": who can use it
//   licence  -- needed to use the shop at all (CfgLicences key, "" = none)
//   level    -- minimum cop_level / medic_level to use it (0 = any)
//   uniforms[], headgear[], goggles[], vests[], backpacks[]
//
// Each row: {classname, "name" ("" = Arma's own), price,
//            "licence" ("" = none), level (0 = any)}
//
// Where each shop is: CfgShops.

class ClothingShops
{
    class civ
    {
        name = "Clothing Store";
        side = "civ";
        licence = "";
        level = 0;
        uniforms[] = {
            {"U_C_Poloshirt_blue"     , "", 250, "", 0},
            {"U_C_Poloshirt_burgundy" , "", 275, "", 0},
            {"U_C_Poloshirt_redwhite" , "", 150, "", 0},
            {"U_C_Poloshirt_salmon"   , "", 175, "", 0},
            {"U_C_Poloshirt_stripped" , "", 125, "", 0},
            {"U_C_Poloshirt_tricolour", "", 350, "", 0},
            {"U_C_Commoner1_1"        , "", 200, "", 0},
            {"U_C_WorkerCoveralls"    , "", 450, "", 0},
            {"U_C_HunterBody_grn"     , "", 1500, "", 0},
            {"U_C_Journalist"         , "", 1200, "", 0},
            {"U_OrestesBody"          , "", 1100, "", 0},
            {"U_Rangemaster"          , "", 1250, "", 0},
            {"U_C_Scientist"          , "", 900, "", 0},
            {"U_C_man_sport_1_F"      , "", 500, "", 0},  // DLC: Apex
            {"U_C_Man_casual_1_F"     , "", 600, "", 0},  // DLC: Apex
            {"U_C_Mechanic_01_F"      , "", 700, "", 0}   // DLC: Laws of War
        };
        headgear[] = {
            {"H_Cap_red"      , "", 100, "", 0},
            {"H_Cap_blu"      , "", 100, "", 0},
            {"H_Cap_grn"      , "", 100, "", 0},
            {"H_Cap_blk"      , "", 100, "", 0},
            {"H_Bandanna_gry" , "", 150, "", 0},
            {"H_Hat_brown"    , "", 250, "", 0},
            {"H_Hat_tan"      , "", 250, "", 0},
            {"H_StrawHat"     , "", 225, "", 0},
            {"H_Booniehat_tan", "", 300, "", 0}
        };
        goggles[] = {
            {"G_Shades_Black"  , "", 50, "", 0},
            {"G_Shades_Blue"   , "", 50, "", 0},
            {"G_Sport_Blackred", "", 75, "", 0},
            {"G_Squares"       , "", 50, "", 0},
            {"G_Aviator"       , "", 100, "", 0},
            {"G_Spectacles"    , "", 50, "", 0}
        };
        backpacks[] = {
            {"B_AssaultPack_cbr", "", 2500, "", 0},
            {"B_AssaultPack_blk", "", 2500, "", 0},
            {"B_FieldPack_cbr"  , "", 3000, "", 0},
            {"B_Kitbag_cbr"     , "", 4500, "", 0},
            {"B_Carryall_cbr"   , "", 6500, "", 0}
        };
    };

    class rebel
    {
        name = "Rebel Clothing";
        side = "civ";
        licence = "rebel";
        level = 0;
        uniforms[] = {
            {"U_IG_Guerilla1_1"         , "", 5000, "", 0},
            {"U_IG_Guerilla2_1"         , "", 5500, "", 0},
            {"U_IG_Guerilla3_1"         , "", 5500, "", 0},
            {"U_I_G_resistanceLeader_F" , "", 11500, "", 0},
            {"U_I_G_Story_Protagonist_F", "", 7500, "", 0},
            {"U_O_SpecopsUniform_ocamo" , "", 17500, "", 0}
        };
        headgear[] = {
            {"H_Shemag_olive"  , "", 850, "", 0},
            {"H_ShemagOpen_khk", "", 800, "", 0},
            {"H_Bandanna_khk"  , "", 500, "", 0},
            {"H_Booniehat_khk" , "", 650, "", 0}
        };
        goggles[] = {
            {"G_Balaclava_blk", "", 2000, "", 0},
            {"G_Bandanna_blk" , "", 1500, "", 0}
        };
        vests[] = {
            {"V_BandollierB_cbr"     , "", 4500, "", 0},
            {"V_HarnessO_brn"        , "", 7500, "", 0},
            {"V_TacVest_khk"         , "", 12500, "", 0},
            {"V_PlateCarrierIA1_dgtl", "", 25000, "", 0}
        };
        backpacks[] = {
            {"B_FieldPack_oli", "", 3000, "", 0},
            {"B_Kitbag_cbr"   , "", 4500, "", 0},
            {"B_Carryall_oli" , "", 6500, "", 0}
        };
    };

    class cop
    {
        name = "Police Uniforms";
        side = "cop";
        licence = "";
        level = 0;
        uniforms[] = {
            {"U_Rangemaster"      , "", 25, "", 0},
            {"U_B_GEN_Soldier_F"  , "", 150, "", 1},  // DLC: Apex (Gendarmerie)
            {"U_B_GEN_Commander_F", "", 250, "", 3}   // DLC: Apex
        };
        headgear[] = {
            {"H_Cap_police"  , "", 25, "", 0},
            {"H_MilCap_gen_F", "", 50, "", 1},  // DLC: Apex
            {"H_Beret_gen_F" , "", 75, "", 3}   // DLC: Apex
        };
        goggles[] = {
            {"G_Aviator"     , "", 50, "", 0},
            {"G_Shades_Black", "", 50, "", 0}
        };
        vests[] = {
            {"V_Rangemaster_belt"  , "", 800, "", 0},
            {"V_TacVest_blk_POLICE", "", 1500, "", 1},
            {"V_TacVest_gen_F"     , "", 2000, "", 2},  // DLC: Apex
            {"V_PlateCarrier1_blk" , "", 3500, "", 4}
        };
        backpacks[] = {
            {"B_AssaultPack_blk", "", 500, "", 0},
            {"B_FieldPack_blk"  , "", 1000, "", 1},
            {"B_Carryall_blk"   , "", 1500, "", 3}
        };
    };

    class medic
    {
        name = "EMS Uniforms";
        side = "medic";
        licence = "";
        level = 0;
        uniforms[] = {
            {"U_C_Paramedic_01_F", "", 25, "", 0},  // DLC: Laws of War
            {"U_Rangemaster"     , "", 25, "", 0}
        };
        headgear[] = {
            {"H_Cap_blk"    , "", 25, "", 0},
            {"H_Cap_marshal", "", 25, "", 2}
        };
        goggles[] = {
            {"G_Aviator", "", 50, "", 0}
        };
        vests[] = {
            {"V_Safety_yellow_F", "", 100, "", 0},  // DLC: Laws of War
            {"V_Safety_orange_F", "", 100, "", 0}   // DLC: Laws of War
        };
        backpacks[] = {
            {"B_AssaultPack_blk" , "", 500, "", 0},
            {"B_Messenger_Gray_F", "", 750, "", 0},  // DLC: Contact
            {"B_FieldPack_blk"   , "", 1000, "", 1}
        };
    };
};
