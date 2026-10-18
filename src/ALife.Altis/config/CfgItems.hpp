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

class CfgItems
{
    // ---- Food and drink ----
    class water
    {
        displayName = "Water Bottle";
        description = "Bottled water.";
        icon = "ui\items\water_ca.paa";
        weight = 1;
    };
    class coffee
    {
        displayName = "Coffee";
        description = "A takeaway coffee.";
        icon = "ui\items\coffee_ca.paa";
        weight = 1;
    };
    class donut
    {
        displayName = "Donut";
        description = "Glazed. A police favourite.";
        icon = "ui\items\donut_ca.paa";
        weight = 1;
    };
    class apple
    {
        displayName = "Apple";
        description = "Fresh from an Altis orchard.";
        icon = "ui\items\apple_ca.paa";
        weight = 1;
    };
    class tbacon
    {
        displayName = "Tactical Bacon";
        description = "Bacon in a can. Keeps forever.";
        icon = "ui\items\tbacon_ca.paa";
        weight = 1;
    };
    class redgull
    {
        displayName = "Redgull";
        description = "Energy drink. Pricey.";
        icon = "ui\items\redgull_ca.paa";
        weight = 1;
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
    };
    class lockpick
    {
        displayName = "Lockpick";
        description = "Opens locked vehicles. Illegal to use.";
        icon = "ui\items\lockpick_ca.paa";
        weight = 1;
    };
    class repairkit
    {
        displayName = "Repair Kit";
        description = "Patches up a damaged vehicle.";
        icon = "ui\items\repairkit_ca.paa";
        weight = 4;
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
        description = "Medics use one to revive an incapacitated player.";
        icon = "ui\items\medkit_ca.paa";
        weight = 2;
    };
};
