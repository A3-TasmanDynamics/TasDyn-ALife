/*
    ALife_fnc_radial (client)

    The interaction wheel (ui\radial.hpp): up to 8 options as separate
    tiles around a centre disc, clockwise from the top. The slice under the mouse lights up and
    the centre shows its name (the title otherwise); click to pick it.
    Clicking the middle or outside the wheel, right-clicking or Esc closes
    it. Used for the Windows-key interactions (ALife_fnc_interact,
    ALife_fnc_vehicleMenu, ALife_fnc_personMenu, ALife_fnc_fine).

    Modes:
        ["open", title, options, subtitle]
            options: [[label, code, args], ...] -- picking one closes the
            wheel, then runs `args call code` (scheduled, once the wheel is
            gone, so it may open another menu or dialog). One option gets a
            "Close" slice beside it. Title / subtitle are plain text.
        ["onLoad", display]
        ["hover"]          - the mouse moved
        ["click", button]
        ["pick", index]
*/

#define IDD 5700
#define IDC_LABEL 5701
#define IDC_TILES 5702
#define IDC_WEDGE 5703
#define IDC_ACCENT 5705
#define IDC_CENTRE 5706
#define IDC_DIM 5709
#define IDC_SLICE 5710
// Sizes in pixels on a 1080p screen (scaled with the resolution); the
// images are 512 px across: centre disc 100, tiles 112-246 of its 256
// radius (make_radial.py).
#define WHEEL_R 200
#define CENTRE_R (100 / 256 * WHEEL_R)
#define TILE_IN (112 / 256 * WHEEL_R)
#define SLICE_R (179 / 256 * WHEEL_R)

params ["_mode", ["_a", ""], ["_b", []], ["_c", ""]];

private _display = uiNamespace getVariable ["ALife_radialDisplay", displayNull];
private _options = missionNamespace getVariable ["ALife_radialOptions", []];
private _count = count _options;
private _safe = { (_this splitString "<>&") joinString "" };
private _k = safezoneH / pixelH / 1080;

// The centre: the hovered option, or the title.
private _fnc_label = {
    params ["_hover"];
    private _title = (missionNamespace getVariable ["ALife_radialTitle", ""]) call _safe;
    private _sub = (missionNamespace getVariable ["ALife_radialSubtitle", ""]) call _safe;
    private _hint = "<br/><t align='center' size='0.85' color='#8b98b3'>Click to pick  ·  Esc to close</t>";
    (_display displayCtrl IDC_LABEL) ctrlSetStructuredText parseText (if (_hover < 0) then {
        format ["<t align='center' size='1.6' font='RobotoCondensedBold' color='#f1f5f9'>%1</t>%2%3", _title,
            if (_sub == "") then { "" } else { format ["<br/><t align='center' size='1' color='#fbbf24'>%1</t>", _sub] }, _hint]
    } else {
        format ["<t align='center' size='0.9' color='#8b98b3'>%1</t><br/><t align='center' size='1.6' font='RobotoCondensedBold' color='#ffffff'>%2</t>%3",
            toUpper _title, ((_options select _hover) select 0) call _safe, _hint]
    });
};

