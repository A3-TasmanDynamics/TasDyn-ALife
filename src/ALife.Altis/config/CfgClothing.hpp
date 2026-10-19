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
            {"U_C_Poloshirt_blue"     , "", 250, "", 0},  // Commoner Clothes (Blue)
            {"U_C_Poloshirt_burgundy" , "", 275, "", 0},  // Commoner Clothes (Burgundy)
            {"U_C_Poloshirt_redwhite" , "", 150, "", 0},  // Commoner Clothes (Red-White)
            {"U_C_Poloshirt_salmon"   , "", 175, "", 0},  // Commoner Clothes (Salmon)
            {"U_C_Poloshirt_stripped" , "", 125, "", 0},  // Commoner Clothes (Striped)
            {"U_C_Poloshirt_tricolour", "", 350, "", 0},  // Commoner Clothes (Tricolor)
            {"U_C_Commoner1_1"        , "", 200, "", 0},  // Commoner Clothes 2
            {"U_C_WorkerCoveralls"    , "", 450, "", 0},  // Worker Coveralls
            {"U_C_HunterBody_grn"     , "", 1500, "", 0},  // Hunting Clothes
            {"U_C_Journalist"         , "", 1200, "", 0},  // Journalist Clothes
            {"U_OrestesBody"          , "", 1100, "", 0},  // Jacket and Shorts
            {"U_Rangemaster"          , "", 1250, "", 0},  // Rangemaster Suit
            {"U_C_Scientist"          , "", 900, "", 0},  // Scientist Clothes
            {"U_C_man_sport_1_F"      , "", 500, "", 0},  // Sport Clothes (Beach) -- DLC: Apex
            {"U_C_Man_casual_1_F"     , "", 600, "", 0},  // Casual Clothes (Navy) -- DLC: Apex
            {"U_C_Mechanic_01_F"      , "", 700, "", 0}  // Mechanic Clothes -- DLC: Laws of War
        };
        headgear[] = {
            {"H_Cap_red"      , "", 100, "", 0},  // Cap (Red)
            {"H_Cap_blu"      , "", 100, "", 0},  // Cap (Blue)
            {"H_Cap_grn"      , "", 100, "", 0},  // Cap (Green)
            {"H_Cap_blk"      , "", 100, "", 0},  // Cap (Black)
            {"H_Bandanna_gry" , "", 150, "", 0},  // Bandana (Black)
            {"H_Hat_brown"    , "", 250, "", 0},  // Hat (Brown)
            {"H_Hat_tan"      , "", 250, "", 0},  // Hat (Tan)
            {"H_StrawHat"     , "", 225, "", 0},  // Straw Hat
            {"H_Booniehat_tan", "", 300, "", 0}  // Boonie Hat (Sand)
        };
        goggles[] = {
            {"G_Shades_Black"  , "", 50, "", 0},  // Shades (Black)
            {"G_Shades_Blue"   , "", 50, "", 0},  // Shades (Blue)
            {"G_Sport_Blackred", "", 75, "", 0},  // Sport Shades (Vulcan)
            {"G_Squares"       , "", 50, "", 0},  // Square Spectacles
            {"G_Aviator"       , "", 100, "", 0},  // Aviator Glasses
            {"G_Spectacles"    , "", 50, "", 0}  // Spectacle Glasses
        };
        backpacks[] = {
            {"B_AssaultPack_cbr", "", 2500, "", 0},  // Assault Pack (Coyote)
            {"B_AssaultPack_blk", "", 2500, "", 0},  // Assault Pack (Black)
            {"B_FieldPack_cbr"  , "", 3000, "", 0},  // Field Pack (Coyote)
            {"B_Kitbag_cbr"     , "", 4500, "", 0},  // Kitbag (Coyote)
            {"B_Carryall_cbr"   , "", 6500, "", 0}  // Carryall Backpack (Coyote)
        };
    };

    class rebel
    {
        name = "Rebel Clothing";
        side = "civ";
        licence = "rebel";
        level = 0;
        uniforms[] = {
            {"U_IG_Guerilla1_1"         , "", 5000, "", 0},  // Guerilla Garment
            {"U_IG_Guerilla2_1"         , "", 5500, "", 0},  // Guerilla Outfit (Plain, Dark)
            {"U_IG_Guerilla3_1"         , "", 5500, "", 0},  // Guerilla Smocks
            {"U_I_G_resistanceLeader_F" , "", 11500, "", 0},  // Combat Fatigues (Stavrou)
            {"U_I_G_Story_Protagonist_F", "", 7500, "", 0},  // Worn Combat Fatigues (Kerry)
            {"U_O_SpecopsUniform_ocamo" , "", 17500, "", 0}  // Recon Fatigues (Hex)
        };
        headgear[] = {
            {"H_Shemag_olive"  , "", 850, "", 0},  // Shemag (Olive)
            {"H_ShemagOpen_khk", "", 800, "", 0},  // Shemag (White)
            {"H_Bandanna_khk"  , "", 500, "", 0},  // Bandana (Khaki)
            {"H_Booniehat_khk" , "", 650, "", 0}  // Boonie Hat (Khaki)
        };
        goggles[] = {
            {"G_Balaclava_blk", "", 2000, "", 0},  // Balaclava (Black)
            {"G_Bandanna_blk" , "", 1500, "", 0}  // Bandana (Black)
        };
        vests[] = {
            {"V_BandollierB_cbr"     , "", 4500, "", 0},  // Slash Bandolier (Coyote)
            {"V_HarnessO_brn"        , "", 7500, "", 0},  // LBV Harness
            {"V_TacVest_khk"         , "", 12500, "", 0},  // Tactical Vest (Khaki)
            {"V_PlateCarrierIA1_dgtl", "", 25000, "", 0}  // GA Carrier Lite (Digital)
        };
        backpacks[] = {
            {"B_FieldPack_oli", "", 3000, "", 0},  // Field Pack (Olive)
            {"B_Kitbag_cbr"   , "", 4500, "", 0},  // Kitbag (Coyote)
            {"B_Carryall_oli" , "", 6500, "", 0}  // Carryall Backpack (Olive)
        };
    };

    class cop
    {
        name = "Police Uniforms";
        side = "cop";
        licence = "";
        level = 0;
        uniforms[] = {
            {"U_Rangemaster"      , "", 25, "", 0},  // Rangemaster Suit
            {"U_B_GEN_Soldier_F"  , "", 150, "", 1},  // Gendarmerie Uniform -- DLC: Apex (Gendarmerie)
            {"U_B_GEN_Commander_F", "", 250, "", 3}  // Gendarmerie Commander Uniform -- DLC: Apex
        };
        headgear[] = {
            {"H_Cap_police"  , "", 25, "", 0},  // Cap (Police)
            {"H_MilCap_gen_F", "", 50, "", 1},  // Military Cap (Gendarmerie) -- DLC: Apex
            {"H_Beret_gen_F" , "", 75, "", 3}  // Beret (Gendarmerie) -- DLC: Apex
        };
        goggles[] = {
            {"G_Aviator"     , "", 50, "", 0},  // Aviator Glasses
            {"G_Shades_Black", "", 50, "", 0}  // Shades (Black)
        };
        vests[] = {
            {"V_Rangemaster_belt"  , "", 800, "", 0},  // Rangemaster Belt
            {"V_TacVest_blk_POLICE", "", 1500, "", 1},  // Tactical Vest (Police)
            {"V_TacVest_gen_F"     , "", 2000, "", 2},  // Gendarmerie Vest -- DLC: Apex
            {"V_PlateCarrier1_blk" , "", 3500, "", 4}  // Carrier Lite (Black)
        };
        backpacks[] = {
            {"B_AssaultPack_blk", "", 500, "", 0},  // Assault Pack (Black)
            {"B_FieldPack_blk"  , "", 1000, "", 1},  // Field Pack (Black)
            {"B_Carryall_blk"   , "", 1500, "", 3}  // Carryall Backpack (Black)
        };
    };

    class medic
    {
        name = "EMS Uniforms";
        side = "medic";
        licence = "";
        level = 0;
        uniforms[] = {
            {"U_C_Paramedic_01_F", "", 25, "", 0},  // Paramedic Outfit -- DLC: Laws of War
            {"U_Rangemaster"     , "", 25, "", 0}  // Rangemaster Suit
        };
        headgear[] = {
            {"H_Cap_blk"    , "", 25, "", 0},  // Cap (Black)
            {"H_Cap_marshal", "", 25, "", 2}  // Marshal Cap
        };
        goggles[] = {
            {"G_Aviator", "", 50, "", 0}  // Aviator Glasses
        };
        vests[] = {
            {"V_Safety_yellow_F", "", 100, "", 0},  // Safety Vest (Yellow) -- DLC: Laws of War
            {"V_Safety_orange_F", "", 100, "", 0}  // Safety Vest (Orange) -- DLC: Laws of War
        };
        backpacks[] = {
            {"B_AssaultPack_blk" , "", 500, "", 0},  // Assault Pack (Black)
            {"B_Messenger_Gray_F", "", 750, "", 0},  // Messenger Bag (Gray) -- DLC: Contact
            {"B_FieldPack_blk"   , "", 1000, "", 1}  // Field Pack (Black)
        };
    };
};

// Uniform textures by group, applied while the uniform is worn (its main
// texture, hidden selection 0; ALife_fnc_applySkins). Placeholder cop and
// medic textures are recoloured vanilla (tools/textures/make_placeholders.py):
// drop the real PAAs in at the same paths under textures\uniforms\.
//
//   cop / medic: rows {uniform classname, texture}
//   gang:        rows {gang tag ("" = any gang), uniform classname, texture},
//                for civilians in a gang (the website's gangs); a row naming
//                the player's own tag wins over a "" row

class UniformTextures
{
    class cop
    {
        textures[] = {
            {"U_Rangemaster", "textures\uniforms\cop_uniform.paa"}  // Rangemaster Suit
        };
    };
    class medic
    {
        textures[] = {
            {"U_Rangemaster", "textures\uniforms\medic_uniform.paa"}  // Rangemaster Suit
        };
    };
    class gang
    {
        textures[] = {
            // {"", "U_IG_Guerilla1_1", "textures\uniforms\gang_guerilla.paa"},   // every gang
            // {"TDA", "U_IG_Guerilla1_1", "textures\uniforms\gang_tda.paa"}      // one gang, by tag
        };
    };
};
