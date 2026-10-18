// Player HUD: health, food and water bars in the bottom-right corner.
// A title layer (inside RscTitles in description.ext), shown and filled by
// ALife_fnc_survival.
//
// Rows: icon 93x1, bar 93x2, value 93x3 (x = 1 health, 2 food, 3 water).

class ALife_Hud
{
    idd = -1;
    duration = 1e+011;
    fadeIn = 0.3;
    fadeOut = 0.3;
    movingEnable = 0;
    onLoad = "uiNamespace setVariable ['ALife_Hud', _this select 0]";
    onUnload = "uiNamespace setVariable ['ALife_Hud', displayNull]";

    class controls
    {
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.043, 0.071, 0.125, 0.72 }; // navy
            x = "safezoneX + safezoneW * 0.855";
            y = "safezoneY + safezoneH * 0.83";
            w = "safezoneW * 0.135";
            h = "safezoneH * 0.112";
        };
        class Stripe: ALife_RscBackground
        {
            colorBackground[] = { 0.984, 0.749, 0.141, 1 }; // amber
            x = "safezoneX + safezoneW * 0.855";
            y = "safezoneY + safezoneH * 0.83";
            w = "safezoneW * 0.002";
            h = "safezoneH * 0.112";
        };
        // Health
        class Icon1: ALife_RscPicture
        {
            idc = 9311;
            text = "ui\items\medkit_ca.paa";
            x = "safezoneX + safezoneW * 0.86";
            y = "safezoneY + safezoneH * 0.836";
            w = "safezoneW * 0.014";
            h = "safezoneH * 0.026";
        };
        class Bar1: ALife_RscProgress
        {
            idc = 9312;
            colorBar[] = { 0.29, 0.81, 0.59, 1 };
            colorBackground[] = { 1, 1, 1, 0.1 };
            colorFrame[] = { 0, 0, 0, 0 };
            shadow = 0;
            x = "safezoneX + safezoneW * 0.878";
            y = "safezoneY + safezoneH * 0.844";
            w = "safezoneW * 0.08";
            h = "safezoneH * 0.01";
        };
        class Value1: ALife_RscText
        {
            idc = 9313;
            style = 1;
            sizeEx = 0.03;
            font = "RobotoCondensedBold";
            x = "safezoneX + safezoneW * 0.958";
            y = "safezoneY + safezoneH * 0.836";
            w = "safezoneW * 0.028";
            h = "safezoneH * 0.026";
        };
        // Food
        class Icon2: ALife_RscPicture
        {
            idc = 9321;
            text = "ui\items\tbacon_ca.paa";
            x = "safezoneX + safezoneW * 0.86";
            y = "safezoneY + safezoneH * 0.870";
            w = "safezoneW * 0.014";
            h = "safezoneH * 0.026";
        };
        class Bar2: ALife_RscProgress
        {
            idc = 9322;
            colorBar[] = { 0.984, 0.749, 0.141, 1 };
            colorBackground[] = { 1, 1, 1, 0.1 };
            colorFrame[] = { 0, 0, 0, 0 };
            shadow = 0;
            x = "safezoneX + safezoneW * 0.878";
            y = "safezoneY + safezoneH * 0.878";
            w = "safezoneW * 0.08";
            h = "safezoneH * 0.01";
        };
        class Value2: ALife_RscText
        {
            idc = 9323;
            style = 1;
            sizeEx = 0.03;
            font = "RobotoCondensedBold";
            x = "safezoneX + safezoneW * 0.958";
            y = "safezoneY + safezoneH * 0.870";
            w = "safezoneW * 0.028";
            h = "safezoneH * 0.026";
        };
        // Water
        class Icon3: ALife_RscPicture
        {
            idc = 9331;
            text = "ui\items\water_ca.paa";
            x = "safezoneX + safezoneW * 0.86";
            y = "safezoneY + safezoneH * 0.904";
            w = "safezoneW * 0.014";
            h = "safezoneH * 0.026";
        };
        class Bar3: ALife_RscProgress
        {
            idc = 9332;
            colorBar[] = { 0.34, 0.62, 1, 1 };
            colorBackground[] = { 1, 1, 1, 0.1 };
            colorFrame[] = { 0, 0, 0, 0 };
            shadow = 0;
            x = "safezoneX + safezoneW * 0.878";
            y = "safezoneY + safezoneH * 0.912";
            w = "safezoneW * 0.08";
            h = "safezoneH * 0.01";
        };
        class Value3: ALife_RscText
        {
            idc = 9333;
            style = 1;
            sizeEx = 0.03;
            font = "RobotoCondensedBold";
            x = "safezoneX + safezoneW * 0.958";
            y = "safezoneY + safezoneH * 0.904";
            w = "safezoneW * 0.028";
            h = "safezoneH * 0.026";
        };
    };
};
