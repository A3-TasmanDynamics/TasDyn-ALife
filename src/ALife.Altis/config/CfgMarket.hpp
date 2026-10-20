// The market: virtual items. Tonic's Config_vItems split in two:
// VirtualShops (what each market sells) and CfgMarket (each item's BASE
// prices). What an item is -- name, icon, weight -- is in CfgItems.
//
// Prices move like a real market (ALife_fnc_market): every unit bought
// pushes an item's price up by `step`, every unit sold pushes it down, and
// between trades it drifts back toward base by `recovery` of the gap every
// `recoveryInterval` seconds (new stock arriving), within min..max. Buy and
// sell prices move together (the sell price keeps its ratio to the buy).
// Current prices are saved (market_prices) and survive restarts; these base
// prices come back only on a wipe or an economy reset.

class VirtualShops
{
    class resources
    {
        name = "Resource Trader";
        side = "civ";
        licence = "";
        level = 0;
        // Buys and sells the same resources (the Windows menu asks which).
        items[] = { "oil_raw", "oil_refined", "copper_ore", "copper_bar", "iron_ore", "iron_bar", "salt_raw", "salt",
                    "sand", "glass", "diamond_raw", "diamond", "rock", "cement" };
        buys[] = { "oil_raw", "oil_refined", "copper_ore", "copper_bar", "iron_ore", "iron_bar", "salt_raw", "salt",
                   "sand", "glass", "diamond_raw", "diamond", "rock", "cement" };
    };

    //   name     -- shown in the shop window
    //   side     -- "civ" / "cop" / "medic": who can use it
    //   licence  -- needed to use the shop at all (CfgLicences key, "" = none)
    //   level    -- minimum cop_level / medic_level to use it (0 = any)
    //   items[]  -- CfgMarket / CfgItems classes it sells to players
    //   buys[]   -- classes it buys from players, at the market's sell price
    //               (a shop with buys[] and no items[] is a trader; with
    //               both, the Windows menu offers Buy and Sell)

    class general
    {
        name = "Market";
        side = "civ";
        licence = "";
        level = 0;
        items[] = { "water", "coffee", "apple", "tbacon", "redgull", "firstaid", "pickaxe", "fuelcan", "lockpick", "toolkit" };
    };

    class rebel
    {
        name = "Rebel Market";
        side = "civ";
        licence = "rebel";
        level = 0;
        items[] = { "water", "tbacon", "firstaid", "lockpick", "zipties", "toolkit", "fuelcan" };
    };

    class cop
    {
        name = "Police Market";
        side = "cop";
        licence = "";
        level = 0;
        items[] = { "water", "coffee", "donut", "firstaid", "toolkit", "fuelcan", "cone", "barrier", "roadlight", "arrowsign", "spikestrip" };
    };

    class medic
    {
        name = "Medical Supplies";
        side = "medic";
        licence = "";
        level = 0;
        items[] = { "defib", "medkit", "firstaid", "water", "coffee", "toolkit", "fuelcan" };
    };
};

class CfgMarket
{
    step = 0.01;              // +1% per unit bought, -1% per unit sold
    recovery = 0.05;          // 5% of the gap back toward base...
    recoveryInterval = 300;   // ...every 5 minutes

    // buyPrice / sellPrice -- base prices, dollars
    // min / max            -- the buy price never leaves this range
    // licence / level      -- needed to buy it (optional)
    class water      { buyPrice = 10; sellPrice = 5; min = 5; max = 30; };  // Water Bottle
    class coffee     { buyPrice = 10; sellPrice = 5; min = 5; max = 30; };  // Coffee
    class apple      { buyPrice = 50; sellPrice = 25; min = 25; max = 150; };  // Apple
    class tbacon     { buyPrice = 75; sellPrice = 37; min = 38; max = 225; };  // Tactical Bacon
    class redgull    { buyPrice = 1500; sellPrice = 750; min = 750; max = 4500; };  // Redgull
    class pickaxe    { buyPrice = 750; sellPrice = 375; min = 375; max = 2250; };  // Pickaxe
    class fuelcan    { buyPrice = 850; sellPrice = 425; min = 425; max = 2550; };  // Fuel Can
    class lockpick   { buyPrice = 150; sellPrice = 75; min = 75; max = 450; };  // Lockpick
    class toolkit    { buyPrice = 350; sellPrice = 175; min = 175; max = 1050; };  // Toolkit
    class zipties    { buyPrice = 500; sellPrice = 250; min = 250; max = 1500; };  // Zip Ties
    class donut      { buyPrice = 25; sellPrice = 12; min = 12; max = 75; };  // Donut
    class spikestrip { buyPrice = 2500; sellPrice = 1250; min = 1250; max = 7500; level = 1; };  // Spike Strip
    class cone       { buyPrice = 50; sellPrice = 25; min = 25; max = 150; };  // Road Cone
    class barrier    { buyPrice = 150; sellPrice = 75; min = 75; max = 450; };  // Road Barrier
    class roadlight  { buyPrice = 250; sellPrice = 125; min = 125; max = 750; };  // Portable Light
    class arrowsign  { buyPrice = 250; sellPrice = 125; min = 125; max = 750; };  // Arrow Sign
    class medkit     { buyPrice = 250; sellPrice = 125; min = 125; max = 750; };  // Medkit
    class firstaid   { buyPrice = 75; sellPrice = 37; min = 37; max = 225; };  // First Aid Kit
    class defib      { buyPrice = 2500; sellPrice = 1250; min = 1250; max = 7500; };  // Defibrillator

    // ---- Resources: the Resource Trader buys them (sellPrice); nobody sells them ----
    class oil_raw     { buyPrice = 1000; sellPrice = 500; min = 400; max = 2000; };  // Crude Oil
    class oil_refined { buyPrice = 6400; sellPrice = 3200; min = 2560; max = 12800; };  // Refined Oil
    class copper_ore  { buyPrice = 600; sellPrice = 300; min = 240; max = 1200; };  // Copper Ore
    class copper_bar  { buyPrice = 3000; sellPrice = 1500; min = 1200; max = 6000; };  // Copper Bar
    class iron_ore    { buyPrice = 700; sellPrice = 350; min = 280; max = 1400; };  // Iron Ore
    class iron_bar    { buyPrice = 4000; sellPrice = 2000; min = 1600; max = 8000; };  // Iron Bar
    class salt_raw    { buyPrice = 400; sellPrice = 200; min = 160; max = 800; };  // Rock Salt
    class salt        { buyPrice = 2600; sellPrice = 1300; min = 1040; max = 5200; };  // Refined Salt
    class sand        { buyPrice = 300; sellPrice = 150; min = 120; max = 600; };  // Sand
    class glass       { buyPrice = 2900; sellPrice = 1450; min = 1160; max = 5800; };  // Glass
    class diamond_raw { buyPrice = 1500; sellPrice = 750; min = 600; max = 3000; };  // Uncut Diamond
    class diamond     { buyPrice = 7000; sellPrice = 3500; min = 2800; max = 14000; };  // Cut Diamond
    class rock        { buyPrice = 300; sellPrice = 150; min = 120; max = 600; };  // Rock
    class cement      { buyPrice = 3900; sellPrice = 1950; min = 1560; max = 7800; };  // Cement Bag
};
