// ALife gameplay settings. Read with
//   getNumber (missionConfigFile >> "CfgALife" >> "<setting>")
// Shops, prices, vehicles, clothing and spawn points have their own Cfg*
// files; this is everything else.

class CfgALife
{
    // ---- Keys (DIK codes) -------------------------------------------------
    phoneKey = 21;                      // Y: phone
    lockKey = 22;                       // U: lock / unlock your own vehicle
    interactKeys[] = { 219, 220 };      // left / right Windows: look at a bag, vehicle or ATM to open it;
                                        // at nothing for your own inventory (to drop things)

    // ---- Death and revive -------------------------------------------------
    respawnTime = 120;                  // seconds on the death screen before Respawn unlocks (or wait for a medic)
    reviveTime = 8;                     // seconds a medic holds Revive (uses one medkit)
    reviveHealth = 0.5;                 // health (0-1) a revived player gets up with, where they fell, with their gear

    // ---- Police slots -----------------------------------------------------
    // mission.sqm police_1 .. police_10: the first copPublicSlots are open to
    // anyone; the rest need players.cop_level >= copWhitelistLevel.
    copPublicSlots = 4;
    copWhitelistLevel = 1;

    // ---- Phone ------------------------------------------------------------
    giveDistance = 5;                   // metres: give items / money to players this close
    messageCooldown = 3;                // seconds between messages
    messageMaxLength = 300;             // characters per message

    // ---- Food, water and the HUD (ALife_fnc_survival) ---------------------
    // Both run 0-100 and drop over time; eating / drinking (CfgItems `food` /
    // `water`) brings them back. At 0 you lose health until you eat or
    // drink. Saved with your gear; a new life starts full.
    class Survival
    {
        foodPerMinute = 1.5;            // ~65 minutes from full to empty
        waterPerMinute = 2;             // ~50 minutes
        emptyDamagePerMinute = 0.08;    // health lost per minute for each one at 0
        warnAt = 20;                    // a notice when one drops to this
        hud = 1;                        // 1: health / food / water bars, bottom right
    };

    // ---- Inventory and dropped items ---------------------------------------
    carryWeight = 24;                   // how much a player can carry (item `weight`s)
    bagClass = "B_AssaultPack_khk";     // what dropped items look like (placeholder)
    cashClass = "Land_Money_F";         // dropping only cash leaves a money stack instead
    cashIcon = "ui\items\cash_ca.paa";  // cash in inventory lists
    bagLifetime = 900;                  // seconds before a dropped bag disappears

    // Vehicle storage for vehicles without a `trunk` in CfgVehicles (e.g.
    // ones not sold in a shop), by type.
    class Trunks
    {
        car = 50;
        truck = 150;
        air = 100;
        ship = 80;
        other = 30;
    };

    // ---- ATMs -------------------------------------------------------------
    // Every ATM on the map works, recognised by model or class, so nothing
    // needs placing. Windows key while looking at one.
    atmModels[] = { "atm_01_f.p3d", "atm_02_f.p3d" }; // structures_f_epc: Civ\Accessories\Atm_0x_F.p3d
    atmClasses[] = { "Land_Atm_01_F", "Land_Atm_02_F" };
    atmRange = 3;                       // metres from the ATM

    // ---- Map / GPS markers for your own people (ALife_fnc_groupMarkers) ---
    class GroupMarkers
    {
        interval = 0.5;                 // seconds between updates
        needGps = 1;                    // 1: you and they both need a GPS; 0: always shown
        spawnGps = 1;                   // 1: every spawn / respawn gets a GPS
        type = "mil_triangle";          // marker icon (CfgMarkers), pointing the way they face
        size = 0.6;
        // Per faction: on (0 = no markers), colour (CfgMarkerColors), and
        // the factions they see ("cop", "medic"). Add "medic" to cop's
        // sees[] for police to see EMS too.
        class cop   { on = 1; colour = "ColorBLUFOR"; sees[] = { "cop" }; };
        class medic { on = 1; colour = "ColorRed";    sees[] = { "medic" }; };
        // Civilians see members of their own gang (website gangs).
        class gang  { on = 1; colour = "ColorOrange"; };
        // You, for everyone (with a GPS when needGps = 1).
        class self  { on = 1; colour = "ColorYellow"; type = "mil_arrow2"; size = 0.8; text = "You"; };
    };
};
