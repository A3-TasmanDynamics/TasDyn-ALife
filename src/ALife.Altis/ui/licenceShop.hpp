// Licence office window (a licence vendor NPC: Windows key). Driven by
// ALife_fnc_licenceShop; control IDs match the #defines there.
//
//   LICENCE OFFICE                              Bank $12,500
//   Licence             Price       Status
//   Driver Licence      $500        Owned
//   Pilot Licence       $25,000
//   <what the selected licence unlocks>
//   [Buy]                                              [Close]

class ALife_LicenceShop
{
    idd = 6000;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_licenceShop";

    class controlsBackground
    {
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.059, 0.09, 0.165, 0.96 };
            x = "safezoneX + safezoneW * 0.33";
            y = "safezoneY + safezoneH * 0.26";
            w = "safezoneW * 0.34";
            h = "safezoneH * 0.48";
        };
        class Accent: ALife_RscBackground
        {
            colorBackground[] = { 0.984, 0.749, 0.141, 1 };
            x = "safezoneX + safezoneW * 0.33";
            y = "safezoneY + safezoneH * 0.26";
            w = "safezoneW * 0.34";
            h = "safezoneH * 0.004";
        };
    };

    class controls
    {
        class Title: ALife_RscText
        {
            idc = 6001;
            sizeEx = 0.045;
            font = "RobotoCondensedBold";
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.275";
            w = "safezoneW * 0.2";
            h = "safezoneH * 0.04";
        };
        class Balance: ALife_RscStructuredText
        {
            idc = 6002;
            x = "safezoneX + safezoneW * 0.5";
            y = "safezoneY + safezoneH * 0.282";
            w = "safezoneW * 0.16";
            h = "safezoneH * 0.03";
        };
        class List: ALife_RscListNBox
        {
            idc = 6010;
            sizeEx = 0.032;
            rowHeight = 0.045;
            columns[] = { 0, 0.55, 0.78 };
            onLBSelChanged = "['select'] call ALife_fnc_licenceShop";
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.33";
            w = "safezoneW * 0.32";
            h = "safezoneH * 0.25";
        };
        class Details: ALife_RscStructuredText
        {
            idc = 6003;
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.59";
            w = "safezoneW * 0.32";
            h = "safezoneH * 0.08";
        };
        class Buy: ALife_RscButton
        {
            idc = 6020;
            text = "Buy";
            sizeEx = 0.032;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 0.98, 0.75, 0.14, 1 };
            colorFocused[] = { 0.96, 0.62, 0.04, 1 };
            colorText[] = { 0.118, 0.071, 0, 1 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "['buy'] call ALife_fnc_licenceShop";
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.68";
            w = "safezoneW * 0.08";
            h = "safezoneH * 0.04";
        };
        class Close: ALife_RscButton
        {
            idc = -1;
            text = "Close";
            sizeEx = 0.032;
            colorBackground[] = { 1, 1, 1, 0.08 };
            colorBackgroundActive[] = { 1, 1, 1, 0.15 };
            colorFocused[] = { 1, 1, 1, 0.1 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "closeDialog 0";
            x = "safezoneX + safezoneW * 0.6";
            y = "safezoneY + safezoneH * 0.68";
            w = "safezoneW * 0.06";
            h = "safezoneH * 0.04";
        };
    };
};
