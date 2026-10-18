// The market: virtual items' BASE prices. Prices then move like a real
// market (ALife_fnc_market): every unit bought pushes an item's price up by
// `step`, every unit sold pushes it down, and between trades it drifts back
// toward base by `recovery` of the gap every `recoveryInterval` seconds (new
// stock arriving), always within min..max. Current prices are saved to the
// database (market_prices) and survive restarts; these base prices come back
// only on a wipe or an economy reset (empty market_prices, then restart).
//
// Defaults below; any item may override step / recovery / sellRatio.
//
//   price      -- base price, dollars
//   min / max  -- the price never leaves this range
//   sellRatio  -- what a shop pays when buying the item from a player
//   licence / level -- needed to buy it (as in CfgVehicles; any faction)
//
// What each shop sells is in CfgShops (items[]); what each item is (name,
// icon, weight) is in CfgItems.

class CfgMarket
{
    step = 0.01;              // +1% per unit bought, -1% per unit sold
    recovery = 0.05;          // 5% of the gap back toward base...
    recoveryInterval = 300;   // ...every 5 minutes
    sellRatio = 0.5;          // shops buy back at half the current price

    class water      { price = 10; min = 5; max = 30; };
    class tbacon     { price = 75; min = 38; max = 225; };
    class lockpick   { price = 150; min = 75; max = 450; };
    class zipties    { price = 500; min = 250; max = 1500; };
    class repairkit  { price = 350; min = 175; max = 1050; };
    class fuelcan    { price = 850; min = 425; max = 2550; };
    class coffee     { price = 10; min = 5; max = 30; };
    class donut      { price = 25; min = 12; max = 75; };
    class spikestrip { price = 2500; min = 1250; max = 7500; level = 1; };
    class medkit     { price = 250; min = 125; max = 750; };
    class apple      { price = 50; min = 25; max = 150; };
    class redgull    { price = 1500; min = 750; max = 4500; };
    class pickaxe    { price = 750; min = 375; max = 2250; };
};
