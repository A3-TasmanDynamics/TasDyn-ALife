/*
    ALife_fnc_radial (client)

    The radial menu (ui\radial.hpp): a title in a disc in the middle of the
    screen and up to 8 options around it. Click one to run it; Esc or
    clicking outside does nothing and closes it. Used for the Windows-key
    interactions (ALife_fnc_interact, ALife_fnc_vehicleMenu).

    Modes:
        ["open", title, options]
            options: [[label, code, args], ...] -- picking one closes the
            menu, then runs `args call code` (scheduled, once the menu is
            gone, so it may open another menu or dialog)
        ["onLoad", display]
        ["pick", index]
*/

#define IDD 5700
#define IDC_TITLE 5701
#define IDC_OPTION 5710

params ["_mode", ["_a", ""], ["_b", []]];

switch (_mode) do {
    case "open": {
        if (dialog) then { closeDialog 0; };
        ALife_radialTitle = _a;
        ALife_radialOptions = _b select [0, 8];
        createDialog "ALife_Radial";
    };

    case "onLoad": {
        private _display = _a;
        uiNamespace setVariable ["ALife_radialDisplay", _display];
        // Work in pixels so the ring is round on any screen shape.
        private _cx = safezoneX + safezoneW / 2;
        private _cy = safezoneY + safezoneH / 2;
        private _px = safezoneH / pixelH / 1080; // 1 = a 1080p screen
        private _disc = 150 * _px;              // the middle, in pixels
        {
            _x ctrlSetPosition [_cx - _disc / 2 * pixelW, _cy - _disc / 2 * pixelH, _disc * pixelW, _disc * pixelH];
            _x ctrlCommit 0;
        } forEach [_display displayCtrl 5702, _display displayCtrl 5703];
        private _title = _display displayCtrl IDC_TITLE;
        _title ctrlSetPosition [_cx - _disc * 0.42 * pixelW, _cy - 22 * _px * pixelH, _disc * 0.84 * pixelW, 60 * _px * pixelH];
        _title ctrlCommit 0;
        _title ctrlSetStructuredText parseText format [
            "<t align='center' size='%2' font='RobotoCondensedBold' color='#f1f5f9'>%1</t>",
            (ALife_radialTitle splitString "<>&") joinString "", 1.15];

        // The options, clockwise from the top.
        private _options = missionNamespace getVariable ["ALife_radialOptions", []];
        private _count = count _options;
        private _radius = 175 * _px;
        private _w = 190 * _px;
        private _h = 46 * _px;
        {
            _x params ["_label"];
            private _angle = 360 * _forEachIndex / (_count max 1);
            private _ox = _cx + (_radius * sin _angle) * pixelW;
            private _oy = _cy - (_radius * cos _angle) * pixelH;
            private _btn = _display ctrlCreate ["ALife_RadialButton", IDC_OPTION + _forEachIndex];
            _btn ctrlSetPosition [_ox - _w / 2 * pixelW, _oy - _h / 2 * pixelH, _w * pixelW, _h * pixelH];
            _btn ctrlCommit 0;
            _btn ctrlSetText _label;
            _btn ctrlAddEventHandler ["ButtonClick", { ["pick", ctrlIDC (_this select 0) - IDC_OPTION] call ALife_fnc_radial; }];
        } forEach _options;
    };

    case "pick": {
        private _option = (missionNamespace getVariable ["ALife_radialOptions", []]) param [_a, []];
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
