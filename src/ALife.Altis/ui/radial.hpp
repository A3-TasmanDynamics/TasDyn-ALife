// Interaction wheel (Windows key): a dark ring cut into slices around a
// centre disc. The slice under the mouse lights up with an amber arc on its
// edge, and the centre shows what it does; click to pick it. Images:
// ui\radial (tools/phone_icons/make_radial.py). Everything is placed in
// pixels by ALife_fnc_radial, so the wheel is round on any screen; the slice
// labels are created there, one per option.

class ALife_Radial
{
    idd = 5700;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_radial";
    onUnload = "uiNamespace setVariable ['ALife_radialDisplay', displayNull]";

    class controls
    {
        class Dim: ALife_RscBackground
        {
            idc = 5709;
            colorBackground[] = { 0, 0, 0, 0.3 };
            x = "safezoneX";
            y = "safezoneY";
            w = "safezoneW";
            h = "safezoneH";
        };
        class Ring: ALife_RscPicture
        {
            idc = 5702;
            text = "ui\radial\ring_ca.paa";
            colorText[] = { 0.165, 0.169, 0.188, 0.97 };
        };
        class Wedge: Ring
        {
            idc = 5703;
            text = "ui\radial\wedge_2_ca.paa";
            colorText[] = { 0.243, 0.251, 0.275, 1 };
        };
        class Lines: Ring
        {
            idc = 5704;
            text = "ui\radial\lines_2_ca.paa";
            colorText[] = { 0.29, 0.298, 0.322, 1 };
        };
        class Accent: Ring
        {
            idc = 5705;
            text = "ui\radial\accent_2_ca.paa";
            colorText[] = { 0.984, 0.749, 0.141, 1 }; // amber
        };
        class Centre: Ring
        {
            idc = 5706;
            text = "ui\radial\centre_ca.paa";
            colorText[] = { 0.102, 0.106, 0.122, 0.98 };
        };
        class Label: ALife_RscStructuredText
        {
            idc = 5701;
        };
    };
};
