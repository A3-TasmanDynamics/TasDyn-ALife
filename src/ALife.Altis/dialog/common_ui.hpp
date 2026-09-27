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
// shape (control-type constants, an ALife_Rsc* naming scheme) — mostly
// adapted to only what this mission's dialogs actually need, except
// ALife_RscMap (see below).
//
// Tried switching to `class RscText;` (etc.) forward-declare + inherit
// instead, on the theory that it would pull in every real engine property
// automatically (no more hand-maintained lists). Confirmed live via RPT
// that this is wrong in this mission's compile context: every control lost
// its `type` entirely ("no type entry inside class .../SpawnMap" etc. for
// *every* control, not just the map) -- the forward declaration resolves
// to an empty stub, not the real engine class. Reverted. Stick to
// self-contained classes.
//
// ALife_RscMap specifically went through three rounds of live "No entry"
// errors patching one missing property/subclass at a time (`text`, then
// `widthRailWay`, then the `Tree` legend subclass) before it became clear
// that control's real property surface (including a few dozen nested
// icon-legend classes) is too large to hand-maintain piecemeal. It's now
// ported wholesale from Tonic's AsYetUntitled/Framework's
// Life_RscMapControl (a real, live, currently-deployed mission), with only
// our own color theme overridden on top -- see docs/TONIC_REFERENCE.md.

#define ALIFE_CT_STATIC    0
#define ALIFE_CT_BUTTON    1
#define ALIFE_CT_LISTBOX   5
#define ALIFE_CT_PROGRESS  8
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
    // Confirmed live: the real RscStructuredText sets `style = ST_LEFT` (0)
    // -- omitting it produced "No entry '.../InfoText.style'" the moment a
    // real client actually opened a dialog using this class, same class of
    // bug as the map control's missing `text`/`widthRailWay` above.
    style = 0; // ST_LEFT
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
    // Same gap as ALife_RscMap -- see the comment there. Fixed proactively
    // here rather than waiting to hit the same error for this control.
    text = "";
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

