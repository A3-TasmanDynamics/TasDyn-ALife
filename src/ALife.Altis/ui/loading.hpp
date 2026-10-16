// Loading overlay (a title layer, included inside RscTitles in
// description.ext). Shown by ALife_fnc_loading from join until spawn.

class ALife_Loading
{
    idd = -1;
    duration = 1e+011;
    fadeIn = 0;
    fadeOut = 1;
    movingEnable = 0;
    onLoad = "uiNamespace setVariable ['ALife_Loading', _this select 0]";
    onUnload = "uiNamespace setVariable ['ALife_Loading', displayNull]";

    class controls
    {
        class Backdrop: ALife_RscBackground
        {
            colorBackground[] = { 0.035, 0.04, 0.05, 1 };
            x = "safezoneX";
            y = "safezoneY";
            w = "safezoneW";
            h = "safezoneH";
        };
        class Title: ALife_RscText
        {
            style = 2;
            text = "TASDYN-ALIFE";
            font = "RobotoCondensedBold";
            sizeEx = 0.07;
            colorText[] = { 0.96, 0.62, 0.04, 1 };
            x = "safezoneX";
            y = "safezoneY + safezoneH * 0.36";
            w = "safezoneW";
            h = "safezoneH * 0.08";
        };
        class Status: ALife_RscStructuredText
        {
            idc = 9102;
            size = 0.032;
            x = "safezoneX + safezoneW * 0.2";
            y = "safezoneY + safezoneH * 0.46";
            w = "safezoneW * 0.6";
            h = "safezoneH * 0.16";
        };
        class Hint: ALife_RscText
        {
            idc = 9103;
            style = 2;
            sizeEx = 0.026;
            colorText[] = { 0.6, 0.64, 0.7, 1 };
            x = "safezoneX";
            y = "safezoneY + safezoneH * 0.64";
            w = "safezoneW";
            h = "safezoneH * 0.04";
        };
    };
};
