// Garage window (at a vehicle shop: Windows -> Garage). Driven by
// ALife_fnc_garageMenu; control IDs match the #defines there.
//
//   GARAGE                                      Bank $12,500
//   Vehicle             Plate      Status
//   Offroad             ABC123     In the garage
//   SUV                 XYZ789     Impounded ($3,000)
//   <details of the selected vehicle>
//   [Take out]  [Store nearby vehicle]                [Close]

class ALife_Garage
{
    idd = 5900;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_garageMenu";

    class controlsBackground
    {
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.059, 0.09, 0.165, 0.96 };
            x = "safezoneX + safezoneW * 0.33";
            y = "safezoneY + safezoneH * 0.28";
            w = "safezoneW * 0.34";
            h = "safezoneH * 0.44";
        };
        class Accent: ALife_RscBackground
        {
            colorBackground[] = { 0.984, 0.749, 0.141, 1 };
            x = "safezoneX + safezoneW * 0.33";
            y = "safezoneY + safezoneH * 0.28";
            w = "safezoneW * 0.34";
            h = "safezoneH * 0.004";
        };
    };

    class controls
    {
        class Title: ALife_RscText
        {
            idc = 5901;
            sizeEx = 0.045;
            font = "RobotoCondensedBold";
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.295";
            w = "safezoneW * 0.2";
            h = "safezoneH * 0.04";
        };
        class Balance: ALife_RscStructuredText
        {
            idc = 5902;
            x = "safezoneX + safezoneW * 0.5";
            y = "safezoneY + safezoneH * 0.302";
            w = "safezoneW * 0.16";
            h = "safezoneH * 0.03";
        };
        class List: ALife_RscListNBox
        {
            idc = 5910;
            sizeEx = 0.032;
            rowHeight = 0.045;
            columns[] = { 0, 0.5, 0.7 };
            onLBSelChanged = "['select'] call ALife_fnc_garageMenu";
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.35";
            w = "safezoneW * 0.32";
            h = "safezoneH * 0.21";
        };
        class Details: ALife_RscStructuredText
        {
            idc = 5903;
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.57";
            w = "safezoneW * 0.32";
            h = "safezoneH * 0.075";
        };
        class TakeOut: ALife_RscButton
        {
            idc = 5920;
            text = "Take out";
            sizeEx = 0.032;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 0.98, 0.75, 0.14, 1 };
            colorFocused[] = { 0.96, 0.62, 0.04, 1 };
            colorText[] = { 0.118, 0.071, 0, 1 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "['retrieve'] call ALife_fnc_garageMenu";
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.655";
            w = "safezoneW * 0.08";
            h = "safezoneH * 0.04";
        };
        class Store: ALife_RscButton
        {
            idc = 5921;
            text = "Store nearby vehicle";
            sizeEx = 0.032;
            colorBackground[] = { 1, 1, 1, 0.08 };
            colorBackgroundActive[] = { 1, 1, 1, 0.15 };
            colorFocused[] = { 1, 1, 1, 0.1 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "['store'] call ALife_fnc_garageMenu";
            x = "safezoneX + safezoneW * 0.425";
            y = "safezoneY + safezoneH * 0.655";
            w = "safezoneW * 0.11";
            h = "safezoneH * 0.04";
        };
        class Close: Store
        {
            idc = -1;
            text = "Close";
            action = "closeDialog 0";
            x = "safezoneX + safezoneW * 0.6";
            w = "safezoneW * 0.06";
        };
    };
};
