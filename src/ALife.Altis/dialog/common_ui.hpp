// TasDyn-ALife — shared dialog base classes.
//
// Forward-declares and lightly customizes the engine's own Rsc* UI base
// classes (RscText, RscButton, RscListBox, RscMapControl, ...) rather than
// hand-rolling self-contained replacements. Two real bugs already came
// from that: the original "Undefined base class 'RscText'" (fixed by
// forward-declaring, in an earlier version of this mission before this
// file existed), then an over-correction into fully self-contained
// classes here that turned out to be missing dozens of properties the
// real engine classes carry -- confirmed live, one missing property at a
// time ("No entry '.../SpawnMap.text'", then "'.../SpawnMap.widthRailWay'"
// once `text` was added) -- exactly the "Dialog Map.widthRailWay" issue
// reported on Bohemia's own forums since Arma 3 1.90, for the same reason:
// a custom map-control base missing properties the real RscMapControl
// always carries. Forward-declaring and inheriting from the real classes
// gives every property automatically and correctly; this file only needs
// to override what should actually look different from the engine
// defaults.

class RscText;
class RscButton;
class RscListBox;
class RscStructuredText;
class RscMapControl;

class ALife_RscBackground: RscText
{
    idc = -1;
    colorBackground[] = { 0, 0, 0, 0.85 };
    colorText[] = { 1, 1, 1, 1 };
    font = "RobotoCondensed";
    sizeEx = 0.02;
    text = "";
    x = 0; y = 0; w = 0; h = 0;
};

class ALife_RscText: RscText
{
    idc = -1;
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

class ALife_RscStructuredText: RscStructuredText
{
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

class ALife_RscButton: RscButton
{
    idc = -1;
    colorBackground[] = { 0.15, 0.15, 0.15, 1 };
    colorBackgroundActive[] = { 0.25, 0.25, 0.25, 1 };
    colorBackgroundDisabled[] = { 0.1, 0.1, 0.1, 1 };
    colorText[] = { 1, 1, 1, 1 };
    font = "RobotoCondensed";
    sizeEx = 0.025;
    text = "";
    action = "";
    x = 0; y = 0; w = 0.1; h = 0.04;
};

class ALife_RscListBox: RscListBox
{
    idc = -1;
    font = "RobotoCondensed";
    sizeEx = 0.025;
    rowHeight = 0.04;
    colorBackground[] = { 0.1, 0.1, 0.1, 0.9 };
    colorSelectBackground[] = { 0.2, 0.4, 0.6, 1 };
    colorSelectBackground2[] = { 0.2, 0.4, 0.6, 1 };
    colorText[] = { 1, 1, 1, 1 };
    x = 0; y = 0; w = 0.2; h = 0.3;
};

class ALife_RscMap: RscMapControl
{
    idc = -1;
    colorBackground[] = { 0.1, 0.1, 0.1, 1 };
    x = 0; y = 0; w = 0.4; h = 0.4;
};
