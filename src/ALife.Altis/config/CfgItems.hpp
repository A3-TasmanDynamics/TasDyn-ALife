// Virtual items: things you carry that aren't Arma gear. Stored per faction
// in players.<faction>_items as [[class,count],...]. Every change goes
// through the server (ALife_fnc_itemsChange). Prices are in CfgMarket;
// which shops sell them is in CfgShops (items[]).
//
//   displayName -- shown in the phone's Inventory app and shops
//   description -- one line, shown when selected
//   icon        -- picture path (tools/phone_icons/make_items.py)
//   weight      -- counts towards carry weight (CfgALife >> carryWeight) and
//                  vehicle trunk space
//   use         -- what the Use button does (ALife_fnc_useItem), one used up:
//                    "consume"  eat / drink: `food` / `water` back (of 100, CfgALife >>
//                               Survival), energy = 1 resets stamina, heal = health (0-1)
//                    "heal"     heal yourself by `heal`
//                    "repair"   fully repair the vehicle you're next to
//                    "refuel"   add `fuel` (0-1 of a tank) to the vehicle you're next to
//                    "lockpick" unlock the locked vehicle you're next to, with `chance`
//                  useTime: seconds with a progress bar (moving cancels). No `use`
//                  = not usable from the inventory.
//   anim        -- the looped kneeling move played while using it (one the
//                  game blends into, so everyone sees it smoothly: the
//                  AinvPknlMstpSnonWnonDnon_medic0..5 loops), "" = none (vanilla
//                  Arma has no eating / drinking animation)

class CfgItems
{
    // ---- Food and drink ----
    class water
    {
        displayName = "Water Bottle";
        description = "Bottled water.";
        icon = "ui\items\water_ca.paa";
        weight = 1;
        use = "consume"; useTime = 2; water = 45;
    };
    class coffee
    {
        displayName = "Coffee";
        description = "A takeaway coffee.";
        icon = "ui\items\coffee_ca.paa";
        weight = 1;
        use = "consume"; useTime = 2; water = 20; energy = 1;
    };
    class donut
    {
        displayName = "Donut";
        description = "Glazed. A police favourite.";
        icon = "ui\items\donut_ca.paa";
        weight = 1;
        use = "consume"; useTime = 2; food = 20;
    };
    class apple
    {
        displayName = "Apple";
        description = "Fresh from an Altis orchard.";
        icon = "ui\items\apple_ca.paa";
        weight = 1;
        use = "consume"; useTime = 2; food = 15; water = 5;
    };
    class tbacon
    {
        displayName = "Tactical Bacon";
        description = "Bacon in a can. Keeps forever.";
        icon = "ui\items\tbacon_ca.paa";
        weight = 1;
        use = "consume"; useTime = 3; food = 40;
    };
    class redgull
    {
        displayName = "Redgull";
        description = "Energy drink. Pricey.";
        icon = "ui\items\redgull_ca.paa";
        weight = 1;
        use = "consume"; useTime = 2; water = 30; energy = 1;
    };

    // ---- Tools ----
    class pickaxe
    {
        displayName = "Pickaxe";
        description = "For mining rock and ore.";
        icon = "ui\items\pickaxe_ca.paa";
        weight = 2;
    };
    class fuelcan
    {
        displayName = "Fuel Can";
        description = "A full jerry can to refuel a vehicle.";
        icon = "ui\items\fuelcan_ca.paa";
        weight = 5;
        use = "refuel"; useTime = 8; fuel = 0.25;   // a quarter tank
        anim = "AinvPknlMstpSnonWnonDnon_medic0";
    };
    class lockpick
    {
        displayName = "Lockpick";
        description = "Opens locked vehicles. Illegal to use.";
        icon = "ui\items\lockpick_ca.paa";
        weight = 1;
        use = "lockpick"; useTime = 10; chance = 0.35; // used up either way
        anim = "AinvPknlMstpSnonWnonDnon_medic1";
    };
    class toolkit
    {
        displayName = "Toolkit";
        description = "Repairs a damaged vehicle.";
        icon = "ui\items\toolkit_ca.paa";
        weight = 4;
        use = "repair"; useTime = 12;
        anim = "AinvPknlMstpSnonWnonDnon_medic4";
    };
    class zipties
    {
        displayName = "Zip Ties";
        description = "Restrains a player. Rebel market only.";
        icon = "ui\items\zipties_ca.paa";
        weight = 1;
    };
    class spikestrip
    {
        displayName = "Spike Strip";
        description = "Police only. Deploy across a road to stop a vehicle.";
        icon = "ui\items\spikestrip_ca.paa";
        weight = 5;
    };

