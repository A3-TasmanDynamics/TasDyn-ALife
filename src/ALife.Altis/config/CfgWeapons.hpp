// Weapons, magazines, attachments and kit (map, GPS, binoculars, NVGs,
// first aid, toolkit) sold in shops: one class per Arma classname, with what
// it costs and who may buy it, per faction. Read with missionConfigFile >>
// "CfgWeapons" -- the mission's own list, not Arma's configFile.
//
//   type          -- "weapon", "magazine", "attachment" or "item" (shop tabs)
//   magazines[]   -- weapons: the magazines sold for it
//   attachments[] -- weapons: the attachments sold that fit it
//   class civ / cop / medic -- that faction can buy it:
//     price, licence, level -- as in CfgVehicles
//
// Which shop sells what is in CfgShops (weapons[]).

class CfgWeapons
{
    class hgun_P07_F
    {
        type = "weapon";
        magazines[] = { "16Rnd_9x21_Mag" };
        attachments[] = { "optic_Yorris", "optic_MRD", "acc_flashlight_pistol", "muzzle_snds_L" };
        class civ   { price = 1500; };
        class cop   { price = 1500; };
    };
    class hgun_Rook40_F
    {
        type = "weapon";
        magazines[] = { "16Rnd_9x21_Mag" };
        attachments[] = { "optic_Yorris", "optic_MRD", "acc_flashlight_pistol", "muzzle_snds_L" };
        class civ   { price = 2000; };
    };
    class hgun_ACPC2_F
    {
        type = "weapon";
        magazines[] = { "9Rnd_45ACP_Mag" };
        attachments[] = { "optic_Yorris", "optic_MRD", "acc_flashlight_pistol", "muzzle_snds_L" };
        class civ   { price = 3500; };
    };
    class hgun_Pistol_heavy_02_F  // Zubr
    {
        type = "weapon";
        magazines[] = { "6Rnd_45ACP_Cylinder" };
        attachments[] = { "optic_Yorris", "optic_MRD", "acc_flashlight_pistol", "muzzle_snds_L" };
        class civ   { price = 4500; };
    };
    class hgun_Pistol_heavy_01_F  // 4-five
    {
        type = "weapon";
        magazines[] = { "11Rnd_45ACP_Mag" };
        attachments[] = { "optic_Yorris", "optic_MRD", "acc_flashlight_pistol", "muzzle_snds_L" };
        class civ   { price = 6000; };
    };
    class hgun_PDW2000_F
    {
        type = "weapon";
        magazines[] = { "30Rnd_9x21_Mag" };
        attachments[] = { "optic_Yorris", "optic_MRD", "acc_flashlight_pistol", "muzzle_snds_L" };
        class civ   { price = 15000; };
    };
    class arifle_AKS_F
    {
        type = "weapon";
        magazines[] = { "30Rnd_545x39_Mag_F" };
        attachments[] = { "optic_ACO_grn", "optic_Holosight", "optic_Holosight_blk_F", "optic_MRCO", "optic_Arco", "optic_Hamr", "acc_flashlight" };
        class civ   { price = 22000; };
    };
    class arifle_TRG20_F
    {
        type = "weapon";
        magazines[] = { "30Rnd_556x45_Stanag" };
        attachments[] = { "optic_ACO_grn", "optic_Holosight", "optic_Holosight_blk_F", "optic_MRCO", "optic_Arco", "optic_Hamr", "acc_flashlight" };
        class civ   { price = 25000; };
    };
    class arifle_TRG21_F
    {
        type = "weapon";
        magazines[] = { "30Rnd_556x45_Stanag" };
        attachments[] = { "optic_ACO_grn", "optic_Holosight", "optic_Holosight_blk_F", "optic_MRCO", "optic_Arco", "optic_Hamr", "acc_flashlight" };
        class civ   { price = 30000; };
    };
    class arifle_Mk20_F
    {
        type = "weapon";
        magazines[] = { "30Rnd_556x45_Stanag" };
        attachments[] = { "optic_ACO_grn", "optic_Holosight", "optic_Holosight_blk_F", "optic_MRCO", "optic_Arco", "optic_Hamr", "acc_flashlight" };
        class civ   { price = 30000; };
    };
    class arifle_Katiba_F
    {
        type = "weapon";
        magazines[] = { "30Rnd_65x39_caseless_green" };
        attachments[] = { "optic_ACO_grn", "optic_Holosight", "optic_Holosight_blk_F", "optic_MRCO", "optic_Arco", "optic_Hamr", "acc_flashlight" };
        class civ   { price = 35000; };
    };
    class arifle_AKM_F  // DLC: Apex
    {
        type = "weapon";
        magazines[] = { "30Rnd_762x39_Mag_F" };
        attachments[] = { "optic_ACO_grn", "optic_Holosight", "optic_Holosight_blk_F", "optic_MRCO", "optic_Arco", "optic_Hamr", "acc_flashlight" };
        class civ   { price = 40000; };
    };
    class srifle_DMR_01_F  // Rahim
    {
        type = "weapon";
        magazines[] = { "10Rnd_762x54_Mag" };
        attachments[] = { "optic_ACO_grn", "optic_Holosight", "optic_Holosight_blk_F", "optic_MRCO", "optic_Arco", "optic_Hamr", "acc_flashlight" };
        class civ   { price = 50000; };
    };
    class srifle_EBR_F  // Mk18 ABR
    {
        type = "weapon";
        magazines[] = { "20Rnd_762x51_Mag" };
        attachments[] = { "optic_ACO_grn", "optic_Holosight", "optic_Holosight_blk_F", "optic_MRCO", "optic_Arco", "optic_Hamr", "acc_flashlight" };
        class civ   { price = 60000; };
    };
    class hgun_P07_snds_F
    {
        type = "weapon";
        magazines[] = { "16Rnd_9x21_Mag" };
        attachments[] = { "optic_Yorris", "optic_MRD", "acc_flashlight_pistol", "muzzle_snds_L" };
        class cop   { price = 2000; };
    };
    class SMG_05_F  // DLC: Apex (Protector)
    {
        type = "weapon";
        magazines[] = { "30Rnd_9x21_Mag_SMG_02" };
        attachments[] = { "optic_ACO_grn_smg", "optic_Holosight", "optic_Holosight_blk_F", "optic_ACO_grn", "acc_flashlight", "muzzle_snds_L" };
        class cop   { price = 12000; level = 1; };
    };
    class SMG_02_F  // Sting
    {
        type = "weapon";
        magazines[] = { "30Rnd_9x21_Mag_SMG_02" };
        attachments[] = { "optic_ACO_grn_smg", "optic_Holosight", "optic_Holosight_blk_F", "optic_ACO_grn", "acc_flashlight", "muzzle_snds_L" };
        class cop   { price = 15000; level = 2; };
    };
    class arifle_MXC_F
    {
        type = "weapon";
        magazines[] = { "30Rnd_65x39_caseless_mag" };
        attachments[] = { "optic_ACO_grn", "optic_Holosight", "optic_Holosight_blk_F", "optic_MRCO", "optic_Arco", "optic_Hamr", "acc_flashlight" };
        class cop   { price = 30000; level = 3; };
    };
    class arifle_MX_F
    {
        type = "weapon";
        magazines[] = { "30Rnd_65x39_caseless_mag" };
        attachments[] = { "optic_ACO_grn", "optic_Holosight", "optic_Holosight_blk_F", "optic_MRCO", "optic_Arco", "optic_Hamr", "acc_flashlight" };
        class cop   { price = 35000; level = 4; };
    };
    class arifle_MXM_F
    {
        type = "weapon";
        magazines[] = { "30Rnd_65x39_caseless_mag" };
        attachments[] = { "optic_ACO_grn", "optic_Holosight", "optic_Holosight_blk_F", "optic_MRCO", "optic_Arco", "optic_Hamr", "acc_flashlight" };
        class cop   { price = 45000; level = 5; };
    };
    class 16Rnd_9x21_Mag
    {
        type = "magazine";
        class civ   { price = 25; };
        class cop   { price = 25; };
    };
    class 9Rnd_45ACP_Mag
    {
        type = "magazine";
        class civ   { price = 45; };
    };
    class 6Rnd_45ACP_Cylinder
    {
        type = "magazine";
        class civ   { price = 50; };
    };
    class 11Rnd_45ACP_Mag
    {
        type = "magazine";
        class civ   { price = 60; };
    };
    class 30Rnd_9x21_Mag
    {
        type = "magazine";
        class civ   { price = 75; };
    };
    class 30Rnd_545x39_Mag_F
    {
        type = "magazine";
        class civ   { price = 150; };
    };
    class 30Rnd_556x45_Stanag
    {
        type = "magazine";
        class civ   { price = 125; };
    };
    class 30Rnd_65x39_caseless_green
    {
        type = "magazine";
        class civ   { price = 275; };
    };
    class 30Rnd_762x39_Mag_F
    {
        type = "magazine";
        class civ   { price = 300; };
    };
    class 10Rnd_762x54_Mag
    {
        type = "magazine";
        class civ   { price = 500; };
    };
    class 20Rnd_762x51_Mag
    {
        type = "magazine";
        class civ   { price = 500; };
    };
    class 30Rnd_9x21_Mag_SMG_02
    {
        type = "magazine";
        class cop   { price = 75; level = 1; };
    };
    class 30Rnd_65x39_caseless_mag
    {
        type = "magazine";
        class cop   { price = 130; level = 3; };
    };
    class optic_Yorris
    {
        type = "attachment";
        class civ   { price = 1000; };
    };
    class optic_MRD
    {
        type = "attachment";
        class civ   { price = 1000; };
    };
    class optic_ACO_grn_smg
    {
        type = "attachment";
        class civ   { price = 1500; };
    };
    class acc_flashlight_pistol
    {
        type = "attachment";
        class civ   { price = 500; };
        class cop   { price = 500; };
    };
    class optic_ACO_grn
    {
        type = "attachment";
        class civ   { price = 3500; };
    };
    class optic_Holosight
    {
        type = "attachment";
        class civ   { price = 3600; };
    };
    class optic_MRCO
    {
        type = "attachment";
        class civ   { price = 7500; };
        class cop   { price = 2500; level = 3; };
    };
    class optic_Arco
    {
        type = "attachment";
        class civ   { price = 8000; };
        class cop   { price = 2500; level = 3; };
    };
    class optic_Hamr
    {
        type = "attachment";
        class civ   { price = 8500; };
        class cop   { price = 3500; level = 4; };
    };
    class acc_flashlight  // Clothing
    {
        type = "attachment";
        class civ   { price = 1000; };
        class cop   { price = 750; level = 1; };
    };
    class muzzle_snds_L
    {
        type = "attachment";
        class cop   { price = 650; };
    };
    class optic_Holosight_blk_F
    {
        type = "attachment";
        class cop   { price = 1200; level = 1; };
    };
    class ItemMap
    {
        type = "item";
        class civ   { price = 50; };
        class cop   { price = 50; };
        class medic { price = 50; };
    };
    class ItemCompass
    {
        type = "item";
        class civ   { price = 50; };
        class cop   { price = 50; };
        class medic { price = 50; };
    };
    class ItemWatch
    {
        type = "item";
        class civ   { price = 50; };
        class cop   { price = 50; };
        class medic { price = 50; };
    };
    class ItemRadio
    {
        type = "item";
        class civ   { price = 100; };
        class cop   { price = 100; };
        class medic { price = 100; };
    };
    class ItemGPS
    {
        type = "item";
        class civ   { price = 100; };
        class cop   { price = 100; };
        class medic { price = 100; };
    };
    class Binocular
    {
        type = "item";
        class civ   { price = 150; };
        class cop   { price = 150; };
        class medic { price = 150; };
    };
    class Rangefinder
    {
        type = "item";
        class cop   { price = 500; level = 2; };
    };
    class FirstAidKit
    {
        type = "item";
        class civ   { price = 150; };
        class cop   { price = 150; };
        class medic { price = 50; };
    };
    class ToolKit
    {
        type = "item";
        class cop   { price = 250; };
        class medic { price = 250; };
    };
    class NVGoggles
    {
        type = "item";
        class civ   { price = 2000; };
        class cop   { price = 1200; level = 1; };
        class medic { price = 1200; level = 1; };
    };
    class Medikit
    {
        type = "item";
        class medic { price = 500; };
    };
    class Chemlight_green
    {
        type = "item";
        class civ   { price = 30; };
        class medic { price = 30; };
    };
};
