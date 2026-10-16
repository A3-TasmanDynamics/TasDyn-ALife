// Death screen. Driven by ALife_fnc_deathScreen; control IDs match the
// #defines at the top of functions\client\fn_deathScreen.sqf.
//
// Layout: "YOU ARE DEAD" across the top, a panel in the lower middle with
// the killer, nearest medic, the countdown bar and the Respawn button --
// the world (your body) stays visible behind it.

class ALife_DeathScreen
{
    idd = 4900;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_deathScreen";

    class controlsBackground
    {
        class Shade: ALife_RscBackground
        {
            colorBackground[] = { 0.25, 0, 0, 0.18 };
            x = "safezoneX";
            y = "safezoneY";
            w = "safezoneW";
            h = "safezoneH";
        };
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.06, 0.07, 0.09, 0.9 };
            x = "safezoneX + safezoneW * 0.32";
            y = "safezoneY + safezoneH * 0.55";
            w = "safezoneW * 0.36";
            h = "safezoneH * 0.38";
        };
        class PanelAccent: ALife_RscBackground
        {
            colorBackground[] = { 0.85, 0.2, 0.2, 1 };
            x = "safezoneX + safezoneW * 0.32";
            y = "safezoneY + safezoneH * 0.55";
            w = "safezoneW * 0.36";
            h = "safezoneH * 0.004";
        };
    };

    class controls
    {
        class Title: ALife_RscText
        {
            style = 2;
            text = "YOU ARE DEAD";
            font = "RobotoCondensedBold";
            sizeEx = 0.09;
            colorText[] = { 0.95, 0.3, 0.3, 1 };
            shadow = 2;
            x = "safezoneX";
            y = "safezoneY + safezoneH * 0.12";
            w = "safezoneW";
            h = "safezoneH * 0.1";
        };

        class Killer: ALife_RscStructuredText
        {
            idc = 4910;
            size = 0.03;
            x = "safezoneX + safezoneW * 0.33";
            y = "safezoneY + safezoneH * 0.57";
            w = "safezoneW * 0.34";
            h = "safezoneH * 0.12";
        };

        class Medic: ALife_RscStructuredText
        {
            idc = 4920;
            size = 0.03;
            colorBackground[] = { 0.11, 0.12, 0.15, 1 };
            x = "safezoneX + safezoneW * 0.33";
            y = "safezoneY + safezoneH * 0.70";
            w = "safezoneW * 0.34";
            h = "safezoneH * 0.04";
        };

        class Timer: ALife_RscStructuredText
        {
            idc = 4930;
            size = 0.03;
            x = "safezoneX + safezoneW * 0.33";
            y = "safezoneY + safezoneH * 0.755";
            w = "safezoneW * 0.34";
            h = "safezoneH * 0.04";
        };

        class Bar: ALife_RscProgress
        {
            idc = 4931;
            colorBar[] = { 0.85, 0.2, 0.2, 1 };
            colorBackground[] = { 0.15, 0.16, 0.19, 1 };
            colorFrame[] = { 0, 0, 0, 0 };
            x = "safezoneX + safezoneW * 0.33";
            y = "safezoneY + safezoneH * 0.80";
            w = "safezoneW * 0.34";
            h = "safezoneH * 0.018";
        };

        class Respawn: ALife_RscButton
        {
            idc = 4940;
            text = "RESPAWN";
            font = "RobotoCondensedBold";
            sizeEx = 0.04;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 1, 0.72, 0.2, 1 };
            colorFocused[] = { 1, 0.72, 0.2, 1 };
            colorBackgroundDisabled[] = { 0.2, 0.21, 0.24, 1 };
            colorDisabled[] = { 0.5, 0.52, 0.56, 1 };
            colorText[] = { 0.05, 0.05, 0.05, 1 };
            action = "['respawn'] call ALife_fnc_deathScreen";
            x = "safezoneX + safezoneW * 0.33";
            y = "safezoneY + safezoneH * 0.835";
            w = "safezoneW * 0.34";
            h = "safezoneH * 0.055";
        };

        class Leave: ALife_RscButton
        {
            text = "Leave server";
            colorBackground[] = { 0.06, 0.07, 0.09, 0 };
            colorText[] = { 0.6, 0.64, 0.7, 1 };
            action = "['leave'] call ALife_fnc_deathScreen";
            x = "safezoneX + safezoneW * 0.33";
            y = "safezoneY + safezoneH * 0.895";
            w = "safezoneW * 0.34";
            h = "safezoneH * 0.03";
        };
    };
};
