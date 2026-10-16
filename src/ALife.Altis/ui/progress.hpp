// Small progress bar for timed actions (e.g. reviving). A title layer,
// included inside RscTitles in description.ext; see functions\client\fn_revive.sqf.

class ALife_Progress
{
    idd = -1;
    duration = 1e+011;
    fadeIn = 0;
    fadeOut = 0.2;
    movingEnable = 0;
    onLoad = "uiNamespace setVariable ['ALife_Progress', _this select 0]";
    onUnload = "uiNamespace setVariable ['ALife_Progress', displayNull]";

    class controls
    {
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.05, 0.055, 0.07, 0.85 };
            x = "safezoneX + safezoneW * 0.4";
            y = "safezoneY + safezoneH * 0.66";
            w = "safezoneW * 0.2";
            h = "safezoneH * 0.06";
        };
        class Label: ALife_RscText
        {
            idc = 9201;
            style = 2;
            sizeEx = 0.032;
            x = "safezoneX + safezoneW * 0.4";
            y = "safezoneY + safezoneH * 0.665";
            w = "safezoneW * 0.2";
            h = "safezoneH * 0.03";
        };
        class Bar: ALife_RscProgress
        {
            idc = 9202;
            colorBar[] = { 0.56, 0.94, 0.64, 1 };
            colorBackground[] = { 1, 1, 1, 0.12 };
            colorFrame[] = { 0, 0, 0, 0 };
            x = "safezoneX + safezoneW * 0.41";
            y = "safezoneY + safezoneH * 0.7";
            w = "safezoneW * 0.18";
            h = "safezoneH * 0.008";
        };
    };
};
