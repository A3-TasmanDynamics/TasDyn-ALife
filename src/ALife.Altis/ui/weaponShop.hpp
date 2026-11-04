// Weapon shop (ALife_fnc_weaponShop): the selected item lies on a dark
// table in 3D filling the screen (drag to turn it, scroll to zoom), with
// two panels along the bottom -- the shop and your cart.
//
//   +- POLICE ARMOURY ------- Bank $ Cash $ -+  +- YOUR CART ---- $4,080 -+
//   | [Weapons][Magazines][Attachments][Items]  |  Taser Pistol   1  $2,000 |
//   | Search [__________]                     |  |  Taser Cart.   2     $80 |
//   | TASERS                                  |  |                          |
//   |    Taser Pistol                 $2,000  |  | [Remove]      [Clear]    |
//   |    ...                                  |  | [Pay cash] [Pay bank]    |
//   | Taser Pistol  Takes: Taser Cartridges   |  | [Close]                  |
//   | Qty [1]                 [Add to cart]   |  +--------------------------+
//   +-----------------------------------------+
//
// Control IDs match the #defines in functions\client\fn_weaponShop.sqf.

#define WS_X "safezoneX + safezoneW * 0.215"
#define WS_W "safezoneW * 0.33"
#define WS_IX "safezoneX + safezoneW * 0.222"
#define WS_IW "safezoneW * 0.316"
#define WC_X "safezoneX + safezoneW * 0.552"
#define WC_W "safezoneW * 0.233"
#define WC_IX "safezoneX + safezoneW * 0.559"
#define WC_IW "safezoneW * 0.219"

class ALife_WeaponShop
{
    idd = 5150;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_weaponShop";
    onUnload = "['unload'] call ALife_fnc_weaponShop";

    class controlsBackground
    {
        class ShopPanel: ALife_RscBackground
        {
            colorBackground[] = { 0.059, 0.09, 0.165, 0.94 };
            x = WS_X;
            y = "safezoneY + safezoneH * 0.58";
            w = WS_W;
            h = "safezoneH * 0.385";
        };
        class ShopHeader: ShopPanel
        {
            colorBackground[] = { 0.035, 0.055, 0.105, 1 };
            h = "safezoneH * 0.036";
        };
        class ShopAccent: ShopPanel
        {
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            h = "safezoneH * 0.003";
        };
        class CartPanel: ShopPanel
        {
            x = WC_X;
            w = WC_W;
        };
        class CartHeader: ShopHeader
        {
            x = WC_X;
            w = WC_W;
        };
        class CartAccent: ShopAccent
        {
            x = WC_X;
            w = WC_W;
        };
    };