    // ---- Medical ----
    class medkit
    {
        displayName = "Medkit";
        description = "Heals you. Medics also use one to revive an incapacitated player.";
        icon = "ui\items\medkit_ca.paa";
        weight = 2;
        use = "heal"; useTime = 6; heal = 1;         // yourself; medics also revive with it
        anim = "AinvPknlMstpSnonWnonDnon_medic2";
    };

    // ---- Resources (gathered and processed: CfgResources.hpp) ----
    class oil_raw
    {
        displayName = "Crude Oil";
        description = "Unprocessed oil from an oil field. Refine it at an oil refinery.";
        icon = "ui\items\oil_raw_ca.paa";
        weight = 7;
    };
    class oil_refined
    {
        displayName = "Refined Oil";
        description = "Processed oil, ready to sell or use.";
        icon = "ui\items\oil_refined_ca.paa";
        weight = 6;
    };
    class copper_ore
    {
        displayName = "Copper Ore";
        description = "Unprocessed copper from a copper mine. Smelt it into bars.";
        icon = "ui\items\copper_ore_ca.paa";
        weight = 4;
    };
    class copper_bar
    {
        displayName = "Copper Bar";
        description = "Smelted copper.";
        icon = "ui\items\copper_bar_ca.paa";
        weight = 3;
    };
    class iron_ore
    {
        displayName = "Iron Ore";
        description = "Unprocessed iron from an iron mine. Smelt it into bars.";
        icon = "ui\items\iron_ore_ca.paa";
        weight = 5;
    };
    class iron_bar
    {
        displayName = "Iron Bar";
        description = "Smelted iron.";
        icon = "ui\items\iron_bar_ca.paa";
        weight = 3;
    };
    class salt_raw
    {
        displayName = "Rock Salt";
        description = "Unprocessed salt from a salt flat. Refine it.";
        icon = "ui\items\salt_raw_ca.paa";
        weight = 3;
    };
    class salt
    {
        displayName = "Refined Salt";
        description = "Clean, refined salt.";
        icon = "ui\items\salt_ca.paa";
        weight = 1;
    };
    class sand
    {
        displayName = "Sand";
        description = "Sand from a sand mine. Melt it into glass.";
        icon = "ui\items\sand_ca.paa";
        weight = 3;
    };
    class glass
    {
        displayName = "Glass";
        description = "A pane of glass, made from sand.";
        icon = "ui\items\glass_ca.paa";
        weight = 1;
    };
    class diamond_raw
    {
        displayName = "Uncut Diamond";
        description = "A rough diamond from a diamond mine. Have it cut.";
        icon = "ui\items\diamond_raw_ca.paa";
        weight = 4;
    };
    class diamond
    {
        displayName = "Cut Diamond";
        description = "A cut and polished diamond.";
        icon = "ui\items\diamond_ca.paa";
        weight = 2;
    };
    class rock
    {
        displayName = "Rock";
        description = "Rock from a quarry. Grind it into cement.";
        icon = "ui\items\rock_ca.paa";
        weight = 6;
    };
    class cement
    {
        displayName = "Cement Bag";
        description = "A bag of cement, made from rock.";
        icon = "ui\items\cement_ca.paa";
        weight = 5;
    };
};
