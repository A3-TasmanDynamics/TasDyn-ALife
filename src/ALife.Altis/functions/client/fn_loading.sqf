/*
    ALife_fnc_loading (client)

    The black loading overlay shown from joining until you're spawned. It's
    a title layer (cutRsc), so the spawn menu dialog sits on top of it and
    the world stays hidden behind both.

    Modes:
        ["show", text]  - show the overlay with a status line
        ["error", text] - show a problem (in red) with how to leave
        ["hide"]        - fade out to the game

    Text may use <br/> for line breaks.
*/

params ["_mode", ["_text", ""]];

if (!hasInterface) exitWith {};

private _fnc_display = {
    private _display = uiNamespace getVariable ["ALife_Loading", displayNull];
    if (isNull _display) then {
        "ALife_loading" cutRsc ["ALife_Loading", "PLAIN", 0, false];
        _display = uiNamespace getVariable ["ALife_Loading", displayNull];
    };
    _display
};

switch (_mode) do {
    case "show": {
        private _display = call _fnc_display;
        (_display displayCtrl 9102) ctrlSetStructuredText parseText format ["<t align='center' size='1.2'>%1</t>", _text];
        (_display displayCtrl 9103) ctrlSetText "";
    };
    case "error": {
        private _display = call _fnc_display;
        (_display displayCtrl 9102) ctrlSetStructuredText parseText format ["<t align='center' size='1.2' color='#fca5a5'>%1</t>", _text];
        (_display displayCtrl 9103) ctrlSetText "Press Esc and choose Abort to leave.";
    };
    case "hide": {
        "ALife_loading" cutFadeOut 1;
    };
};
