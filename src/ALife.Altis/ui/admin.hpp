// Admin menu (phone -> Admin -> Open admin menu). Driven by ALife_fnc_admin;
// control IDs match the #defines there. The action buttons are created by
// ALife_fnc_admin, only the ones the staff member has permission for.
//
//   ADMIN  .  Moderator
//   PLAYERS (12)        | SELECTED PLAYER              | YOU
//   Name        Faction | name / faction / UID / ...   | [God mode] [Invisible]
//   ...                 | Reason / message [........]  | [Markers]  [Map teleport]
//                       | Amount [....] [Account v]    | [Heal me]  [Stop spectating]
//                       | Ban length [1 day v]         | SERVER
//                       | Item [Water v]               | Vehicle class [........]
//                       | [Teleport to] [Bring here]   | [Announce] [Spawn vehicle]
//                       | [Spectate]    [Heal/revive]  | [Repair]   [Refuel] [Delete]
//                       | ...                          |                    [Close]

class ALife_Admin
{
    idd = 5800;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_admin";
    onUnload = "ALife_adminOpen = false";

    class controlsBackground
    {
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.059, 0.09, 0.165, 0.97 };
            x = "safezoneX + safezoneW * 0.16";
            y = "safezoneY + safezoneH * 0.15";
            w = "safezoneW * 0.68";
            h = "safezoneH * 0.70";
        };
        class Accent: ALife_RscBackground
        {
            colorBackground[] = { 0.984, 0.749, 0.141, 1 };
            x = "safezoneX + safezoneW * 0.16";
            y = "safezoneY + safezoneH * 0.15";
            w = "safezoneW * 0.68";
            h = "safezoneH * 0.004";
        };
    };

    class controls
    {
        class Title: ALife_RscStructuredText
        {
            idc = 5801;
            x = "safezoneX + safezoneW * 0.17";
            y = "safezoneY + safezoneH * 0.163";
            w = "safezoneW * 0.66";
            h = "safezoneH * 0.035";
        };
        class PlayersLabel: ALife_RscText
        {
            idc = 5802;
            sizeEx = 0.028;
            colorText[] = { 0.545, 0.596, 0.702, 1 };
            x = "safezoneX + safezoneW * 0.17";
            y = "safezoneY + safezoneH * 0.205";
            w = "safezoneW * 0.2";
            h = "safezoneH * 0.025";
        };
        class Players: ALife_RscListNBox
        {
            idc = 5810;
            sizeEx = 0.03;
            rowHeight = 0.04;
            columns[] = { 0, 0.68 };
            onLBSelChanged = "['select'] call ALife_fnc_admin";
            x = "safezoneX + safezoneW * 0.17";
            y = "safezoneY + safezoneH * 0.235";
            w = "safezoneW * 0.2";
            h = "safezoneH * 0.595";
        };

        class Details: ALife_RscStructuredText
        {
            idc = 5803;
            x = "safezoneX + safezoneW * 0.385";
            y = "safezoneY + safezoneH * 0.205";
            w = "safezoneW * 0.22";
            h = "safezoneH * 0.13";
        };
        class ReasonLabel: PlayersLabel
        {
            idc = -1;
            text = "Reason / message";
            x = "safezoneX + safezoneW * 0.385";
            y = "safezoneY + safezoneH * 0.34";
        };
        class Reason: ALife_RscEdit
        {
            idc = 5820;
            sizeEx = 0.03;
            x = "safezoneX + safezoneW * 0.385";
            y = "safezoneY + safezoneH * 0.365";
            w = "safezoneW * 0.22";
            h = "safezoneH * 0.034";
        };
        class AmountLabel: ReasonLabel
        {
            text = "Amount / quantity  -  account";
            y = "safezoneY + safezoneH * 0.405";
        };
        class Amount: Reason
        {
            idc = 5821;
            y = "safezoneY + safezoneH * 0.43";
            w = "safezoneW * 0.09";
        };
        class Account: ALife_RscCombo
        {
            idc = 5822;
            x = "safezoneX + safezoneW * 0.48";
            y = "safezoneY + safezoneH * 0.43";
            w = "safezoneW * 0.125";
            h = "safezoneH * 0.034";
        };
        class BanLabel: ReasonLabel
        {
            text = "Ban length  -  item";
            y = "safezoneY + safezoneH * 0.47";
        };
        class BanLength: Account
        {
            idc = 5823;
            x = "safezoneX + safezoneW * 0.385";
            y = "safezoneY + safezoneH * 0.495";
            w = "safezoneW * 0.09";
        };
        class Item: Account
        {
            idc = 5824;
            y = "safezoneY + safezoneH * 0.495";
        };

        class YouLabel: PlayersLabel
        {
            idc = -1;
            text = "YOU";
            x = "safezoneX + safezoneW * 0.62";
        };
        class ServerLabel: YouLabel
        {
            text = "SERVER";
            y = "safezoneY + safezoneH * 0.39";
        };
        class VehicleLabel: YouLabel
        {
            text = "Vehicle class (spawn)";
            y = "safezoneY + safezoneH * 0.415";
        };
        class VehicleClass: Reason
        {
            idc = 5825;
            text = "C_Offroad_01_F";
            x = "safezoneX + safezoneW * 0.62";
            y = "safezoneY + safezoneH * 0.44";
            w = "safezoneW * 0.21";
        };

        class Close: ALife_RscButton
        {
            text = "Close";
            sizeEx = 0.032;
            colorBackground[] = { 1, 1, 1, 0.06 };
            colorBackgroundActive[] = { 1, 1, 1, 0.15 };
            colorFocused[] = { 1, 1, 1, 0.1 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "closeDialog 0";
            x = "safezoneX + safezoneW * 0.76";
            y = "safezoneY + safezoneH * 0.8";
            w = "safezoneW * 0.07";
            h = "safezoneH * 0.035";
        };
    };
};
