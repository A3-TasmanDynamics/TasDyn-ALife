// Spawn point picker. Driven by ALife_fnc_spawnMenu; control IDs match the
// #defines at the top of functions\client\fn_spawnMenu.sqf.
//
// Layout: a panel covering most of the screen -- header across the top,
// the list of spawn points on the left with details and the Spawn button
// under it, and the map on the right.

class ALife_SpawnMenu
{
    idd = 4800;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_spawnMenu";
    onUnload = "['onUnload'] call ALife_fnc_spawnMenu";

    class controlsBackground
    {
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.07, 0.08, 0.1, 0.96 };
            x = "safezoneX + safezoneW * 0.08";
            y = "safezoneY + safezoneH * 0.08";
            w = "safezoneW * 0.84";
            h = "safezoneH * 0.84";
        };
        class HeaderBar: ALife_RscBackground
        {
            colorBackground[] = { 0.075, 0.106, 0.18, 1 };
            x = "safezoneX + safezoneW * 0.08";
            y = "safezoneY + safezoneH * 0.08";
            w = "safezoneW * 0.84";
            h = "safezoneH * 0.065";
        };
        class Accent: ALife_RscBackground
        {
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            x = "safezoneX + safezoneW * 0.08";
            y = "safezoneY + safezoneH * 0.145";
            w = "safezoneW * 0.84";
            h = "safezoneH * 0.003";
        };
    };

    class controls
    {
        class Header: ALife_RscStructuredText
        {
            idc = 4801;
            size = 0.03;
            x = "safezoneX + safezoneW * 0.09";
            y = "safezoneY + safezoneH * 0.092";
            w = "safezoneW * 0.82";
            h = "safezoneH * 0.045";
        };

        class List: ALife_RscListNBox
        {
            idc = 4810;
            sizeEx = 0.032;
            rowHeight = 0.055;
            onLBSelChanged = "['select', _this select 0, _this select 1] call ALife_fnc_spawnMenu";
            onLBDblClick = "['confirm'] call ALife_fnc_spawnMenu";
            x = "safezoneX + safezoneW * 0.09";
            y = "safezoneY + safezoneH * 0.165";
            w = "safezoneW * 0.25";
            h = "safezoneH * 0.46";
        };

        class Info: ALife_RscStructuredText
        {
            idc = 4830;
            size = 0.03;
            colorBackground[] = { 0.075, 0.106, 0.18, 1 };
            x = "safezoneX + safezoneW * 0.09";
            y = "safezoneY + safezoneH * 0.635";
            w = "safezoneW * 0.25";
            h = "safezoneH * 0.15";
        };

        class Spawn: ALife_RscButton
        {
            idc = 4840;
            text = "SPAWN";
            font = "RobotoCondensedBold";
            sizeEx = 0.04;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 1, 0.72, 0.2, 1 };
            colorFocused[] = { 1, 0.72, 0.2, 1 };
            colorText[] = { 0.118, 0.071, 0, 1 };
            action = "['confirm'] call ALife_fnc_spawnMenu";
            x = "safezoneX + safezoneW * 0.09";
            y = "safezoneY + safezoneH * 0.795";
            w = "safezoneW * 0.25";
            h = "safezoneH * 0.06";
        };

        class Leave: ALife_RscButton
        {
            text = "Leave server";
            colorBackground[] = { 0.075, 0.106, 0.18, 1 };
            colorText[] = { 0.6, 0.64, 0.7, 1 };
            action = "['leave'] call ALife_fnc_spawnMenu";
            x = "safezoneX + safezoneW * 0.09";
            y = "safezoneY + safezoneH * 0.865";
            w = "safezoneW * 0.25";
            h = "safezoneH * 0.04";
        };

        class Map: ALife_RscMap
        {
            idc = 4820;
            x = "safezoneX + safezoneW * 0.35";
            y = "safezoneY + safezoneH * 0.165";
            w = "safezoneW * 0.56";
            h = "safezoneH * 0.74";
        };
    };
};
