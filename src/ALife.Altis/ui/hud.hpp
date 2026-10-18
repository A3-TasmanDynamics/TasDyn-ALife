// Player HUD: health, food and water, bottom-right. Two styles, picked in
// the phone's Settings (ALife_fnc_phoneSettings "hudStyle"); ALife_fnc_survival
// shows one and hides the other. A title layer, inside RscTitles in
// description.ext. Icons: ui\hud (tools/phone_icons/make_hud.py).
//
//   minimal: backing disc 93x2, icon 93x1, number 93x3, one row each
//   rings:   backing disc 93x7, track 93x4, filled arc 93x5 (ring_00 ..
//            ring_20), icon 93x6 -- along the bottom edge
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
        class MiniDisc1: ALife_RscPicture
        {
            idc = 9312;
            text = "ui\hud\disc_ca.paa";
            colorText[] = { 0.043, 0.071, 0.125, 0.7 };
            x = "safezoneX + safezoneW * 0.936";
            y = "safezoneY + safezoneH * 0.835";
            w = "safezoneW * 0.023";
            h = "safezoneH * 0.041";
        };
        class MiniIcon1: ALife_RscPicture
        {
            idc = 9311;
            text = "ui\hud\health_ca.paa";
            x = "safezoneX + safezoneW * 0.9405";
            y = "safezoneY + safezoneH * 0.843";
            w = "safezoneW * 0.014";
            h = "safezoneH * 0.025";
        };
        class MiniValue1: ALife_RscText
        {
            idc = 9313;
            shadow = 2;
            sizeEx = 0.045;
            font = "RobotoCondensedBold";
            x = "safezoneX + safezoneW * 0.961";
            y = "safezoneY + safezoneH * 0.837";
            w = "safezoneW * 0.036";
            h = "safezoneH * 0.037";
        };
        // Minimal: Food
        class MiniDisc2: ALife_RscPicture
        {
            idc = 9322;
            text = "ui\hud\disc_ca.paa";
            colorText[] = { 0.043, 0.071, 0.125, 0.7 };
            x = "safezoneX + safezoneW * 0.936";
            y = "safezoneY + safezoneH * 0.881";
            w = "safezoneW * 0.023";
            h = "safezoneH * 0.041";
        };
        class MiniIcon2: ALife_RscPicture
        {
            idc = 9321;
            text = "ui\hud\food_ca.paa";
            x = "safezoneX + safezoneW * 0.9405";
            y = "safezoneY + safezoneH * 0.889";
            w = "safezoneW * 0.014";
            h = "safezoneH * 0.025";
        };
        class MiniValue2: ALife_RscText
        {
            idc = 9323;
            shadow = 2;
            sizeEx = 0.045;
            font = "RobotoCondensedBold";
            x = "safezoneX + safezoneW * 0.961";
            y = "safezoneY + safezoneH * 0.883";
            w = "safezoneW * 0.036";
            h = "safezoneH * 0.037";
        };
        // Minimal: Water
        class MiniDisc3: ALife_RscPicture
        {
            idc = 9332;
            text = "ui\hud\disc_ca.paa";
            colorText[] = { 0.043, 0.071, 0.125, 0.7 };
            x = "safezoneX + safezoneW * 0.936";
            y = "safezoneY + safezoneH * 0.927";
            w = "safezoneW * 0.023";
            h = "safezoneH * 0.041";
        };
        class MiniIcon3: ALife_RscPicture
        {
            idc = 9331;
            text = "ui\hud\water_ca.paa";
            x = "safezoneX + safezoneW * 0.9405";
            y = "safezoneY + safezoneH * 0.935";
            w = "safezoneW * 0.014";
            h = "safezoneH * 0.025";
        };
        class MiniValue3: ALife_RscText
        {
            idc = 9333;
            shadow = 2;
            sizeEx = 0.045;
            font = "RobotoCondensedBold";
            x = "safezoneX + safezoneW * 0.961";
            y = "safezoneY + safezoneH * 0.929";
            w = "safezoneW * 0.036";
            h = "safezoneH * 0.037";
        };
        // Rings: Health
        class RingDisc1: ALife_RscPicture
        {
            idc = 9317;
            text = "ui\hud\disc_ca.paa";
            colorText[] = { 0.043, 0.071, 0.125, 0.7 };
            x = "safezoneX + safezoneW * 0.866";
            y = "safezoneY + safezoneH * 0.925";
            w = "safezoneW * 0.037";
            h = "safezoneH * 0.066";
        };
        class RingTrack1: RingDisc1
        {
            idc = 9314;
            text = "ui\hud\ring_bg_ca.paa";
            colorText[] = { 1, 1, 1, 0.3 };
        };
        class RingArc1: RingDisc1
        {
            idc = 9315;
            text = "ui\hud\ring_20_ca.paa";
            colorText[] = { 1, 1, 1, 1 };
        };
        class RingIcon1: ALife_RscPicture
        {
            idc = 9316;
            text = "ui\hud\health_ca.paa";
            x = "safezoneX + safezoneW * 0.8770";
            y = "safezoneY + safezoneH * 0.945";
            w = "safezoneW * 0.015";
            h = "safezoneH * 0.026";
        };
        // Rings: Food
        class RingDisc2: ALife_RscPicture
        {
            idc = 9327;
            text = "ui\hud\disc_ca.paa";
            colorText[] = { 0.043, 0.071, 0.125, 0.7 };
            x = "safezoneX + safezoneW * 0.908";
            y = "safezoneY + safezoneH * 0.925";
            w = "safezoneW * 0.037";
            h = "safezoneH * 0.066";
        };
        class RingTrack2: RingDisc2
        {
            idc = 9324;
            text = "ui\hud\ring_bg_ca.paa";
            colorText[] = { 1, 1, 1, 0.3 };
        };
        class RingArc2: RingDisc2
        {
            idc = 9325;
            text = "ui\hud\ring_20_ca.paa";
            colorText[] = { 1, 1, 1, 1 };
        };
        class RingIcon2: ALife_RscPicture
        {
            idc = 9326;
            text = "ui\hud\food_ca.paa";
            x = "safezoneX + safezoneW * 0.9190";
            y = "safezoneY + safezoneH * 0.945";
            w = "safezoneW * 0.015";
            h = "safezoneH * 0.026";
        };
        // Rings: Water
        class RingDisc3: ALife_RscPicture
        {
            idc = 9337;
            text = "ui\hud\disc_ca.paa";
            colorText[] = { 0.043, 0.071, 0.125, 0.7 };
            x = "safezoneX + safezoneW * 0.950";
            y = "safezoneY + safezoneH * 0.925";
            w = "safezoneW * 0.037";
            h = "safezoneH * 0.066";
        };
        class RingTrack3: RingDisc3
        {
            idc = 9334;
            text = "ui\hud\ring_bg_ca.paa";
            colorText[] = { 1, 1, 1, 0.3 };
        };
        class RingArc3: RingDisc3
        {
            idc = 9335;
            text = "ui\hud\ring_20_ca.paa";
            colorText[] = { 1, 1, 1, 1 };
        };
        class RingIcon3: ALife_RscPicture
        {
            idc = 9336;
            text = "ui\hud\water_ca.paa";
            x = "safezoneX + safezoneW * 0.9610";
            y = "safezoneY + safezoneH * 0.945";
            w = "safezoneW * 0.015";
            h = "safezoneH * 0.026";
        };
    };
};
