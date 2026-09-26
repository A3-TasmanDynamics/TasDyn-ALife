// TasDyn-ALife — shared dialog base classes.
//
// These are self-contained (explicit `type = N` control-type constants,
// full property sets) rather than `class Foo : RscText` inheriting from the
// engine's own UI config. That inheritance approach is what broke
// dialog/spawnMenu.hpp earlier ("Undefined base class 'RscText'" — the
// mission config compiler needs those forward-declared, and it's easy to
// forget on every new dialog). Defining our own base classes from scratch
// sidesteps that whole class of bug permanently, and gives every dialog in
// this mission a single, consistent look instead of the engine's default
// styling. Reviewed a real framework's equivalent file for the general
// shape (control-type constants, an ALife_Rsc* naming scheme) — not copied
// wholesale, adapted to only what this mission's dialogs actually need.

#define ALIFE_CT_STATIC    0
#define ALIFE_CT_BUTTON    1
#define ALIFE_CT_LISTBOX   5
#define ALIFE_CT_STRUCTURED_TEXT 13
#define ALIFE_CT_MAP_MAIN  101

class ALife_RscBackground
{
    type = ALIFE_CT_STATIC;
    idc = -1;
    style = 0;
    colorBackground[] = { 0, 0, 0, 0.85 };
    colorText[] = { 1, 1, 1, 1 };
    font = "RobotoCondensed";
    sizeEx = 0.02;
    text = "";
    x = 0; y = 0; w = 0; h = 0;
};

class ALife_RscText
{
    type = ALIFE_CT_STATIC;
    idc = -1;
    style = 0;
    colorBackground[] = { 0, 0, 0, 0 };
    colorText[] = { 1, 1, 1, 1 };
    font = "RobotoCondensed";
    sizeEx = 0.025;
    text = "";
    x = 0; y = 0; w = 0.1; h = 0.04;
};

class ALife_RscTitle: ALife_RscText
{
    style = 2; // ST_CENTER
    sizeEx = 0.035;
    colorBackground[] = { 0.1, 0.1, 0.1, 1 };
};

class ALife_RscStructuredText
{
    type = ALIFE_CT_STRUCTURED_TEXT;
    idc = -1;
    colorBackground[] = { 0, 0, 0, 0 };
    colorText[] = { 1, 1, 1, 1 };
    size = 0.025;
    text = "";
    x = 0; y = 0; w = 0.1; h = 0.04;
    class Attributes
    {
        font = "RobotoCondensed";
        color = "#FFFFFF";
        align = "left";
        shadow = 1;
    };
};

class ALife_RscButton
{
    type = ALIFE_CT_BUTTON;
    idc = -1;
    style = 2; // ST_CENTER
    colorBackground[] = { 0.15, 0.15, 0.15, 1 };
    colorBackgroundActive[] = { 0.25, 0.25, 0.25, 1 };
    colorBackgroundDisabled[] = { 0.1, 0.1, 0.1, 1 };
    colorBorder[] = { 0, 0, 0, 1 };
    colorDisabled[] = { 0.5, 0.5, 0.5, 1 };
    colorFocused[] = { 0.25, 0.25, 0.25, 1 };
    colorShadow[] = { 0, 0, 0, 1 };
    colorText[] = { 1, 1, 1, 1 };
    font = "RobotoCondensed";
    sizeEx = 0.025;
    borderSize = 0;
    offsetPressedX = 0.001; offsetPressedY = 0.001;
    offsetX = 0; offsetY = 0;
    soundClick[] = { "\A3\ui_f\data\sound\RscButton\soundClick", 0.09, 1 };
    soundEnter[] = { "\A3\ui_f\data\sound\RscButton\soundEnter", 0.09, 1 };
    soundEscape[] = { "\A3\ui_f\data\sound\RscButton\soundEscape", 0.09, 1 };
    soundPush[] = { "\A3\ui_f\data\sound\RscButton\soundPush", 0.09, 1 };
    text = "";
    action = "";
    x = 0; y = 0; w = 0.1; h = 0.04;
};

