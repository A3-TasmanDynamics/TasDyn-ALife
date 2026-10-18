/*
    ALife_fnc_toast (client)

    Our in-game notifications (instead of Arma's hint box): cards that stack
    in the top-right corner, with a coloured stripe for the kind of notice.
    Up to 4 at once; the newest goes on top. They only go away when their
    time is up: opening, closing or switching menus doesn't touch them --
    each card is drawn on whatever is on top (one of our menus, Arma's
    inventory, the open map, or the game screen) and moved across when that
    changes.

    Arguments:
        0: STRING - text (may use <br/>)
        1: STRING - "info", "success", "warning", "error" or "message"
        2: STRING - title (optional; defaults by kind)
        3: NUMBER - seconds on screen (optional)
*/

params [["_text", "", [""]], ["_type", "info", [""]], ["_title", "", [""]], ["_duration", 7, [0]]];

if (!hasInterface || { _text == "" }) exitWith {};

// [hex for the title text, RGBA for the stripe]
(switch (_type) do {
    case "success": { ["#22c55e", [0.29, 0.81, 0.59, 1]] };
    case "warning": { ["#fbbf24", [0.96, 0.65, 0.14, 1]] };
    case "error": { ["#fca5a5", [0.97, 0.44, 0.41, 1]] };
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
private _markup = format [
    "<t size='1.45' font='RobotoCondensedBold' color='%1'>%2</t><br/><t size='1.35' color='#f1f5f9'>%3</t>",
    _colour, (_title splitString "<>&") joinString "", _text];

// A notice: [markup, stripe colour, expires at, controls [bg, bar, body] or [], height]
if (isNil "ALife_toasts") then { ALife_toasts = []; };
ALife_toasts = [[_markup, _rgba, diag_tickTime + _duration, [], 0]] + ALife_toasts;
while { count ALife_toasts > 4 } do {
    { ctrlDelete _x; } forEach ((ALife_toasts deleteAt (count ALife_toasts - 1)) select 3);
};

// One loop draws them all: re-creates cards whose screen closed, moves them
// to whatever's on top, stacks them, and fades them out when their time is up.
if (!isNil "ALife_toastLoop" && { !scriptDone ALife_toastLoop }) exitWith {};
ALife_toastLoop = [] spawn {
    private _x0 = safezoneX + safezoneW * 0.72;
    private _w = safezoneW * 0.27;
    private _top = safezoneY + safezoneH * 0.08;
    private _gap = safezoneH * 0.01;
    private _pad = safezoneH * 0.01;
    private _ours = [5600, 5500, 5400, 5300, 5100, 5000, 4900, 4800];

    while { ALife_toasts isNotEqualTo [] } do {
        // The screen on top: one of our menus, else Arma's inventory, else
        // the map (only while it's open -- Arma keeps the map display loaded
        // when it's closed, so a notice drawn there would be hidden), else
        // the game screen.
        private _display = displayNull;
        { private _d = findDisplay _x; if (!isNull _d) exitWith { _display = _d; }; } forEach (_ours + [602]);
        if (isNull _display && { visibleMap }) then { _display = findDisplay 12; };
        if (isNull _display) then { _display = findDisplay 46; };

        private _now = diag_tickTime;
        // Time's up: fade out and remove.
        {
            _x params ["", "", "_expires", "_ctrls"];
            if (_now >= _expires) then {
                { _x ctrlSetFade 1; _x ctrlCommit 0.4; } forEach _ctrls;
                _ctrls spawn { uiSleep 0.4; { ctrlDelete _x; } forEach _this; };
            };
        } forEach ALife_toasts;
        ALife_toasts = ALife_toasts select { _now < (_x select 2) };

        if (!isNull _display) then {
            private _yy = _top;
            {
                _x params ["_markup", "_rgba", "", "_ctrls", "_h"];
                // Its screen closed or another is on top now: draw it there.
                if (_ctrls isEqualTo [] || { (_ctrls findIf { isNull _x }) >= 0 } || { ctrlParent (_ctrls select 0) != _display }) then {
                    { ctrlDelete _x; } forEach _ctrls;
                    private _bg = _display ctrlCreate ["ALife_RscBackground", -1];
                    private _bar = _display ctrlCreate ["ALife_RscBackground", -1];
                    private _body = _display ctrlCreate ["ALife_RscStructuredText", -1];
                    _bg ctrlSetBackgroundColor [0.05, 0.055, 0.07, 0.95];
                    _bar ctrlSetBackgroundColor _rgba;
                    // Size the card to its text.
                    _body ctrlSetPosition [_x0 + safezoneW * 0.01, _yy, _w - safezoneW * 0.017, safezoneH * 0.2];
                    _body ctrlCommit 0;
                    _body ctrlSetStructuredText parseText _markup;
                    _h = ctrlTextHeight _body + _pad * 2;
                    _ctrls = [_bg, _bar, _body];
                    _bg ctrlSetPosition [_x0, _yy, _w, _h];
                    _bar ctrlSetPosition [_x0, _yy, safezoneW * 0.003, _h];
                    _body ctrlSetPosition [_x0 + safezoneW * 0.01, _yy + _pad, _w - safezoneW * 0.017, _h - _pad * 2];
                    { _x ctrlCommit 0; } forEach _ctrls;
                    _x set [3, _ctrls];
                    _x set [4, _h];
                };
                _ctrls params ["_bg", "_bar", "_body"];
                // Slide into its place in the stack.
                if (abs (((ctrlPosition _bg) select 1) - _yy) > 0.0001) then {
                    _bg ctrlSetPosition [_x0, _yy, _w, _h];
                    _bar ctrlSetPosition [_x0, _yy, safezoneW * 0.003, _h];
                    _body ctrlSetPosition [_x0 + safezoneW * 0.01, _yy + _pad, _w - safezoneW * 0.017, _h - _pad * 2];
                    { _x ctrlCommit 0.15; } forEach _ctrls;
                };
                _yy = _yy + _h + _gap;
            } forEach ALife_toasts;
        };
        uiSleep 0.1;
    };
};
