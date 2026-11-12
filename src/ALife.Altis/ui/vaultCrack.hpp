// The evidence vault keypad (ALife_fnc_evidenceClient "crack"): type the
// code on the keypad, Enter to try it; each try is listed with how many
// digits are right and in place, and how many are right but elsewhere.
//
//   EVIDENCE VAULT KEYPAD
//        _ _ _ _                  1 2 3 4   2 right place   1 wrong place
//   4 digits · 10 tries left      ...
//   [1][2][3]
//   [4][5][6]
//   [7][8][9]
//   [C][0][Enter]                 [Close]

#define VC_X "safezoneX + safezoneW * 0.33"
#define VC_W "safezoneW * 0.34"

class ALife_VaultCrack
{
    idd = 6100;
    movingEnable = 0;
    enableSimulation = 1;

    class controlsBackground
    {
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.059, 0.09, 0.165, 0.96 };
            x = VC_X;
            y = "safezoneY + safezoneH * 0.25";
            w = VC_W;
            h = "safezoneH * 0.5";
        };
        class Accent: Panel
        {
            colorBackground[] = { 0.97, 0.44, 0.41, 1 };
            h = "safezoneH * 0.004";
        };
        class Screen: Panel
        {
            colorBackground[] = { 0.02, 0.05, 0.03, 1 };
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.31";
            w = "safezoneW * 0.14";
            h = "safezoneH * 0.07";
        };
    };

    class controls
    {
        class Title: ALife_RscText
        {
            idc = 6101;
            text = "EVIDENCE VAULT KEYPAD";
            font = "RobotoCondensedBold";
            sizeEx = 0.04;
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.262";
            w = "safezoneW * 0.32";
            h = "safezoneH * 0.035";
        };
        class Entry: ALife_RscStructuredText
        {
            idc = 6103;
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.318";
            w = "safezoneW * 0.14";
            h = "safezoneH * 0.06";
        };
        class Status: ALife_RscStructuredText
        {
            idc = 6102;
            size = 0.03;
            x = "safezoneX + safezoneW * 0.34";
            y = "safezoneY + safezoneH * 0.385";
            w = "safezoneW * 0.14";
            h = "safezoneH * 0.03";
        };
        class Key1: ALife_RscButton
        {
            idc = 6111;
            text = "1";
            font = "RobotoCondensedBold";
            sizeEx = 0.045;
            colorBackground[] = { 1, 1, 1, 0.1 };
            colorBackgroundActive[] = { 1, 1, 1, 0.22 };
            colorFocused[] = { 1, 1, 1, 0.1 };
            action = "['key', 1] call ALife_fnc_evidenceClient";
            x = "safezoneX + safezoneW * 0.348";
            y = "safezoneY + safezoneH * 0.425";
            w = "safezoneW * 0.04";
            h = "safezoneH * 0.06";
        };
        class Key2: Key1 { idc = 6112; text = "2"; action = "['key', 2] call ALife_fnc_evidenceClient"; x = "safezoneX + safezoneW * 0.39"; };
        class Key3: Key1 { idc = 6113; text = "3"; action = "['key', 3] call ALife_fnc_evidenceClient"; x = "safezoneX + safezoneW * 0.432"; };
        class Key4: Key1 { idc = 6114; text = "4"; action = "['key', 4] call ALife_fnc_evidenceClient"; y = "safezoneY + safezoneH * 0.49"; };
        class Key5: Key4 { idc = 6115; text = "5"; action = "['key', 5] call ALife_fnc_evidenceClient"; x = "safezoneX + safezoneW * 0.39"; };
        class Key6: Key4 { idc = 6116; text = "6"; action = "['key', 6] call ALife_fnc_evidenceClient"; x = "safezoneX + safezoneW * 0.432"; };
        class Key7: Key1 { idc = 6117; text = "7"; action = "['key', 7] call ALife_fnc_evidenceClient"; y = "safezoneY + safezoneH * 0.555"; };
        class Key8: Key7 { idc = 6118; text = "8"; action = "['key', 8] call ALife_fnc_evidenceClient"; x = "safezoneX + safezoneW * 0.39"; };
        class Key9: Key7 { idc = 6119; text = "9"; action = "['key', 9] call ALife_fnc_evidenceClient"; x = "safezoneX + safezoneW * 0.432"; };
        class KeyClear: Key1
        {
            idc = 6120;
            text = "C";
            action = "['clear'] call ALife_fnc_evidenceClient";
            y = "safezoneY + safezoneH * 0.62";
        };
        class Key0: KeyClear { idc = 6110; text = "0"; action = "['key', 0] call ALife_fnc_evidenceClient"; x = "safezoneX + safezoneW * 0.39"; };
        class KeyEnter: KeyClear
        {
            idc = 6121;
            text = "OK";
            colorBackground[] = { 0.29, 0.81, 0.59, 0.9 };
            colorBackgroundActive[] = { 0.35, 0.9, 0.65, 1 };
            colorFocused[] = { 0.29, 0.81, 0.59, 0.9 };
            colorText[] = { 0.02, 0.1, 0.05, 1 };
            action = "['enter'] call ALife_fnc_evidenceClient";
            x = "safezoneX + safezoneW * 0.432";
        };
        class History: ALife_RscListNBox
        {
            idc = 6130;
            sizeEx = 0.03;
            rowHeight = 0.04;
            columns[] = { 0, 0.32, 0.66 };
            x = "safezoneX + safezoneW * 0.49";
            y = "safezoneY + safezoneH * 0.31";
            w = "safezoneW * 0.17";
            h = "safezoneH * 0.37";
        };
        class Close: ALife_RscButton
        {
            text = "Close";
            sizeEx = 0.032;
            colorBackground[] = { 0, 0, 0, 0 };
            colorBackgroundActive[] = { 1, 1, 1, 0.1 };
            colorFocused[] = { 0, 0, 0, 0 };
            colorText[] = { 0.75, 0.78, 0.82, 1 };
            action = "closeDialog 0";
            x = "safezoneX + safezoneW * 0.6";
            y = "safezoneY + safezoneH * 0.695";
            w = "safezoneW * 0.06";
            h = "safezoneH * 0.04";
        };
    };
};
