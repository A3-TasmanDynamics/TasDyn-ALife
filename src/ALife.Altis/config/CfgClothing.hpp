// Clothing sold in shops: uniforms, vests, headgear, glasses and
// backpacks, one class per Arma classname, with what it costs and who may
// buy it, per faction.
//
//   slot   -- "uniform", "vest", "headgear", "goggles" or "backpack"
//   class civ / cop / medic -- that faction can buy it:
//     price, licence, level -- as in CfgVehicles
//
// Which shop sells what is in CfgShops (clothing[]).

class CfgClothing
{
    class U_IG_Guerilla1_1
    {
        slot = "uniform";
        class civ   { price = 5000; };
    };
    class U_IG_Guerilla2_1
    {
        slot = "uniform";
        class civ   { price = 5500; };
    };
    class U_IG_Guerilla3_1
    {
        slot = "uniform";
        class civ   { price = 5500; };
    };
    class U_I_G_resistanceLeader_F
    {
        slot = "uniform";
        class civ   { price = 11500; };
    };
    class U_I_G_Story_Protagonist_F
    {
        slot = "uniform";
        class civ   { price = 7500; };
    };
    class U_O_SpecopsUniform_ocamo
    {
        slot = "uniform";
        class civ   { price = 17500; };
    };
    class V_BandollierB_cbr
    {
        slot = "vest";
        class civ   { price = 4500; };
    };
    class V_HarnessO_brn
    {
        slot = "vest";
        class civ   { price = 7500; };
    };
    class V_TacVest_khk
    {
        slot = "vest";
        class civ   { price = 12500; };
    };
    class V_PlateCarrierIA1_dgtl
    {
        slot = "vest";
        class civ   { price = 25000; };
    };
    class H_Shemag_olive
    {
        slot = "headgear";
        class civ   { price = 850; };
    };
    class H_ShemagOpen_khk
    {
        slot = "headgear";
        class civ   { price = 800; };
    };
    class H_Bandanna_khk
    {
        slot = "headgear";
        class civ   { price = 500; };
    };
    class H_Booniehat_khk
    {
        slot = "headgear";
        class civ   { price = 650; };
    };
    class G_Balaclava_blk
    {
        slot = "goggles";
        class civ   { price = 2000; };
    };
    class G_Bandanna_blk
    {
        slot = "goggles";
        class civ   { price = 1500; };
    };
    class B_FieldPack_oli
    {
        slot = "backpack";
        class civ   { price = 3000; };
    };
    class B_Kitbag_cbr
    {
        slot = "backpack";
        class civ   { price = 4500; };
    };
    class B_Carryall_oli
    {
        slot = "backpack";
        class civ   { price = 6500; };
    };
    class U_C_Poloshirt_blue
    {
        slot = "uniform";
        class civ   { price = 250; };
    };
    class U_C_Poloshirt_burgundy
    {
        slot = "uniform";
        class civ   { price = 275; };
    };
    class U_C_Poloshirt_redwhite
    {
        slot = "uniform";
        class civ   { price = 150; };
    };
    class U_C_Poloshirt_salmon
    {
        slot = "uniform";
        class civ   { price = 175; };
    };
    class U_C_Poloshirt_stripped
    {
        slot = "uniform";
        class civ   { price = 125; };
    };
    class U_C_Poloshirt_tricolour
    {
        slot = "uniform";
        class civ   { price = 350; };
    };
    class U_C_Commoner1_1
    {
        slot = "uniform";
        class civ   { price = 200; };
    };
    class U_C_WorkerCoveralls
    {
        slot = "uniform";
        class civ   { price = 450; };
    };
    class U_C_HunterBody_grn
    {
        slot = "uniform";
        class civ   { price = 1500; };
    };
    class U_C_Journalist
    {
        slot = "uniform";
        class civ   { price = 1200; };
    };
    class U_OrestesBody
    {
        slot = "uniform";
        class civ   { price = 1100; };
    };
    class U_Rangemaster
    {
        slot = "uniform";
        class civ   { price = 1250; };
        class cop   { price = 25; };
        class medic { price = 25; };
    };
    class U_C_Scientist
    {
        slot = "uniform";
        class civ   { price = 900; };
    };
    class U_C_man_sport_1_F  // DLC: Apex
    {
        slot = "uniform";
        class civ   { price = 500; };
    };
    class U_C_Man_casual_1_F  // DLC: Apex
    {
        slot = "uniform";
        class civ   { price = 600; };
    };
    class U_C_Mechanic_01_F  // DLC: Laws of War
    {
        slot = "uniform";
        class civ   { price = 700; };
    };
    class H_Cap_red
    {
        slot = "headgear";
        class civ   { price = 100; };
    };
    class H_Cap_blu
    {
        slot = "headgear";
        class civ   { price = 100; };
    };
    class H_Cap_grn
    {
        slot = "headgear";
        class civ   { price = 100; };
    };
    class H_Cap_blk
    {
        slot = "headgear";
        class civ   { price = 100; };
        class medic { price = 25; };
    };
    class H_Bandanna_gry
    {
        slot = "headgear";
        class civ   { price = 150; };
    };
    class H_Hat_brown
    {
        slot = "headgear";
        class civ   { price = 250; };
    };
    class H_Hat_tan
    {
        slot = "headgear";
        class civ   { price = 250; };
    };
    class H_StrawHat
    {
        slot = "headgear";
        class civ   { price = 225; };
    };
    class H_Booniehat_tan  // Glasses
    {
        slot = "headgear";
        class civ   { price = 300; };
    };
    class G_Shades_Black
    {
        slot = "goggles";
        class civ   { price = 50; };
        class cop   { price = 50; };
    };
    class G_Shades_Blue
    {
        slot = "goggles";
        class civ   { price = 50; };
    };
    class G_Sport_Blackred
    {
        slot = "goggles";
        class civ   { price = 75; };
    };
    class G_Squares
    {
        slot = "goggles";
        class civ   { price = 50; };
    };
    class G_Aviator
    {
        slot = "goggles";
        class civ   { price = 100; };
        class cop   { price = 50; };
        class medic { price = 50; };
    };
    class G_Spectacles  // Backpacks
    {
        slot = "goggles";
        class civ   { price = 50; };
    };
    class B_AssaultPack_cbr
    {
        slot = "backpack";
        class civ   { price = 2500; };
    };
    class B_AssaultPack_blk
    {
        slot = "backpack";
        class civ   { price = 2500; };
        class cop   { price = 500; };
        class medic { price = 500; };
    };
    class B_FieldPack_cbr
    {
        slot = "backpack";
        class civ   { price = 3000; };
    };
    class B_Carryall_cbr
    {
        slot = "backpack";
        class civ   { price = 6500; };
    };
    class U_B_GEN_Soldier_F  // DLC: Apex (Gendarmerie)
    {
        slot = "uniform";
        class cop   { price = 150; level = 1; };
    };
    class U_B_GEN_Commander_F  // DLC: Apex
    {
        slot = "uniform";
        class cop   { price = 250; level = 3; };
    };
    class V_Rangemaster_belt
    {
        slot = "vest";
        class cop   { price = 800; };
    };
    class V_TacVest_blk_POLICE
    {
        slot = "vest";
        class cop   { price = 1500; level = 1; };
    };
    class V_TacVest_gen_F  // DLC: Apex
    {
        slot = "vest";
        class cop   { price = 2000; level = 2; };
    };
    class V_PlateCarrier1_blk
    {
        slot = "vest";
        class cop   { price = 3500; level = 4; };
    };
    class H_Cap_police
    {
        slot = "headgear";
        class cop   { price = 25; };
    };
    class H_MilCap_gen_F  // DLC: Apex
    {
        slot = "headgear";
        class cop   { price = 50; level = 1; };
    };
    class H_Beret_gen_F  // DLC: Apex
    {
        slot = "headgear";
        class cop   { price = 75; level = 3; };
    };
    class B_FieldPack_blk
    {
        slot = "backpack";
        class cop   { price = 1000; level = 1; };
        class medic { price = 1000; level = 1; };
    };
    class B_Carryall_blk
    {
        slot = "backpack";
        class cop   { price = 1500; level = 3; };
    };
    class U_C_Paramedic_01_F
    {
        slot = "uniform";
        class medic { price = 25; };
    };
    class V_Safety_yellow_F  // DLC: Laws of War
    {
        slot = "vest";
        class medic { price = 100; };
    };
    class V_Safety_orange_F  // DLC: Laws of War
    {
        slot = "vest";
        class medic { price = 100; };
    };
    class H_Cap_marshal
    {
        slot = "headgear";
        class medic { price = 25; level = 2; };
    };
    class B_Messenger_Gray_F  // DLC: Contact
    {
        slot = "backpack";
        class medic { price = 750; };
    };
};
