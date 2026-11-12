// Weapon shop (ALife_fnc_weaponShop). The middle of the screen is the
// selected item on a dark table in 3D (drag to turn it, scroll to zoom);
// the shop on the left, the kit and your cart on the right.
//
//   +- POLICE ARMOURY ----+                          +- Taser Pistol ---------+
//   | Bank $  Cash $      |                          | Weapon $2,000          |
//   | [Weapons][Mags]...  |                          | Muzzle   [None      v] |
//   | Search [_________]  |        (3D preview)      | Light    [Flashlight v]|
//   | - TASERS (2)        |                          | Optic    [None      v] |
//   |      Taser Pistol   |                          | Bipod    [None      v] |
//   |      Taser Rifle    |                          | Mags     [Cartr.  v][2]|
//   | + PISTOLS (1)       |                          | Kit total       $2,830 |
//   |  ...                |                          | [  Add kit to cart   ] |
//   |                     |                          | YOUR CART       $2,830 |
//   | <info>              |                          | ... Remove / Clear     |
//   +---------------------+                          | Pay cash  /  Pay bank  |
//                                                    +------------------------+
//
// Control IDs match the #defines in functions\client\fn_weaponShop.sqf.

#define WS_LX "safezoneX + safezoneW * 0.015"
#define WS_LIX "safezoneX + safezoneW * 0.025"
#define WS_LW "safezoneW * 0.255"
#define WS_LIW "safezoneW * 0.235"
#define WS_RX "safezoneX + safezoneW * 0.73"
#define WS_RIX "safezoneX + safezoneW * 0.74"
#define WS_RW "safezoneW * 0.255"
#define WS_RIW "safezoneW * 0.235"
#define WS_FIELD_X "safezoneX + safezoneW * 0.80"
#define WS_ROW_H "safezoneH * 0.032"

class ALife_WeaponShop
{
    idd = 5150;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_weaponShop";
    onUnload = "['unload'] call ALife_fnc_weaponShop";

    class controlsBackground
    {
        class LeftPanel: ALife_RscBackground
        {
            colorBackground[] = { 0.059, 0.09, 0.165, 0.94 };
            x = WS_LX;
            y = "safezoneY + safezoneH * 0.06";
            w = WS_LW;
            h = "safezoneH * 0.88";
        };
        class LeftHeader: LeftPanel
        {
            colorBackground[] = { 0.035, 0.055, 0.105, 1 };
            h = "safezoneH * 0.075";
        };
        class LeftAccent: LeftPanel
        {
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            h = "safezoneH * 0.003";
        };
        class RightPanel: LeftPanel
        {
            x = WS_RX;
            w = WS_RW;
        };
        class RightHeader: LeftHeader
        {
            x = WS_RX;
            w = WS_RW;
        };
        class RightAccent: LeftAccent
        {
            x = WS_RX;
            w = WS_RW;
        };
        class CartHeader: RightHeader
        {
            y = "safezoneY + safezoneH * 0.443";
            h = "safezoneH * 0.038";
        };
    };

    class controls
    {
        class Hint: ALife_RscStructuredText
        {
            idc = 5180;
            size = 0.03;
            x = "safezoneX + safezoneW * 0.3";
            y = "safezoneY + safezoneH * 0.9";
            w = "safezoneW * 0.4";
            h = "safezoneH * 0.03";
        };

