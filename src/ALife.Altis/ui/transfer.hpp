// Inventory screen (Windows key): dropped bags and vehicle trunks on the
// left, the player on the right. Driven by ALife_fnc_transfer; control IDs
// match the #defines there. Both lists allow several rows to be selected
// (Ctrl/Shift-click).
//
//   Vehicle Trunk - Offroad                         Trunk: 12 / 50
//   Trunk                         | Your inventory  (6 / 24)
//   [3] Medkit                    | $1,200  Cash
//                                 | [3] Medkit
//   [ 1 ] [Take] [Take all]       | [ 1 ] [Store] [Store all]
//   [Close]

class ALife_Transfer
{
    idd = 5300;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_transfer";

    class controlsBackground
    {
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.059, 0.09, 0.165, 0.95 };
            x = "safezoneX + safezoneW * 0.27";
            y = "safezoneY + safezoneH * 0.27";
            w = "safezoneW * 0.46";
            h = "safezoneH * 0.45";
        };
        class Header: ALife_RscBackground
        {
            colorBackground[] = { 1, 1, 1, 0.05 };
            x = "safezoneX + safezoneW * 0.27";
            y = "safezoneY + safezoneH * 0.27";
            w = "safezoneW * 0.46";
            h = "safezoneH * 0.042";
        };
        class Accent: ALife_RscBackground
        {
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            x = "safezoneX + safezoneW * 0.27";
            y = "safezoneY + safezoneH * 0.312";
            w = "safezoneW * 0.46";
            h = "safezoneH * 0.003";
        };
    };

    class controls
    {
        class Title: ALife_RscText
        {
            idc = 5301;
            font = "RobotoCondensedBold";
            sizeEx = 0.036;
            x = "safezoneX + safezoneW * 0.277";
            y = "safezoneY + safezoneH * 0.272";
            w = "safezoneW * 0.3";
            h = "safezoneH * 0.038";
        };
        class Weight: ALife_RscText
        {
            idc = 5302;
            style = 1; // right
            sizeEx = 0.032;
            colorText[] = { 0.75, 0.78, 0.82, 1 };
            x = "safezoneX + safezoneW * 0.55";
            y = "safezoneY + safezoneH * 0.272";
            w = "safezoneW * 0.173";
            h = "safezoneH * 0.038";
        };

        // Left: the bag / trunk.
        class ContainerLabel: ALife_RscText
        {
            idc = 5310;
            font = "RobotoCondensedBold";
            sizeEx = 0.03;
            colorText[] = { 0.75, 0.78, 0.82, 1 };
            x = "safezoneX + safezoneW * 0.277";
            y = "safezoneY + safezoneH * 0.322";
            w = "safezoneW * 0.22";
            h = "safezoneH * 0.03";
        };
        class List5311: ALife_RscListBox
        {
            idc = 5311;
            style = 32; // LB_MULTI: select several rows
            sizeEx = 0.03;
            rowHeight = 0.042;
            colorBackground[] = { 1, 1, 1, 0.04 };
            colorSelectBackground[] = { 0.96, 0.62, 0.04, 0.9 };
            colorSelectBackground2[] = { 0.96, 0.62, 0.04, 0.9 };
            colorSelect[] = { 0.118, 0.071, 0, 1 };
            colorSelect2[] = { 0.118, 0.071, 0, 1 };
            onLBDblClick = "['moveRow', 'container', _this select 1] call ALife_fnc_transfer";
            x = "safezoneX + safezoneW * 0.277";
            y = "safezoneY + safezoneH * 0.355";
            w = "safezoneW * 0.22";
            h = "safezoneH * 0.27";
        };
        class ContainerQty: ALife_RscEdit
        {
            idc = 5312;
            text = "1";
            maxChars = 9;
            sizeEx = 0.032;
            colorBackground[] = { 1, 1, 1, 0.07 };
            x = "safezoneX + safezoneW * 0.277";
            y = "safezoneY + safezoneH * 0.635";
            w = "safezoneW * 0.05";
            h = "safezoneH * 0.035";
        };
        class Take: ALife_RscButton
        {
            text = "Take";
            font = "RobotoCondensedBold";
            sizeEx = 0.032;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 1, 0.72, 0.2, 1 };
            colorFocused[] = { 1, 0.72, 0.2, 1 };
            colorText[] = { 0.118, 0.071, 0, 1 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "['move', 'container', false] call ALife_fnc_transfer";
            x = "safezoneX + safezoneW * 0.331";
            y = "safezoneY + safezoneH * 0.635";
            w = "safezoneW * 0.08";
            h = "safezoneH * 0.035";
        };
        class TakeAll: ALife_RscButton
        {
            text = "Take all";
            sizeEx = 0.032;
            colorBackground[] = { 1, 1, 1, 0.1 };
            colorBackgroundActive[] = { 1, 1, 1, 0.2 };
            colorFocused[] = { 1, 1, 1, 0.14 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "['move', 'container', true] call ALife_fnc_transfer";
            x = "safezoneX + safezoneW * 0.415";
            y = "safezoneY + safezoneH * 0.635";
            w = "safezoneW * 0.082";
            h = "safezoneH * 0.035";
        };

        // Right: the player.
        class PlayerLabel: ALife_RscText
        {
            idc = 5320;
            font = "RobotoCondensedBold";
            sizeEx = 0.03;
            colorText[] = { 0.75, 0.78, 0.82, 1 };
            x = "safezoneX + safezoneW * 0.503";
            y = "safezoneY + safezoneH * 0.322";
            w = "safezoneW * 0.22";
            h = "safezoneH * 0.03";
        };
        class List5321: ALife_RscListBox
        {
            idc = 5321;
            style = 32; // LB_MULTI: select several rows
            sizeEx = 0.03;
            rowHeight = 0.042;
            colorBackground[] = { 1, 1, 1, 0.04 };
            colorSelectBackground[] = { 0.96, 0.62, 0.04, 0.9 };
            colorSelectBackground2[] = { 0.96, 0.62, 0.04, 0.9 };
            colorSelect[] = { 0.118, 0.071, 0, 1 };
            colorSelect2[] = { 0.118, 0.071, 0, 1 };
            onLBDblClick = "['moveRow', 'player', _this select 1] call ALife_fnc_transfer";
            x = "safezoneX + safezoneW * 0.503";
            y = "safezoneY + safezoneH * 0.355";
            w = "safezoneW * 0.22";
            h = "safezoneH * 0.27";
        };
        class PlayerQty: ALife_RscEdit
        {
            idc = 5322;
            text = "1";
            maxChars = 9;
            sizeEx = 0.032;
            colorBackground[] = { 1, 1, 1, 0.07 };
            x = "safezoneX + safezoneW * 0.503";
            y = "safezoneY + safezoneH * 0.635";
            w = "safezoneW * 0.05";
            h = "safezoneH * 0.035";
        };
        class Store: ALife_RscButton
        {
            idc = 5323;
            text = "Store";
            font = "RobotoCondensedBold";
            sizeEx = 0.032;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 1, 0.72, 0.2, 1 };
            colorFocused[] = { 1, 0.72, 0.2, 1 };
            colorText[] = { 0.118, 0.071, 0, 1 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "['move', 'player', false] call ALife_fnc_transfer";
            x = "safezoneX + safezoneW * 0.557";
            y = "safezoneY + safezoneH * 0.635";
            w = "safezoneW * 0.08";
            h = "safezoneH * 0.035";
        };
        class StoreAll: ALife_RscButton
        {
            idc = 5324;
            text = "Store all";
            sizeEx = 0.032;
            colorBackground[] = { 1, 1, 1, 0.1 };
            colorBackgroundActive[] = { 1, 1, 1, 0.2 };
            colorFocused[] = { 1, 1, 1, 0.14 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "['move', 'player', true] call ALife_fnc_transfer";
            x = "safezoneX + safezoneW * 0.641";
            y = "safezoneY + safezoneH * 0.635";
            w = "safezoneW * 0.082";
            h = "safezoneH * 0.035";
        };

        class Hint: ALife_RscText
        {
            text = "Ctrl/Shift-click to pick several. Double-click a row to move all of it.";
            sizeEx = 0.026;
            colorText[] = { 0.545, 0.596, 0.702, 1 };
            x = "safezoneX + safezoneW * 0.352";
            y = "safezoneY + safezoneH * 0.677";
            w = "safezoneW * 0.285";
            h = "safezoneH * 0.035";
        };
        class Use: ALife_RscButton
        {
            idc = 5325;
            text = "Use";
            sizeEx = 0.032;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 0.98, 0.75, 0.14, 1 };
            colorFocused[] = { 0.96, 0.62, 0.04, 1 };
            colorText[] = { 0.118, 0.071, 0, 1 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "['use'] call ALife_fnc_transfer";
            x = "safezoneX + safezoneW * 0.641";
            y = "safezoneY + safezoneH * 0.677";
            w = "safezoneW * 0.082";
            h = "safezoneH * 0.035";
        };
        class Close: ALife_RscButton
        {
            text = "Close";
            sizeEx = 0.032;
            colorBackground[] = { 1, 1, 1, 0.06 };
            colorBackgroundActive[] = { 1, 1, 1, 0.15 };
            colorFocused[] = { 1, 1, 1, 0.1 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "closeDialog 0";
            x = "safezoneX + safezoneW * 0.277";
            y = "safezoneY + safezoneH * 0.677";
            w = "safezoneW * 0.07";
            h = "safezoneH * 0.035";
        };
    };
};
