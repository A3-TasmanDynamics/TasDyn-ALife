// Progress bar for timed actions (reviving, using an item). A title layer,
// included inside RscTitles in description.ext; see functions\client\fn_revive.sqf
// and fn_useItem.sqf. Styled like the HUD (ui\hud.hpp): no box, a bold
// shadowed label over an amber bar on a dark navy track, low in the middle.

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
        class Label: ALife_RscText
        {
            idc = 9201;
            style = 2;
            shadow = 2;
            sizeEx = 0.04;
            font = "RobotoCondensedBold";
            x = "safezoneX + safezoneW * 0.35";
            y = "safezoneY + safezoneH * 0.8";
            w = "safezoneW * 0.3";
            h = "safezoneH * 0.035";
        };
        class Track: ALife_RscBackground
        {
            colorBackground[] = { 0.043, 0.071, 0.125, 0.7 }; // navy, as behind the HUD
            x = "safezoneX + safezoneW * 0.408";
            y = "safezoneY + safezoneH * 0.837";
            w = "safezoneW * 0.184";
            h = "safezoneH * 0.016";
        };
        class Bar: ALife_RscProgress
        {
            idc = 9202;
            colorBar[] = { 0.984, 0.749, 0.141, 1 };          // amber
            colorBackground[] = { 1, 1, 1, 0.12 };
            colorFrame[] = { 0, 0, 0, 0 };
            shadow = 0;
            x = "safezoneX + safezoneW * 0.41";
            y = "safezoneY + safezoneH * 0.8405";
            w = "safezoneW * 0.18";
            h = "safezoneH * 0.009";
        };
    };
};
