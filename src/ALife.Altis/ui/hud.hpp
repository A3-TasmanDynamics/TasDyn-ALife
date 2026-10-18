// Player HUD: health, food and water, bottom-right. Two styles, picked in
// the phone's Settings (ALife_fnc_phoneSettings "hudStyle"); ALife_fnc_survival
// shows one and hides the other. A title layer, inside RscTitles in
// description.ext. Icons: ui\hud (tools/phone_icons/make_hud.py).
//
//   minimal: icon 93x1 + number 93x3, one row each
//   rings:   track 93x4, filled arc 93x5 (ring_00 .. ring_20), icon 93x6
//   (x = 1 health, 2 food, 3 water)

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
        // Minimal: Health
        class MiniIcon1: ALife_RscPicture
        {
            idc = 9311;
            text = "ui\hud\health_ca.paa";
            x = "safezoneX + safezoneW * 0.953";
            y = "safezoneY + safezoneH * 0.856";
            w = "safezoneW * 0.012";
            h = "safezoneH * 0.022";
        };
        class MiniValue1: ALife_RscText
        {
            idc = 9313;
            shadow = 2;
            sizeEx = 0.034;
            font = "RobotoCondensedBold";
            x = "safezoneX + safezoneW * 0.966";
            y = "safezoneY + safezoneH * 0.853";
            w = "safezoneW * 0.03";
            h = "safezoneH * 0.028";
        };
        // Minimal: Food
        class MiniIcon2: ALife_RscPicture
        {
            idc = 9321;
            text = "ui\hud\food_ca.paa";
            x = "safezoneX + safezoneW * 0.953";
            y = "safezoneY + safezoneH * 0.886";
            w = "safezoneW * 0.012";
            h = "safezoneH * 0.022";
        };
        class MiniValue2: ALife_RscText
        {
            idc = 9323;
            shadow = 2;
            sizeEx = 0.034;
            font = "RobotoCondensedBold";
            x = "safezoneX + safezoneW * 0.966";
            y = "safezoneY + safezoneH * 0.883";
            w = "safezoneW * 0.03";
            h = "safezoneH * 0.028";
        };
        // Minimal: Water
        class MiniIcon3: ALife_RscPicture
        {
            idc = 9331;
            text = "ui\hud\water_ca.paa";
            x = "safezoneX + safezoneW * 0.953";
            y = "safezoneY + safezoneH * 0.916";
            w = "safezoneW * 0.012";
            h = "safezoneH * 0.022";
        };
        class MiniValue3: ALife_RscText
        {
            idc = 9333;
            shadow = 2;
            sizeEx = 0.034;
            font = "RobotoCondensedBold";
            x = "safezoneX + safezoneW * 0.966";
            y = "safezoneY + safezoneH * 0.913";
            w = "safezoneW * 0.03";
            h = "safezoneH * 0.028";
        };
        // Rings: Health
        class RingTrack1: ALife_RscPicture
        {
            idc = 9314;
            text = "ui\hud\ring_bg_ca.paa";
            colorText[] = { 1, 1, 1, 0.22 };
            x = "safezoneX + safezoneW * 0.896";
            y = "safezoneY + safezoneH * 0.892";
            w = "safezoneW * 0.028";
            h = "safezoneH * 0.05";
        };
        class RingArc1: RingTrack1
        {
            idc = 9315;
            text = "ui\hud\ring_20_ca.paa";
            colorText[] = { 1, 1, 1, 1 };
        };
        class RingIcon1: ALife_RscPicture
        {
            idc = 9316;
            text = "ui\hud\health_ca.paa";
            x = "safezoneX + safezoneW * 0.9045";
            y = "safezoneY + safezoneH * 0.907";
            w = "safezoneW * 0.011";
            h = "safezoneH * 0.02";
        };
        // Rings: Food
        class RingTrack2: ALife_RscPicture
        {
            idc = 9324;
            text = "ui\hud\ring_bg_ca.paa";
            colorText[] = { 1, 1, 1, 0.22 };
            x = "safezoneX + safezoneW * 0.927";
            y = "safezoneY + safezoneH * 0.892";
            w = "safezoneW * 0.028";
            h = "safezoneH * 0.05";
        };
        class RingArc2: RingTrack2
        {
            idc = 9325;
            text = "ui\hud\ring_20_ca.paa";
            colorText[] = { 1, 1, 1, 1 };
        };
        class RingIcon2: ALife_RscPicture
        {
            idc = 9326;
            text = "ui\hud\food_ca.paa";
            x = "safezoneX + safezoneW * 0.9355";
            y = "safezoneY + safezoneH * 0.907";
            w = "safezoneW * 0.011";
            h = "safezoneH * 0.02";
        };
        // Rings: Water
        class RingTrack3: ALife_RscPicture
        {
            idc = 9334;
            text = "ui\hud\ring_bg_ca.paa";
            colorText[] = { 1, 1, 1, 0.22 };
            x = "safezoneX + safezoneW * 0.958";
            y = "safezoneY + safezoneH * 0.892";
            w = "safezoneW * 0.028";
            h = "safezoneH * 0.05";
        };
        class RingArc3: RingTrack3
        {
            idc = 9335;
            text = "ui\hud\ring_20_ca.paa";
            colorText[] = { 1, 1, 1, 1 };
        };
        class RingIcon3: ALife_RscPicture
        {
            idc = 9336;
            text = "ui\hud\water_ca.paa";
            x = "safezoneX + safezoneW * 0.9665";
            y = "safezoneY + safezoneH * 0.907";
            w = "safezoneW * 0.011";
            h = "safezoneH * 0.02";
        };
    };
};