        // ---- The shop (left) -------------------------------------------------
        class Title: ALife_RscText
        {
            idc = 5151;
            font = "RobotoCondensedBold";
            sizeEx = 0.038;
            x = WS_LIX;
            y = "safezoneY + safezoneH * 0.066";
            w = WS_LIW;
            h = "safezoneH * 0.035";
        };
        class Balance: ALife_RscStructuredText
        {
            idc = 5152;
            size = 0.028;
            x = WS_LIX;
            y = "safezoneY + safezoneH * 0.1";
            w = WS_LIW;
            h = "safezoneH * 0.028";
        };
        class Tab0: ALife_RscButton
        {
            idc = 5160;
            text = "Weapons";
            sizeEx = 0.029;
            colorBackground[] = { 1, 1, 1, 0.08 };
            colorBackgroundActive[] = { 1, 1, 1, 0.18 };
            colorFocused[] = { 1, 1, 1, 0.08 };
            action = "['tab', 0] call ALife_fnc_weaponShop";
            x = WS_LIX;
            y = "safezoneY + safezoneH * 0.145";
            w = "safezoneW * 0.0565";
            h = WS_ROW_H;
        };
        class Tab1: Tab0
        {
            idc = 5161;
            text = "Magazines";
            action = "['tab', 1] call ALife_fnc_weaponShop";
            x = "safezoneX + safezoneW * 0.0845";
        };
        class Tab2: Tab0
        {
            idc = 5162;
            text = "Attachments";
            action = "['tab', 2] call ALife_fnc_weaponShop";
            x = "safezoneX + safezoneW * 0.144";
        };
        class Tab3: Tab0
        {
            idc = 5163;
            text = "Items";
            action = "['tab', 3] call ALife_fnc_weaponShop";
            x = "safezoneX + safezoneW * 0.2035";
        };
        class SearchLabel: ALife_RscText
        {
            text = "Search";
            sizeEx = 0.03;
            colorText[] = { 0.545, 0.596, 0.702, 1 };
            x = WS_LIX;
            y = "safezoneY + safezoneH * 0.187";
            w = "safezoneW * 0.04";
            h = "safezoneH * 0.03";
        };
        class Search: ALife_RscEdit
        {
            idc = 5164;
            sizeEx = 0.03;
            x = "safezoneX + safezoneW * 0.065";
            y = "safezoneY + safezoneH * 0.187";
            w = "safezoneW * 0.195";
            h = "safezoneH * 0.03";
        };
        class List: ALife_RscListNBox
        {
            idc = 5165;
            sizeEx = 0.031;
            rowHeight = 0.045;
            columns[] = { 0, 0.72 };
            onLBSelChanged = "['select'] call ALife_fnc_weaponShop";
            onLBDblClick = "['add', 'list'] call ALife_fnc_weaponShop";
            x = WS_LIX;
            y = "safezoneY + safezoneH * 0.225";
            w = WS_LIW;
            h = "safezoneH * 0.64";
        };
        class Info: ALife_RscStructuredText
        {
            idc = 5166;
            size = 0.028;
            x = WS_LIX;
            y = "safezoneY + safezoneH * 0.872";
            w = WS_LIW;
            h = "safezoneH * 0.06";
        };

