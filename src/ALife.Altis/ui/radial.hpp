// Interaction wheel (Windows key): separate slice tiles around a centre
// disc, with clear gaps between them, in the website's navy. The slice
// under the mouse lights up with an amber band on its outer edge, and the
// centre shows what it does; click to pick it. Images: ui\radial
// (tools/phone_icons/make_radial.py, 512 px) -- one highlight per slice
// position, never rotated (Arma stretches rotated pictures). Everything is
// placed in pixels by ALife_fnc_radial, so the wheel is round on any
// screen; the slice labels are created there, one per option.

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
            colorBackground[] = { 0, 0, 0, 0.25 };
            x = "safezoneX";
            y = "safezoneY";
            w = "safezoneW";
            h = "safezoneH";
        };
        class Tiles: ALife_RscPicture
        {
            idc = 5702;
            text = "ui\radial\tiles_2_ca.paa";
            colorText[] = { 0.075, 0.106, 0.18, 0.88 };     // navy
        };
        class Wedge: Tiles
        {
            idc = 5703;
            text = "ui\radial\wedge_2_0_ca.paa";
            colorText[] = { 0.16, 0.21, 0.32, 0.97 };       // the hovered slice
        };
        class Accent: Tiles
        {
            idc = 5705;
            text = "ui\radial\accent_2_0_ca.paa";
            colorText[] = { 0.984, 0.749, 0.141, 1 };       // amber
        };
        class Centre: Tiles
        {
            idc = 5706;
            text = "ui\radial\centre_ca.paa";
            colorText[] = { 0.043, 0.071, 0.125, 0.95 };
        };
        class Label: ALife_RscStructuredText
        {
            idc = 5701;
        };
    };
};