switch (_mode) do {
    case "open": {
        if (dialog) then { closeDialog 0; };
        private _list = _b select [0, 8];
        if (count _list == 1) then { _list pushBack ["Close", {}, []]; };
        ALife_radialTitle = _a;
        ALife_radialSubtitle = _c;
        ALife_radialOptions = _list;
        ALife_radialHover = -2; // nothing drawn yet
        createDialog "ALife_Radial";
    };

    case "onLoad": {
        _display = _a;
        uiNamespace setVariable ["ALife_radialDisplay", _display];
        private _n = (_count max 2) min 8;
        private _cx = safezoneX + safezoneW / 2;
        private _cy = safezoneY + safezoneH / 2;
        private _r = WHEEL_R * _k;
        {
            private _ctrl = _display displayCtrl _x;
            _ctrl ctrlSetPosition [_cx - _r * pixelW, _cy - _r * pixelH, 2 * _r * pixelW, 2 * _r * pixelH];
            _ctrl ctrlCommit 0;
        } forEach [IDC_TILES, IDC_WEDGE, IDC_ACCENT, IDC_CENTRE];
        (_display displayCtrl IDC_TILES) ctrlSetText format ["ui\radial\tiles_%1_ca.paa", _n];
        { (_display displayCtrl _x) ctrlShow false; } forEach [IDC_WEDGE, IDC_ACCENT];

        private _label = _display displayCtrl IDC_LABEL;
        private _lw = CENTRE_R * 1.8 * _k;
        _label ctrlSetPosition [_cx - _lw / 2 * pixelW, _cy - 38 * _k * pixelH, _lw * pixelW, 84 * _k * pixelH];
        _label ctrlCommit 0;

        // One label per slice, at the middle of its part of the ring.
        // One label per slice, centred on its tile: as wide as the tile is
        // at that radius, wrapping onto a second line when it needs to.
        private _sr = SLICE_R * _k;
        private _sw = ((2 * SLICE_R * sin (180 / _n)) min 150) * 0.86 * _k;
        private _lineH = 21 * _k;
        {
            private _angle = 360 * _forEachIndex / _n;
            private _label = (_x select 0) call _safe;
            private _lines = [1, 2] select ((count _label) * 9 * _k > _sw); // ~9 px a character at this size
            private _text = _display ctrlCreate ["ALife_RscStructuredText", IDC_SLICE + _forEachIndex];
            _text ctrlSetPosition [
                _cx + (_sr * sin _angle - _sw / 2) * pixelW,
                _cy + (-_sr * cos _angle - _lineH * _lines / 2 - 2 * _k) * pixelH,
                _sw * pixelW, (_lineH * 2 + 6 * _k) * pixelH];
            _text ctrlCommit 0;
        } forEach _options;

        _display displayAddEventHandler ["MouseMoving", { ["hover"] call ALife_fnc_radial; false }];
        _display displayAddEventHandler ["MouseButtonUp", { ["click", _this select 1] call ALife_fnc_radial; false }];
        ["hover"] call ALife_fnc_radial;
    };

    case "hover": {
        if (isNull _display || { _count == 0 }) exitWith {};
        private _n = (_count max 2) min 8;
        getMousePosition params ["_mx", "_my"];
        private _dx = (_mx - (safezoneX + safezoneW / 2)) / pixelW;
        private _dy = (_my - (safezoneY + safezoneH / 2)) / pixelH;
        private _dist = sqrt (_dx * _dx + _dy * _dy);
        private _hover = -1;
        if (_dist > TILE_IN * _k && { _dist < WHEEL_R * 1.25 * _k }) then {
            private _angle = ((_dx atan2 -_dy) + 360 + 180 / _n) mod 360; // 0 = top, clockwise
            _hover = floor (_angle / (360 / _n));
            if (_hover >= _count) then { _hover = -1; };
        };
        if (_hover == (missionNamespace getVariable ["ALife_radialHover", -2])) exitWith {};
        ALife_radialHover = _hover;

        { (_display displayCtrl _x) ctrlShow (_hover >= 0); } forEach [IDC_WEDGE, IDC_ACCENT];
        if (_hover >= 0) then {
            // One image per slice position (never rotated: Arma stretches rotated pictures).
            (_display displayCtrl IDC_WEDGE) ctrlSetText format ["ui\radial\wedge_%1_%2_ca.paa", _n, _hover];
            (_display displayCtrl IDC_ACCENT) ctrlSetText format ["ui\radial\accent_%1_%2_ca.paa", _n, _hover];
        };
        for "_i" from 0 to _count - 1 do {
            private _text = _display displayCtrl (IDC_SLICE + _i);
            _text ctrlSetStructuredText parseText format ["<t align='center' size='%2' font='%4' color='%3'>%1</t>",
                ((_options select _i) select 0) call _safe, 1.4, ["#c9d2e3", "#ffffff"] select (_i == _hover),
                ["RobotoCondensed", "RobotoCondensedBold"] select (_i == _hover)];
        };
        [_hover] call _fnc_label;
    };

    case "click": {
        if (isNull _display) exitWith {};
        private _hover = missionNamespace getVariable ["ALife_radialHover", -1];
        if (_a == 0 && { _hover >= 0 }) exitWith { ["pick", _hover] call ALife_fnc_radial; };
        closeDialog 0; // the middle, outside the wheel, or right-click
    };

    case "pick": {
        private _option = _options param [_a, []];
        closeDialog 0;
        if (_option isEqualTo []) exitWith {};
        _option params ["", "_code", ["_args", []]];
        [_code, _args] spawn {
            params ["_code", "_args"];
            waitUntil { !dialog };
            _args call _code;
        };
    };
};
