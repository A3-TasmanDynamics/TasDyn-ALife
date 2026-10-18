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
    //   name     -- shown in the shop window
    //   side     -- "civ" / "cop" / "medic": who can use it
    //   licence  -- needed to use the shop at all (CfgLicences key, "" = none)
    //   level    -- minimum cop_level / medic_level to use it (0 = any)
    //   items[]  -- CfgMarket / CfgItems classes

    class general
    {
        name = "Market";
        side = "civ";
        licence = "";
        level = 0;
        items[] = { "water", "coffee", "apple", "tbacon", "redgull", "pickaxe", "fuelcan", "lockpick", "toolkit" };
    };

    class rebel
    {
        name = "Rebel Market";
        side = "civ";
        licence = "rebel";
        level = 0;
        items[] = { "water", "tbacon", "lockpick", "zipties", "toolkit", "fuelcan" };
    };

    class cop
    {
        name = "Police Market";
        side = "cop";
        licence = "";
        level = 0;
        items[] = { "water", "coffee", "donut", "toolkit", "fuelcan", "spikestrip" };
    };

    class medic
    {
        name = "Medical Supplies";
        side = "medic";
        licence = "";
        level = 0;
        items[] = { "medkit", "water", "coffee", "toolkit", "fuelcan" };
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
    class water      { buyPrice = 10; sellPrice = 5; min = 5; max = 30; };
    class coffee     { buyPrice = 10; sellPrice = 5; min = 5; max = 30; };
    class apple      { buyPrice = 50; sellPrice = 25; min = 25; max = 150; };
    class tbacon     { buyPrice = 75; sellPrice = 37; min = 38; max = 225; };
    class redgull    { buyPrice = 1500; sellPrice = 750; min = 750; max = 4500; };
    class pickaxe    { buyPrice = 750; sellPrice = 375; min = 375; max = 2250; };
    class fuelcan    { buyPrice = 850; sellPrice = 425; min = 425; max = 2550; };
    class lockpick   { buyPrice = 150; sellPrice = 75; min = 75; max = 450; };
    class toolkit    { buyPrice = 350; sellPrice = 175; min = 175; max = 1050; };
    class zipties    { buyPrice = 500; sellPrice = 250; min = 250; max = 1500; };
    class donut      { buyPrice = 25; sellPrice = 12; min = 12; max = 75; };
    class spikestrip { buyPrice = 2500; sellPrice = 1250; min = 1250; max = 7500; level = 1; };
    class medkit     { buyPrice = 250; sellPrice = 125; min = 125; max = 750; };
};
