// Shop window (e.g. Medical Supplies). Driven by ALife_fnc_shop; control
// IDs match the #defines in functions\client\fn_shop.sqf.
//
//   MEDICAL SUPPLIES                         Bank $750 | Cash $0
//   Item              Price      You have
//   Medkit            $250       x3
//   <description of the selected item>
//   Qty [1]           [Pay cash]  [Pay bank]           [Close]

class ALife_Shop
{
    idd = 5100;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_shop";

    class controlsBackground
    {
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.06, 0.07, 0.09, 0.96 };
            x = "safezoneX + safezoneW * 0.33";
            y = "safezoneY + safezoneH * 0.28";
            w = "safezoneW * 0.34";
            h = "safezoneH * 0.44";
        };
        class Accent: ALife_RscBackground
        {
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            x = "safezoneX + safezoneW * 0.33";
            y = "safezoneY + safezoneH * 0.28";
            w = "safezoneW * 0.34";
            h = "safezoneH * 0.004";
        };
    };

    class controls
    {
        class Title: ALife_RscText
        {
            idc = 5101;
            font = "RobotoCondensedBold";
            sizeEx = 0.042;
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.295";
            w = "safezoneW * 0.16";
            h = "safezoneH * 0.04";
        };
        class Balance: ALife_RscStructuredText
        {
            idc = 5102;
            size = 0.03;
            x = "safezoneX + safezoneW * 0.5";
            y = "safezoneY + safezoneH * 0.302";
            w = "safezoneW * 0.16";
            h = "safezoneH * 0.03";
        };
        class List: ALife_RscListNBox
        {
            idc = 5110;
            sizeEx = 0.032;
            rowHeight = 0.05;
            columns[] = { 0, 0.55, 0.8 };
            onLBSelChanged = "['select'] call ALife_fnc_shop";
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.35";
            w = "safezoneW * 0.32";
            h = "safezoneH * 0.21";
        };
        class Description: ALife_RscStructuredText
        {
            idc = 5103;
            size = 0.029;
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.57";
            w = "safezoneW * 0.32";
            h = "safezoneH * 0.06";
        };
        class QtyLabel: ALife_RscText
        {
            text = "Qty";
            sizeEx = 0.032;
            colorText[] = { 0.75, 0.78, 0.82, 1 };
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.655";
            w = "safezoneW * 0.02";
            h = "safezoneH * 0.04";
        };
        class Qty: ALife_RscEdit
        {
            idc = 5120;
            text = "1";
            maxChars = 2;
            sizeEx = 0.034;
            x = "safezoneX + safezoneW * 0.362";
            y = "safezoneY + safezoneH * 0.655";
            w = "safezoneW * 0.03";
            h = "safezoneH * 0.04";
        };
        class PayCash: ALife_RscButton
        {
            idc = 5130;
            text = "Pay cash";
            sizeEx = 0.032;
            colorBackground[] = { 1, 1, 1, 0.12 };
            colorBackgroundActive[] = { 1, 1, 1, 0.22 };
            colorFocused[] = { 1, 1, 1, 0.16 };
            action = "['buy', 'cash'] call ALife_fnc_shop";
            x = "safezoneX + safezoneW * 0.4";
            y = "safezoneY + safezoneH * 0.655";
            w = "safezoneW * 0.08";
            h = "safezoneH * 0.04";
        };
        class PayBank: ALife_RscButton
        {
            idc = 5131;
            text = "Pay bank";
            font = "RobotoCondensedBold";
            sizeEx = 0.032;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 1, 0.72, 0.2, 1 };
            colorFocused[] = { 1, 0.72, 0.2, 1 };
            colorText[] = { 0.05, 0.05, 0.05, 1 };
            action = "['buy', 'bank'] call ALife_fnc_shop";
            x = "safezoneX + safezoneW * 0.485";
            y = "safezoneY + safezoneH * 0.655";
            w = "safezoneW * 0.08";
            h = "safezoneH * 0.04";
        };
        class Close: ALife_RscButton
        {
            text = "Close";
            sizeEx = 0.032;
            colorBackground[] = { 0, 0, 0, 0 };
            colorBackgroundActive[] = { 1, 1, 1, 0.1 };
            colorFocused[] = { 0, 0, 0, 0 };
            colorText[] = { 0.75, 0.78, 0.82, 1 };
            action = "closeDialog 0";
            x = "safezoneX + safezoneW * 0.6";
            y = "safezoneY + safezoneH * 0.655";
            w = "safezoneW * 0.06";
            h = "safezoneH * 0.04";
        };
    };
};
