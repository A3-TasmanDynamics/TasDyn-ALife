/*
    ALife_fnc_toast (client)

    Our in-game notifications (instead of Arma's hint box): cards that stack
    in the top-right corner, with a coloured stripe for the kind of notice,
    then fade out. Up to 4 at once; the newest goes on top. They're drawn on
    whichever of our menus is open (or the game screen), so they're never
    hidden behind a menu.

    Arguments:
        0: STRING - text (may use <br/>)
        1: STRING - "info", "success", "warning", "error" or "message"
        2: STRING - title (optional; defaults by kind)
        3: NUMBER - seconds on screen (optional)

    ["layout"] re-stacks the open notices (used internally).
*/

params [["_text", "", [""]], ["_type", "info", [""]], ["_title", "", [""]], ["_duration", 5, [0]]];

if (!hasInterface) exitWith {};
if (isNil "ALife_toasts") then { ALife_toasts = []; };

private _x0 = safezoneX + safezoneW * 0.77;
private _w = safezoneW * 0.22;
private _top = safezoneY + safezoneH * 0.08;
private _gap = safezoneH * 0.008;

// Stack from the top: drop notices whose menu closed, then slide the rest into place.
if (_text == "layout") exitWith {
    ALife_toasts = ALife_toasts select { !isNull (_x select 2) };
    private _yy = _top;
    {
        _x params ["_bg", "_bar", "_body", "_h"];
        _bg ctrlSetPosition [_x0, _yy, _w, _h];
        _bar ctrlSetPosition [_x0, _yy, safezoneW * 0.0025, _h];
        _body ctrlSetPosition [_x0 + safezoneW * 0.008, _yy + safezoneH * 0.006, _w - safezoneW * 0.014, _h - safezoneH * 0.012];
        { _x ctrlCommit 0.15; } forEach [_bg, _bar, _body];
        _yy = _yy + _h + _gap;
    } forEach ALife_toasts;
};

if (_text == "") exitWith {};

// Draw on top of whichever of our menus is open, else on the game screen.
private _display = findDisplay 46;
{
    private _d = findDisplay _x;
    if (!isNull _d) exitWith { _display = _d; };
} forEach [5400, 5300, 5100, 5000, 4900, 4800];
if (isNull _display) exitWith {};

// [hex for the title text, RGBA for the stripe]
(switch (_type) do {
    case "success": { ["#4bce97", [0.29, 0.81, 0.59, 1]] };
    case "warning": { ["#f5a524", [0.96, 0.65, 0.14, 1]] };
    case "error": { ["#f87168", [0.97, 0.44, 0.41, 1]] };
    case "message": { ["#9f8fef", [0.62, 0.56, 0.94, 1]] };
    default { ["#579dff", [0.34, 0.62, 1, 1]] };
}) params ["_colour", "_rgba"];
if (_title == "") then {
    _title = switch (_type) do {
        case "success": { "Done" };
        case "warning": { "Heads up" };
        case "error": { "Something went wrong" };
        case "message": { "New message" };
        default { "Notice" };
    };
};

private _bg = _display ctrlCreate ["ALife_RscBackground", -1];
private _bar = _display ctrlCreate ["ALife_RscBackground", -1];
private _body = _display ctrlCreate ["ALife_RscStructuredText", -1];

_bg ctrlSetBackgroundColor [0.05, 0.055, 0.07, 0.94];
_bar ctrlSetBackgroundColor _rgba;

// Size the card to its text.
_body ctrlSetPosition [_x0 + safezoneW * 0.008, _top, _w - safezoneW * 0.014, safezoneH * 0.1];
_body ctrlCommit 0;
_body ctrlSetStructuredText parseText format [
    "<t size='0.95' font='RobotoCondensedBold' color='%1'>%2</t><br/><t size='1' color='#eef2f6'>%3</t>",
    _colour, (_title splitString "<>&") joinString "", _text];
private _textH = ctrlTextHeight _body;
private _h = _textH + safezoneH * 0.012;

_bg ctrlSetPosition [_x0, _top, _w, _h];
_bar ctrlSetPosition [_x0, _top, safezoneW * 0.0025, _h];
_body ctrlSetPosition [_x0 + safezoneW * 0.008, _top + safezoneH * 0.006, _w - safezoneW * 0.014, _textH];
{ _x ctrlSetFade 1; _x ctrlCommit 0; _x ctrlSetFade 0; _x ctrlCommit 0.2; } forEach [_bg, _bar, _body];

ALife_toasts = [[_bg, _bar, _body, _h]] + ALife_toasts;
while { count ALife_toasts > 4 } do {
    { ctrlDelete _x; } forEach ((ALife_toasts deleteAt (count ALife_toasts - 1)) select [0, 3]);
};
["layout"] call ALife_fnc_toast;

[_bg, _bar, _body, _duration] spawn {
    params ["_bg", "_bar", "_body", "_duration"];
    uiSleep _duration;
    { _x ctrlSetFade 1; _x ctrlCommit 0.4; } forEach [_bg, _bar, _body];
    uiSleep 0.4;
    { ctrlDelete _x; } forEach [_bg, _bar, _body];
    ["layout"] call ALife_fnc_toast;
};
