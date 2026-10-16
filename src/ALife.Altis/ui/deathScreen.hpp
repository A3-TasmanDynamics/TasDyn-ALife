// Death screen. Driven by ALife_fnc_deathScreen; control IDs match the
// #defines at the top of functions\client\fn_deathScreen.sqf.
//
// A slim bar low in the middle of the screen, so the bird's-eye view of
// the body fills the rest:
//
//   YOU ARE DEAD                              Nearest medic 450 m
//   Killed by Name  |  Weapon  |  143 m
//   Respawn in 1:43 (86%)                         [ RESPAWN ]
//   ===================-----------
//                                                  Leave server

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
            colorBackground[] = { 0.06, 0.07, 0.09, 0.88 };
            x = "safezoneX + safezoneW * 0.30";
            y = "safezoneY + safezoneH * 0.78";
            w = "safezoneW * 0.40";
            h = "safezoneH * 0.125";
        };
        class Accent: ALife_RscBackground
        {
            colorBackground[] = { 0.85, 0.2, 0.2, 1 };
            x = "safezoneX + safezoneW * 0.30";
            y = "safezoneY + safezoneH * 0.78";
            w = "safezoneW * 0.0025";
            h = "safezoneH * 0.125";
        };
    };

    class controls
    {
        class Title: ALife_RscText
        {
            text = "YOU ARE DEAD";
            font = "RobotoCondensedBold";
            sizeEx = 0.034;
            colorText[] = { 0.95, 0.32, 0.32, 1 };
            x = "safezoneX + safezoneW * 0.31";
            y = "safezoneY + safezoneH * 0.788";
            w = "safezoneW * 0.18";
            h = "safezoneH * 0.032";
        };

        class Medic: ALife_RscStructuredText
        {
            idc = 4920;
            size = 0.026;
            x = "safezoneX + safezoneW * 0.49";
            y = "safezoneY + safezoneH * 0.792";
            w = "safezoneW * 0.20";
            h = "safezoneH * 0.028";
        };

        class Killer: ALife_RscStructuredText
        {
            idc = 4910;
            size = 0.027;
            x = "safezoneX + safezoneW * 0.31";
            y = "safezoneY + safezoneH * 0.822";
            w = "safezoneW * 0.38";
            h = "safezoneH * 0.028";
        };

        class Timer: ALife_RscStructuredText
        {
            idc = 4930;
            size = 0.025;
            x = "safezoneX + safezoneW * 0.31";
            y = "safezoneY + safezoneH * 0.856";
            w = "safezoneW * 0.27";
            h = "safezoneH * 0.026";
        };

        class Bar: ALife_RscProgress
        {
            idc = 4931;
            colorBar[] = { 0.85, 0.2, 0.2, 1 };
            colorBackground[] = { 0.17, 0.18, 0.21, 1 };
            colorFrame[] = { 0, 0, 0, 0 };
            x = "safezoneX + safezoneW * 0.31";
            y = "safezoneY + safezoneH * 0.886";
            w = "safezoneW * 0.27";
            h = "safezoneH * 0.007";
        };

        class Respawn: ALife_RscButton
        {
            idc = 4940;
            text = "RESPAWN";
            font = "RobotoCondensedBold";
            sizeEx = 0.03;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 1, 0.72, 0.2, 1 };
            colorFocused[] = { 1, 0.72, 0.2, 1 };
            colorBackgroundDisabled[] = { 0.17, 0.18, 0.21, 1 };
            colorDisabled[] = { 0.45, 0.47, 0.52, 1 };
            colorText[] = { 0.05, 0.05, 0.05, 1 };
            action = "['respawn'] call ALife_fnc_deathScreen";
            x = "safezoneX + safezoneW * 0.595";
            y = "safezoneY + safezoneH * 0.854";
            w = "safezoneW * 0.095";
            h = "safezoneH * 0.04";
        };

        class Leave: ALife_RscButton
        {
            text = "Leave server";
            style = 1; // right-aligned
            sizeEx = 0.022;
            colorBackground[] = { 0, 0, 0, 0 };
            colorBackgroundActive[] = { 0, 0, 0, 0 };
            colorFocused[] = { 0, 0, 0, 0 };
            colorText[] = { 0.6, 0.64, 0.7, 0.8 };
            action = "['leave'] call ALife_fnc_deathScreen";
            x = "safezoneX + safezoneW * 0.60";
            y = "safezoneY + safezoneH * 0.908";
            w = "safezoneW * 0.10";
            h = "safezoneH * 0.022";
        };
    };
};
