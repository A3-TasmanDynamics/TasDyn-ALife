// Virtual items: things you carry that aren't Arma gear. Stored per faction
// in players.<faction>_items as [[class,count],...]. Every change goes
// through the server (ALife_fnc_itemsChange).
//
//   displayName -- shown in the phone's Inventory app and shops
//   description -- one line, shown when selected
//   icon        -- picture path
//   weight      -- counts towards carry weight (CfgALife >> carryWeight) and
//                  vehicle trunk space

class CfgItems
{
    class medkit
    {
        displayName = "Medkit";
        description = "Medics use one to revive an incapacitated player.";
        icon = "\A3\ui_f\data\igui\cfg\actions\heal_ca.paa";
        weight = 2;
    };
};