        // ---- The kit (right, top) --------------------------------------------
        class KitTitle: ALife_RscStructuredText
        {
            idc = 5190;
            size = 0.03;
            x = WS_RIX;
            y = "safezoneY + safezoneH * 0.064";
            w = WS_RIW;
            h = "safezoneH * 0.068";
        };
        class SlotLabel0: ALife_RscText
        {
            idc = 5181;
            text = "Muzzle";
            sizeEx = 0.03;
            colorText[] = { 0.545, 0.596, 0.702, 1 };
            x = WS_RIX;
            y = "safezoneY + safezoneH * 0.145";
            w = "safezoneW * 0.06";
            h = WS_ROW_H;
        };
        class SlotLabel1: SlotLabel0
        {
            idc = 5182;
            text = "Light / laser";
            y = "safezoneY + safezoneH * 0.185";
        };
        class SlotLabel2: SlotLabel0
        {
            idc = 5183;
            text = "Optic";
            y = "safezoneY + safezoneH * 0.225";
        };
        class SlotLabel3: SlotLabel0
        {
            idc = 5184;
            text = "Bipod";
            y = "safezoneY + safezoneH * 0.265";
        };
        class MagLabel: SlotLabel0
        {
            idc = 5186;
            text = "Magazines";
            y = "safezoneY + safezoneH * 0.305";
        };
        class Slot0: ALife_RscCombo
        {
            idc = 5191;
            sizeEx = 0.029;
            x = WS_FIELD_X;
            y = "safezoneY + safezoneH * 0.145";
            w = "safezoneW * 0.175";
            h = WS_ROW_H;
        };
        class Slot1: Slot0
        {
            idc = 5192;
            y = "safezoneY + safezoneH * 0.185";
        };
        class Slot2: Slot0
        {
            idc = 5193;
            y = "safezoneY + safezoneH * 0.225";
        };
        class Slot3: Slot0
        {
            idc = 5194;
            y = "safezoneY + safezoneH * 0.265";
        };
        class Mag: Slot0
        {
            idc = 5195;
            y = "safezoneY + safezoneH * 0.305";
            w = "safezoneW * 0.13";
        };
        class MagCount: ALife_RscEdit
        {
            idc = 5196;
            text = "2";
            maxChars = 2;
            sizeEx = 0.03;
            tooltip = "How many magazines (0-10); the first goes in the weapon";
            x = "safezoneX + safezoneW * 0.935";
            y = "safezoneY + safezoneH * 0.305";
            w = "safezoneW * 0.04";
            h = WS_ROW_H;
        };
        class QtyLabel: SlotLabel0
        {
            idc = 5169;
            text = "Quantity";
        };
        class Qty: ALife_RscEdit
        {
            idc = 5168;
            text = "1";
            maxChars = 2;
            sizeEx = 0.03;
            x = WS_FIELD_X;
            y = "safezoneY + safezoneH * 0.145";
            w = "safezoneW * 0.04";
            h = WS_ROW_H;
        };
        class KitTotal: ALife_RscStructuredText
        {
            idc = 5197;
            size = 0.03;
            x = WS_RIX;
            y = "safezoneY + safezoneH * 0.348";
            w = WS_RIW;
            h = "safezoneH * 0.032";
        };
        class Add: ALife_RscButton
        {
            idc = 5167;
            text = "Add to cart";
            font = "RobotoCondensedBold";
            sizeEx = 0.031;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 1, 0.72, 0.2, 1 };
            colorFocused[] = { 1, 0.72, 0.2, 1 };
            colorText[] = { 0.118, 0.071, 0, 1 };
            action = "['add'] call ALife_fnc_weaponShop";
            x = WS_RIX;
            y = "safezoneY + safezoneH * 0.388";
            w = WS_RIW;
            h = "safezoneH * 0.038";
        };

        // ---- Your cart (right, bottom) ---------------------------------------
        class CartTitle: ALife_RscStructuredText
        {
            idc = 5170;
            size = 0.03;
            x = WS_RIX;
            y = "safezoneY + safezoneH * 0.448";
            w = WS_RIW;
            h = "safezoneH * 0.028";
        };
        class CartList: ALife_RscListNBox
        {
            idc = 5171;
            sizeEx = 0.029;
            rowHeight = 0.04;
            columns[] = { 0, 0.66, 0.76 };
            onLBDblClick = "['remove'] call ALife_fnc_weaponShop";
            x = WS_RIX;
            y = "safezoneY + safezoneH * 0.49";
            w = WS_RIW;
            h = "safezoneH * 0.33";
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
            x = WS_RIX;
            y = "safezoneY + safezoneH * 0.828";
            w = "safezoneW * 0.1155";
            h = "safezoneH * 0.03";
        };
        class Clear: Remove
        {
            idc = 5173;
            text = "Clear";
            action = "['clear'] call ALife_fnc_weaponShop";
            x = "safezoneX + safezoneW * 0.8595";
        };
        class PayCash: Remove
        {
            idc = 5174;
            text = "Pay cash";
            action = "['checkout', 'cash'] call ALife_fnc_weaponShop";
            y = "safezoneY + safezoneH * 0.864";
            h = "safezoneH * 0.034";
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
            x = "safezoneX + safezoneW * 0.8595";
        };
        class Close: Remove
        {
            idc = 5176;
            text = "Close";
            colorBackground[] = { 0, 0, 0, 0 };
            colorText[] = { 0.75, 0.78, 0.82, 1 };
            action = "closeDialog 0";
            y = "safezoneY + safezoneH * 0.903";
            w = WS_RIW;
        };
    };
};
