// Death screen. Driven by ALife_fnc_deathScreen; control IDs match the
// #defines at the top of functions\client\fn_deathScreen.sqf.
//
// One compact rectangle low in the middle of the screen, so the bird's-eye
// view of the body fills the rest:
//
//   | YOU ARE INCAPACITATED                 Nearest medic 450 m
//   | ---------------------------------------------------------
//   | Killed by Name  |  Weapon  |  143 m
//   | Respawn in 1:43 (86%)                      [ RESPAWN ]
//   | =====================-----------
//
// No "leave" button: disconnecting while incapacitated is combat logging
// (recorded as an anti-cheat flag + staff log entry, posted to Discord).

class ALife_DeathScreen
{
    idd = 4900;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_deathScreen";

    class controlsBackground
    {
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.05, 0.055, 0.07, 0.94 };
            x = "safezoneX + safezoneW * 0.29";
            y = "safezoneY + safezoneH * 0.745";
            w = "safezoneW * 0.42";
            h = "safezoneH * 0.17";
        };
        class Accent: ALife_RscBackground
        {
            colorBackground[] = { 0.9, 0.25, 0.25, 1 };
            x = "safezoneX + safezoneW * 0.29";
            y = "safezoneY + safezoneH * 0.745";
            w = "safezoneW * 0.003";
            h = "safezoneH * 0.17";
        };
        class Divider: ALife_RscBackground
        {
            colorBackground[] = { 1, 1, 1, 0.1 };
            x = "safezoneX + safezoneW * 0.302";
            y = "safezoneY + safezoneH * 0.801";
            w = "safezoneW * 0.398";
            h = "safezoneH * 0.002";
        };
    };

    class controls
    {
        class Title: ALife_RscText
        {
            text = "YOU ARE INCAPACITATED";
            font = "RobotoCondensedBold";
            sizeEx = 0.044;
            colorText[] = { 1, 0.38, 0.38, 1 };
            shadow = 1;
            x = "safezoneX + safezoneW * 0.302";
            y = "safezoneY + safezoneH * 0.755";
            w = "safezoneW * 0.25";
            h = "safezoneH * 0.042";
        };

        class Medic: ALife_RscStructuredText
        {
            idc = 4920;
            size = 0.031;
            x = "safezoneX + safezoneW * 0.55";
            y = "safezoneY + safezoneH * 0.762";
            w = "safezoneW * 0.15";
            h = "safezoneH * 0.034";
        };

        class Killer: ALife_RscStructuredText
        {
            idc = 4910;
            size = 0.034;
            x = "safezoneX + safezoneW * 0.302";
            y = "safezoneY + safezoneH * 0.811";
            w = "safezoneW * 0.398";
            h = "safezoneH * 0.036";
        };

        class Timer: ALife_RscStructuredText
        {
            idc = 4930;
            size = 0.032;
            x = "safezoneX + safezoneW * 0.302";
            y = "safezoneY + safezoneH * 0.853";
            w = "safezoneW * 0.28";
            h = "safezoneH * 0.034";
        };

        class Bar: ALife_RscProgress
        {
            idc = 4931;
            colorBar[] = { 0.9, 0.25, 0.25, 1 };
            colorBackground[] = { 1, 1, 1, 0.12 };
            colorFrame[] = { 0, 0, 0, 0 };
            x = "safezoneX + safezoneW * 0.302";
            y = "safezoneY + safezoneH * 0.892";
            w = "safezoneW * 0.28";
            h = "safezoneH * 0.009";
        };

        class Respawn: ALife_RscButton
        {
            idc = 4940;
            text = "RESPAWN";
            font = "RobotoCondensedBold";
            sizeEx = 0.036;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 1, 0.72, 0.2, 1 };
            colorFocused[] = { 1, 0.72, 0.2, 1 };
            colorBackgroundDisabled[] = { 1, 1, 1, 0.08 };
            colorDisabled[] = { 1, 1, 1, 0.35 };
            colorText[] = { 0.05, 0.05, 0.05, 1 };
            action = "['respawn'] call ALife_fnc_deathScreen";
            x = "safezoneX + safezoneW * 0.592";
            y = "safezoneY + safezoneH * 0.852";
            w = "safezoneW * 0.108";
            h = "safezoneH * 0.05";
        };
    };
};
