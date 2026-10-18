// Radial menu (Windows key): a title in a navy disc in the middle of the
// screen, the options in a ring around it. The option buttons are created
// by ALife_fnc_radial (as many as there are, up to 8), from
// ALife_RadialButton below.

class ALife_RadialButton: ALife_RscButton
{
    style = 2; // centred
    sizeEx = 0.036;
    font = "RobotoCondensedBold";
    shadow = 0;
    colorText[] = { 1, 1, 1, 1 };
    colorBackground[] = { 0.043, 0.071, 0.125, 0.85 };       // navy
    colorBackgroundActive[] = { 0.984, 0.749, 0.141, 1 };    // amber on hover
    colorFocused[] = { 0.043, 0.071, 0.125, 0.85 };
    colorShadow[] = { 0, 0, 0, 0 };
    colorBorder[] = { 0, 0, 0, 0 };
};

class ALife_Radial
{
    idd = 5700;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_radial";
    onUnload = "uiNamespace setVariable ['ALife_radialDisplay', displayNull]";

    class controls
    {
        // Placed in pixels by ALife_fnc_radial, so they're round on any screen.
        class Disc: ALife_RscPicture
        {
            idc = 5702;
            text = "ui\hud\disc_ca.paa";
            colorText[] = { 0.043, 0.071, 0.125, 0.85 };
        };
        class Ring: Disc
        {
            idc = 5703;
            text = "ui\hud\ring_bg_ca.paa";
            colorText[] = { 0.984, 0.749, 0.141, 0.9 };
        };
        class Title: ALife_RscStructuredText
        {
            idc = 5701;
        };
    };
};
