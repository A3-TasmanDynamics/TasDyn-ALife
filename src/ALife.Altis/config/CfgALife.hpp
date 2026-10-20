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
    reviveTime = 8;                     // seconds a medic holds Revive (needs a Defibrillator, not used up)
    reviveHealth = 0.5;                 // health (0-1) a revived player gets up with, where they fell, with their gear

    // ---- Staff ------------------------------------------------------------
    // Server owners (Steam UIDs): every action in the phone's Admin app,
    // whatever their website rank -- including compensating themselves.
    // Everything they do is still written to the staff log (#staff-log).
    owners[] = { "76561198127262076" };   // Andrew Thompson

    // ---- Players (Windows key on a player, ALife_fnc_personMenu) ----------
    restrainKey = 19;                   // police: Left Shift + R (DIK 19) restrains / releases
    surrenderKey = 48;                  // Left Shift + B (DIK 48): hands on your head / down
    holsterKey = 35;                    // Left Shift + H (DIK 35): put your weapon away / draw it

    // Jumping: Left Shift + Space while running on foot (ALife_fnc_jump).
    class Jump
    {
        key = 57;               // Space (with Left Shift)
        up = 3.6;               // metres / second upwards
        forward = 0.6;          // metres / second extra forwards
        minSpeed = 6;           // km/h: you have to be moving (running)
        cooldown = 1.5;         // seconds between jumps
    };

    // ---- Dying (and not being revived) ------------------------------------
    // A new life starts with the spawn point's kit: the gear stays on the
    // body. These also go (your bank never does):
    deathLosesItems = 1;                // virtual items you were carrying (1 = lost)
    deathLosesCash = 1;                 // cash on hand (1 = lost)
    fines[] = { 500, 1000, 2500, 5000, 10000, 25000 }; // what police can fine (max 7)
    fineTimeout = 60;                   // seconds the fined player has to pay or refuse
    healTime = 6;                       // seconds a medic takes to heal a player (one medkit)

    // ---- Player tags (ALife_fnc_playerTags) -------------------------------
    // Names over other players you can see on foot; under the name, police
    // and EMS show their rank (the website's rank names) and civilians in a
    // gang their gang's name.
    class PlayerTags
    {
        on = 1;
        distance = 15;                  // metres
        max = 10;                       // tags at once (the nearest)
        copColour[] = { 0.48, 0.7, 1, 1 };     // the line under the name: police rank
        medicColour[] = { 0.45, 0.85, 0.6, 1 }; // EMS rank
        gangColour[] = { 0.98, 0.75, 0.14, 1 }; // gang name
    };

    // ---- Police placeables (CfgItems use = "place") ---------------------------
    class Placeables
    {
        max = 20;               // placed at once, per player
        lifetime = 3600;        // seconds before a placed object is cleared (the item is gone)
        pickupRange = 4;        // metres: Windows on it to pick it up or move it (police)
        spikeRange = 3.5;       // metres: vehicles this close to a spike strip lose their tyres
        // Choosing where it goes (Use, or Move on a placed one): it follows
        // in front of you until you put it down.
        placeRange = 6;         // metres: furthest from you it can go
        moveRange = 30;         // metres: how far you can carry a placed one while moving it
        placeKey = 57;          // Space: put it here
        cancelKey = 14;         // Backspace: cancel
        turnLeftKey = 16;       // Q: turn it 15 degrees
        turnRightKey = 18;      // E: turn it 15 degrees
    };

    // ---- Lights and sirens (police / EMS vehicles, ALife_fnc_emergency) ----
    // Any vehicle the faction's shops sell; the driver toggles them. Where
    // the lights sit: CfgVehicles >> <class> >> lights[], else the roof.
    class Emergency
    {
        lightsKey = 38;         // Left Shift + L (DIK 38): light bar on / off
        sirenKey = 33;          // Left Shift + F (DIK 33): siren on / off
        lightRange = 1200;      // metres: you see a vehicle's lights from this far
        sirenRange = 600;       // metres: you hear its siren from this far
        flash = 0.25;           // seconds each side of the light bar stays lit
        class cop
        {
            colours[] = { { 1, 0.05, 0.05 }, { 0.05, 0.15, 1 } };   // red / blue
            siren = "ALife_SirenPolice";                              // CfgSounds (description.ext)
            sirenLength = 2.17;                                       // seconds, one loop of it
        };
        class medic
        {
            colours[] = { { 1, 0.05, 0.05 }, { 1, 1, 1 } };          // red / white
            siren = "ALife_SirenEMS";
            sirenLength = 3.28;
        };
    };

    // ---- Lockers (police / EMS, ALife_fnc_locker) ---------------------------
    // Your own locker, the same at every locker of your faction: objects you
    // place in Eden with these names (Windows on one). Weapons and clothing
    // go in a crate (Arma's inventory; its size is how much fits), virtual
    // items in our inventory screen.
    class Lockers
    {
        cop[] = { "police_locker_1", "police_locker_2", "police_locker_3", "police_locker_4", "police_locker_5", "police_locker_6" };
        medic[] = { "medic_locker_1", "medic_locker_2", "medic_locker_3", "medic_locker_4", "medic_locker_5", "medic_locker_6" };
        range = 3;                      // metres from the locker object
        crate = "Box_NATO_Equip_F";     // what your gear sits in while it's open
        itemCapacity = 300;             // weight of virtual items it holds
    };

    // ---- Police tasers (ALife_fnc_taser) ----------------------------------
    // A hit from a taser weapon firing its taser cartridges, by a police
    // officer, does no damage: the target drops for `duration` seconds.
    // The same gun and magazine in anyone else's hands is a normal gun.
    class Taser
    {
        weapons[] = { "arifle_Mk20_plain_F", "hgun_P07_F", "hgun_P07_snds_F" };   // taser rifle, taser pistol (with / without the suppressor)
        magazines[] = { "30Rnd_556x45_Stanag_Tracer_Yellow", "16Rnd_9x21_yellow_Mag" }; // taser cartridges
        duration = 12;          // seconds on the ground
        range = 60;             // metres: further than this, it doesn't take
    };

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
        hud = 1;                        // 1: health / food / water HUD, bottom right
        hudStyle = "rings";           // default style: "minimal" (icons + numbers), "rings" or "off";
                                        // each player can switch it in the phone's Settings
    };

    // ---- Inventory and dropped items ---------------------------------------
    carryWeight = 24;                   // how much a player can carry (item `weight`s)
    bagClass = "B_AssaultPack_khk";     // what dropped items look like (placeholder)
    cashClass = "Land_Money_F";         // dropping only cash leaves a money stack instead
    cashIcon = "ui\items\cash_ca.paa";  // cash in inventory lists
    bagLifetime = 900;                  // seconds before a dropped bag disappears

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
        type = "mil_dot";               // marker icon (CfgMarkers)
        size = 0.6;
        // Per faction: on (0 = no markers), colour (CfgMarkerColors), and
        // the factions they see ("cop", "medic"). Add "medic" to cop's
        // sees[] for police to see EMS too.
        class cop   { on = 1; colour = "ColorBLUFOR"; sees[] = { "cop" }; };
        class medic { on = 1; colour = "ColorGUER";   sees[] = { "medic" }; }; // independent green
        // Civilians see members of their own gang (website gangs).
        class gang  { on = 1; colour = "ColorCIV"; };    // civilian purple
        // You, for everyone (with a GPS when needGps = 1). colour "" = your
        // faction's colour above (police blue, EMS green, civilian purple).
        class self  { on = 1; colour = ""; type = "mil_dot"; size = 0.8; text = "You"; };
    };
};