class ALife_RscListBox
{
    type = ALIFE_CT_LISTBOX;
    idc = -1;
    style = 0;
    font = "RobotoCondensed";
    sizeEx = 0.025;
    rowHeight = 0.04;
    colorBackground[] = { 0.1, 0.1, 0.1, 0.9 };
    colorSelect[] = { 1, 1, 1, 1 };
    colorSelect2[] = { 1, 1, 1, 1 };
    colorSelectBackground[] = { 0.2, 0.4, 0.6, 1 };
    colorSelectBackground2[] = { 0.2, 0.4, 0.6, 1 };
    colorText[] = { 1, 1, 1, 1 };
    colorDisabled[] = { 0.5, 0.5, 0.5, 1 };
    colorScrollbar[] = { 1, 1, 1, 1 };
    soundSelect[] = { "\A3\ui_f\data\sound\RscListbox\soundSelect", 0.09, 1 };
    period = 0;
    maxHistoryDelay = 1;
    autoScrollSpeed = -1; autoScrollDelay = 5; autoScrollRewind = 0;
    arrowEmpty = "#(argb,8,8,3)color(1,1,1,1)";
    arrowFull = "#(argb,8,8,3)color(1,1,1,1)";
    shadow = 0;
    class ListScrollBar
    {
        color[] = { 1, 1, 1, 0.6 };
        colorActive[] = { 1, 1, 1, 1 };
        colorDisabled[] = { 1, 1, 1, 0.3 };
        thumb = "\A3\ui_f\data\gui\cfg\scrollbar\thumb_ca.paa";
        arrowEmpty = "\A3\ui_f\data\gui\cfg\scrollbar\arrowEmpty_ca.paa";
        arrowFull = "\A3\ui_f\data\gui\cfg\scrollbar\arrowFull_ca.paa";
        border = "\A3\ui_f\data\gui\cfg\scrollbar\border_ca.paa";
        autoScrollEnabled = 1; autoScrollDelay = 5; autoScrollRewind = 0; autoScrollSpeed = -1;
    };
    x = 0; y = 0; w = 0.2; h = 0.3;
};

class ALife_RscMap
{
    type = ALIFE_CT_MAP_MAIN;
    idc = -1;
    style = 48; // ST_PICTURE
    colorBackground[] = { 0.1, 0.1, 0.1, 1 };
    colorOutside[] = { 0, 0, 0, 1 };
    colorText[] = { 0, 0, 0, 1 };
    colorSea[] = { 0.467, 0.631, 0.851, 0.5 };
    colorForest[] = { 0.624, 0.78, 0.388, 0.5 };
    colorRocks[] = { 0, 0, 0, 0.3 };
    colorCountlines[] = { 0.572, 0.354, 0.188, 0.25 };
    colorMainCountlines[] = { 0.572, 0.354, 0.188, 0.5 };
    colorCountlinesWater[] = { 0.491, 0.577, 0.702, 0.3 };
    colorMainCountlinesWater[] = { 0.491, 0.577, 0.702, 0.6 };
    colorForestBorder[] = { 0, 0, 0, 0 };
    colorRocksBorder[] = { 0, 0, 0, 0 };
    colorPowerLines[] = { 0.1, 0.1, 0.1, 1 };
    colorRailWay[] = { 0.8, 0.2, 0, 1 };
    colorNames[] = { 0.1, 0.1, 0.1, 0.9 };
    colorInactive[] = { 1, 1, 1, 0.5 };
    colorLevels[] = { 0.286, 0.177, 0.094, 0.5 };
    colorTracks[] = { 0.84, 0.76, 0.65, 0.15 };
    colorRoads[] = { 0.7, 0.7, 0.7, 1 };
    colorMainRoads[] = { 0.9, 0.5, 0.3, 1 };
    colorTracksFill[] = { 0.84, 0.76, 0.65, 1 };
    colorRoadsFill[] = { 1, 1, 1, 1 };
    colorMainRoadsFill[] = { 1, 0.6, 0.4, 1 };
    colorGrid[] = { 0.1, 0.1, 0.1, 0.6 };
    colorGridMap[] = { 0.1, 0.1, 0.1, 0.6 };
    font = "TahomaB";
    fontLabel = "TahomaB"; fontGrid = "TahomaB"; fontUnits = "TahomaB";
    fontNames = "TahomaB"; fontInfo = "TahomaB"; fontLevel = "TahomaB";
    sizeEx = 0.025;
    sizeExLabel = 0.025; sizeExGrid = 0.02; sizeExUnits = 0.025;
    sizeExNames = 0.025; sizeExInfo = 0.025; sizeExLevel = 0.02;
    stickX[] = { 0.2, { "Gamma", 1, 1.5 } };
    stickY[] = { 0.2, { "Gamma", 1, 1.5 } };
    ptsPerSquareSea = 5; ptsPerSquareTxt = 20; ptsPerSquareCLn = 10;
    ptsPerSquareExp = 10; ptsPerSquareCost = 10; ptsPerSquareFor = 9;
    ptsPerSquareForEdge = 15; ptsPerSquareRoad = 6; ptsPerSquareObj = 10;
    showCountourInterval = 0;
    scaleMin = 0.001; scaleMax = 1; scaleDefault = 0.16;
    maxSatelliteAlpha = 0.85; alphaFadeStartScale = 0.35; alphaFadeEndScale = 0.4;
    moveOnEdges = 1;
    x = 0; y = 0; w = 0.4; h = 0.4;
};