    class controls
    {
        class Hint: ALife_RscStructuredText
        {
            idc = 5180;
            size = 0.03;
            x = WS_X;
            y = "safezoneY + safezoneH * 0.545";
            w = "safezoneW * 0.57";
            h = "safezoneH * 0.03";
        };
        // ---- The shop ------------------------------------------------------
        class Title: ALife_RscText
        {
            idc = 5151;
            font = "RobotoCondensedBold";
            sizeEx = 0.036;
            x = WS_IX;
            y = "safezoneY + safezoneH * 0.584";
            w = "safezoneW * 0.15";
            h = "safezoneH * 0.03";
        };
        class Balance: ALife_RscStructuredText
        {
            idc = 5152;
            size = 0.028;
            x = "safezoneX + safezoneW * 0.37";
            y = "safezoneY + safezoneH * 0.588";
            w = "safezoneW * 0.168";
            h = "safezoneH * 0.026";
        };
        class Tab0: ALife_RscButton
        {
            idc = 5160;
            text = "Weapons";
            sizeEx = 0.03;
            colorBackground[] = { 1, 1, 1, 0.08 };
            colorBackgroundActive[] = { 1, 1, 1, 0.18 };
            colorFocused[] = { 1, 1, 1, 0.08 };
            action = "['tab', 0] call ALife_fnc_weaponShop";
            x = WS_IX;
            y = "safezoneY + safezoneH * 0.624";
            w = "safezoneW * 0.0775";
            h = "safezoneH * 0.03";
        };
        class Tab1: Tab0
        {
            idc = 5161;
            text = "Magazines";
            action = "['tab', 1] call ALife_fnc_weaponShop";
            x = "safezoneX + safezoneW * 0.3015";
        };
        class Tab2: Tab0
        {
            idc = 5162;
            text = "Attachments";
            action = "['tab', 2] call ALife_fnc_weaponShop";
            x = "safezoneX + safezoneW * 0.381";
        };
        class Tab3: Tab0
        {
            idc = 5163;
            text = "Items";
            action = "['tab', 3] call ALife_fnc_weaponShop";
            x = "safezoneX + safezoneW * 0.4605";
        };
        class SearchLabel: ALife_RscText
        {
            text = "Search";
            sizeEx = 0.03;
            colorText[] = { 0.545, 0.596, 0.702, 1 };
            x = WS_IX;
            y = "safezoneY + safezoneH * 0.662";
            w = "safezoneW * 0.035";
            h = "safezoneH * 0.03";
        };
        class Search: ALife_RscEdit
        {
            idc = 5164;
            sizeEx = 0.03;
            x = "safezoneX + safezoneW * 0.258";
            y = "safezoneY + safezoneH * 0.662";
            w = "safezoneW * 0.28";
            h = "safezoneH * 0.03";
        };
        class List: ALife_RscListNBox
        {
            idc = 5165;
            sizeEx = 0.031;
            rowHeight = 0.045;
            columns[] = { 0, 0.72 };
            onLBSelChanged = "['select'] call ALife_fnc_weaponShop";
            onLBDblClick = "['add'] call ALife_fnc_weaponShop";
            x = WS_IX;
            y = "safezoneY + safezoneH * 0.7";
            w = WS_IW;
            h = "safezoneH * 0.185";
        };
        class Info: ALife_RscStructuredText
        {
            idc = 5166;
            size = 0.028;
            x = WS_IX;
            y = "safezoneY + safezoneH * 0.89";
            w = WS_IW;
            h = "safezoneH * 0.034";
        };
        class QtyLabel: ALife_RscText
        {
            text = "Qty";
            sizeEx = 0.03;
            colorText[] = { 0.545, 0.596, 0.702, 1 };
            x = WS_IX;
            y = "safezoneY + safezoneH * 0.927";
            w = "safezoneW * 0.02";
            h = "safezoneH * 0.03";
        };
        class Qty: ALife_RscEdit
        {
            idc = 5168;
            text = "1";
            maxChars = 2;
            sizeEx = 0.03;
            x = "safezoneX + safezoneW * 0.242";
            y = "safezoneY + safezoneH * 0.927";
            w = "safezoneW * 0.025";
            h = "safezoneH * 0.03";
        };
        class Add: ALife_RscButton
        {
            idc = 5167;
            text = "Add to cart";
            font = "RobotoCondensedBold";
            sizeEx = 0.03;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 1, 0.72, 0.2, 1 };
            colorFocused[] = { 1, 0.72, 0.2, 1 };
            colorText[] = { 0.118, 0.071, 0, 1 };
            action = "['add'] call ALife_fnc_weaponShop";
            x = "safezoneX + safezoneW * 0.458";
            y = "safezoneY + safezoneH * 0.927";
            w = "safezoneW * 0.08";
            h = "safezoneH * 0.03";
        };

        // ---- Your cart -----------------------------------------------------
        class CartTitle: ALife_RscStructuredText
        {
            idc = 5170;
            size = 0.03;
            x = WC_IX;
            y = "safezoneY + safezoneH * 0.587";
            w = WC_IW;
            h = "safezoneH * 0.028";
        };
        class CartList: ALife_RscListNBox
        {
            idc = 5171;
            sizeEx = 0.03;
            rowHeight = 0.042;
            columns[] = { 0, 0.62, 0.74 };
            onLBDblClick = "['remove'] call ALife_fnc_weaponShop";
            x = WC_IX;
            y = "safezoneY + safezoneH * 0.624";
            w = WC_IW;
            h = "safezoneH * 0.22";
        };
        class Remove: ALife_RscButton
        {
            idc = 5172;
            text = "Remove";
            sizeEx = 0.03;
            colorBackground[] = { 1, 1, 1, 0.1 };
            colorBackgroundActive[] = { 1, 1, 1, 0.2 };
            colorFocused[] = { 1, 1, 1, 0.14 };
            action = "['remove'] call ALife_fnc_weaponShop";
            x = WC_IX;
            y = "safezoneY + safezoneH * 0.852";
            w = "safezoneW * 0.107";
            h = "safezoneH * 0.03";
        };
        class Clear: Remove
        {
            idc = 5173;
            text = "Clear";
            action = "['clear'] call ALife_fnc_weaponShop";
            x = "safezoneX + safezoneW * 0.671";
        };
        class PayCash: Remove
        {
            idc = 5174;
            text = "Pay cash";
            action = "['checkout', 'cash'] call ALife_fnc_weaponShop";
            y = "safezoneY + safezoneH * 0.889";
        };
        class PayBank: PayCash
        {
            idc = 5175;
            text = "Pay bank";
            font = "RobotoCondensedBold";
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 1, 0.72, 0.2, 1 };
            colorFocused[] = { 1, 0.72, 0.2, 1 };
            colorText[] = { 0.118, 0.071, 0, 1 };
            action = "['checkout', 'bank'] call ALife_fnc_weaponShop";
            x = "safezoneX + safezoneW * 0.671";
        };
        class Close: Remove
        {
            idc = 5176;
            text = "Close";
            colorBackground[] = { 0, 0, 0, 0 };
            colorText[] = { 0.75, 0.78, 0.82, 1 };
            action = "closeDialog 0";
            y = "safezoneY + safezoneH * 0.927";
            w = WC_IW;
        };
    };
};