// Map control. The engine's dialog renderer reads a long list of
// RscMapControl properties -- including whole nested legend/icon
// subclasses (Tree, Bunker, Hospital, ...) -- unconditionally, regardless
// of whether the dialog ever actually shows those icons. Three rounds of
// live "No entry" errors (missing `text`, then `widthRailWay`, then the
// `Tree` subclass) proved that patching one property/class at a time
// against this control specifically doesn't converge. Ported wholesale
// from Tonic's AsYetUntitled/Framework's Life_RscMapControl
// (dialog/common.hpp, a real, live, currently-deployed mission) instead --
// a complete, proven-correct structure -- keeping only our own dark color
// theme as overrides on top of it. See docs/TONIC_REFERENCE.md §3/§4.
class ALife_RscMap
{
    access = 0;
    type = ALIFE_CT_MAP_MAIN;
    idc = -1;
    style = 48; // ST_PICTURE
    text = "";
    colorBackground[] = { 0.1, 0.1, 0.1, 1 };
    colorOutside[] = { 0, 0, 0, 1 };
    colorText[] = { 0, 0, 0, 1 };
    font = "TahomaB";
    sizeEx = 0.025;
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
    stickX[] = { 0.2, { "Gamma", 1, 1.5 } };
    stickY[] = { 0.2, { "Gamma", 1, 1.5 } };
    widthRailWay = 4;
    class Legend
    {
        colorBackground[] = { 1, 1, 1, 0.5 };
        color[] = { 0, 0, 0, 1 };
        x = "SafeZoneX + (((safezoneW / safezoneH) min 1.2) / 40)";
        y = "SafeZoneY + safezoneH - 4.5 * ((((safezoneW / safezoneH) min 1.2) / 1.2) / 25)";
        w = "10 * (((safezoneW / safezoneH) min 1.2) / 40)";
        h = "3.5 * ((((safezoneW / safezoneH) min 1.2) / 1.2) / 25)";
        font = "RobotoCondensed";
        sizeEx = "(((((safezoneW / safezoneH) min 1.2) / 1.2) / 25) * 0.8)";
    };
    class ActiveMarker
    {
        color[] = { 0.3, 0.1, 0.9, 1 };
        size = 50;
    };
    class Command
    {
        color[] = { 1, 1, 1, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\waypoint_ca.paa";
        size = 18;
        importance = 1;
        coefMin = 1;
        coefMax = 1;
    };
    class Task
    {
        colorCreated[] = { 1, 1, 1, 1 };
        colorCanceled[] = { 0.7, 0.7, 0.7, 1 };
        colorDone[] = { 0.7, 1, 0.3, 1 };
        colorFailed[] = { 1, 0.3, 0.2, 1 };
        color[] = { "(profilenamespace getvariable ['IGUI_TEXT_RGB_R',0])", "(profilenamespace getvariable ['IGUI_TEXT_RGB_G',1])", "(profilenamespace getvariable ['IGUI_TEXT_RGB_B',1])", "(profilenamespace getvariable ['IGUI_TEXT_RGB_A',0.8])" };
        icon = "\A3\ui_f\data\map\mapcontrol\taskIcon_CA.paa";
        iconCreated = "\A3\ui_f\data\map\mapcontrol\taskIconCreated_CA.paa";
        iconCanceled = "\A3\ui_f\data\map\mapcontrol\taskIconCanceled_CA.paa";
        iconDone = "\A3\ui_f\data\map\mapcontrol\taskIconDone_CA.paa";
        iconFailed = "\A3\ui_f\data\map\mapcontrol\taskIconFailed_CA.paa";
        size = 27;
        importance = 1;
        coefMin = 1;
        coefMax = 1;
    };
    class CustomMark
    {
        color[] = { 0, 0, 0, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\custommark_ca.paa";
        size = 24;
        importance = 1;
        coefMin = 1;
        coefMax = 1;
    };
    class Tree
    {
        color[] = { 0.45, 0.64, 0.33, 0.4 };
        icon = "\A3\ui_f\data\map\mapcontrol\bush_ca.paa";
        size = 12;
        importance = "0.9 * 16 * 0.05";
        coefMin = 0.25;
        coefMax = 4;
    };
    class SmallTree
    {
        color[] = { 0.45, 0.64, 0.33, 0.4 };
        icon = "\A3\ui_f\data\map\mapcontrol\bush_ca.paa";
        size = 12;
        importance = "0.6 * 12 * 0.05";
        coefMin = 0.25;
        coefMax = 4;
    };
    class Bush
    {
        color[] = { 0.45, 0.64, 0.33, 0.4 };
        icon = "\A3\ui_f\data\map\mapcontrol\bush_ca.paa";
        size = "14/2";
        importance = "0.2 * 14 * 0.05 * 0.05";
        coefMin = 0.25;
        coefMax = 4;
    };
    class Church
    {
        color[] = { 1, 1, 1, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\church_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
    };
    class Chapel
    {
        color[] = { 0, 0, 0, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\Chapel_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
    };
    class Cross
    {
        color[] = { 0, 0, 0, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\Cross_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
    };
    class Rock
    {
        color[] = { 0.1, 0.1, 0.1, 0.8 };
        icon = "\A3\ui_f\data\map\mapcontrol\rock_ca.paa";
        size = 12;
        importance = "0.5 * 12 * 0.05";
        coefMin = 0.25;
        coefMax = 4;
    };
    class Bunker
    {
        color[] = { 0, 0, 0, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\bunker_ca.paa";
        size = 14;
        importance = "1.5 * 14 * 0.05";
        coefMin = 0.25;
        coefMax = 4;
    };
    class Fortress
    {
        color[] = { 0, 0, 0, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\bunker_ca.paa";
        size = 16;
        importance = "2 * 16 * 0.05";
        coefMin = 0.25;
        coefMax = 4;
    };
    class Fountain
    {
        color[] = { 0, 0, 0, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\fountain_ca.paa";
        size = 11;
        importance = "1 * 12 * 0.05";
        coefMin = 0.25;
        coefMax = 4;
    };
    class ViewTower
    {
        color[] = { 0, 0, 0, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\viewtower_ca.paa";
        size = 16;
        importance = "2.5 * 16 * 0.05";
        coefMin = 0.5;
        coefMax = 4;
    };
    class Lighthouse
    {
        color[] = { 1, 1, 1, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\lighthouse_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
    };
    class Quay
    {
        color[] = { 1, 1, 1, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\quay_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
    };
    class Fuelstation
    {
        color[] = { 1, 1, 1, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\fuelstation_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
    };
    class Hospital
    {
        color[] = { 1, 1, 1, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\hospital_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
    };
    class BusStop
    {
        color[] = { 1, 1, 1, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\busstop_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
    };
    class Transmitter
    {
        color[] = { 1, 1, 1, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\transmitter_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
    };
    class Stack
    {
        color[] = { 0, 0, 0, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\stack_ca.paa";
        size = 20;
        importance = "2 * 16 * 0.05";
        coefMin = 0.9;
        coefMax = 4;
    };
    class Ruin
    {
        color[] = { 0, 0, 0, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\ruin_ca.paa";
        size = 16;
        importance = "1.2 * 16 * 0.05";
        coefMin = 1;
        coefMax = 4;
    };
    class Tourism
    {
        color[] = { 0, 0, 0, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\tourism_ca.paa";
        size = 16;
        importance = "1 * 16 * 0.05";
        coefMin = 0.7;
        coefMax = 4;
    };
    class Watertower
    {
        color[] = { 1, 1, 1, 1 };
        icon = "\A3\ui_f\data\map\mapcontrol\watertower_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
    };
    class Waypoint
    {
        color[] = { 0, 0, 0, 1 };
        size = 24;
        importance = 1;
        coefMin = 1;
        coefMax = 1;
        icon = "\A3\ui_f\data\map\mapcontrol\waypoint_ca.paa";
    };
    class WaypointCompleted
    {
        color[] = { 0, 0, 0, 1 };
        size = 24;
        importance = 1;
        coefMin = 1;
        coefMax = 1;
        icon = "\A3\ui_f\data\map\mapcontrol\waypointCompleted_ca.paa";
    };
    class power
    {
        icon = "\A3\ui_f\data\map\mapcontrol\power_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
        color[] = { 1, 1, 1, 1 };
    };
    class powersolar
    {
        icon = "\A3\ui_f\data\map\mapcontrol\powersolar_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
        color[] = { 1, 1, 1, 1 };
    };
    class powerwave
    {
        icon = "\A3\ui_f\data\map\mapcontrol\powerwave_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
        color[] = { 1, 1, 1, 1 };
    };
    class powerwind
    {
        icon = "\A3\ui_f\data\map\mapcontrol\powerwind_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
        color[] = { 1, 1, 1, 1 };
    };
    class shipwreck
    {
        icon = "\A3\ui_f\data\map\mapcontrol\shipwreck_CA.paa";
        size = 24;
        importance = 1;
        coefMin = 0.85;
        coefMax = 1;
        color[] = { 1, 1, 1, 1 };
    };
    class LineMarker
    {
        lineDistanceMin = 3e-005;
        lineLengthMin = 5;
        lineWidthThick = 0.014;
        lineWidthThin = 0.008;
        textureComboBoxColor = "#(argb,8,8,3)color(1,1,1,1)";
    };
    moveOnEdges = 1;
    fontLabel = "TahomaB"; fontGrid = "TahomaB"; fontUnits = "TahomaB";
    fontNames = "TahomaB"; fontInfo = "TahomaB"; fontLevel = "TahomaB";
    sizeExLabel = 0.025; sizeExGrid = 0.02; sizeExUnits = 0.025;
    sizeExNames = 0.025; sizeExInfo = 0.025; sizeExLevel = 0.02;
    ptsPerSquareSea = 5; ptsPerSquareTxt = 20; ptsPerSquareCLn = 10;
    ptsPerSquareExp = 10; ptsPerSquareCost = 10; ptsPerSquareFor = 9;
    ptsPerSquareForEdge = 15; ptsPerSquareRoad = 6; ptsPerSquareObj = 10;
    showCountourInterval = 0;
    scaleMin = 0.001; scaleMax = 1; scaleDefault = 0.16;
    maxSatelliteAlpha = 0.85; alphaFadeStartScale = 0.35; alphaFadeEndScale = 0.4;
    shadow = 0;
    x = 0; y = 0; w = 0.4; h = 0.4;
};

// Progress bar. Property set ported from Tonic's AsYetUntitled/Framework's
// Life_RscProgress (a real, live, currently-deployed mission) rather than
// hand-guessed, matching the same reasoning as ALife_RscMap above.
class ALife_RscProgress
{
    type = ALIFE_CT_PROGRESS;
    style = 0;
    idc = -1;
    texture = "";
    shadow = 2;
    colorFrame[] = { 0, 0, 0, 1 };
    colorBackground[] = { 0.11, 0.13, 0.16, 1 };
    colorBar[] = { 0.114, 0.914, 0.714, 1 };
    x = 0; y = 0; w = 0.3; h = 0.02;
};
