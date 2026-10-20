/*
    ALife_fnc_adminApply (client, called by the server)

    A staff action that happens on this player's own machine
    (ALife_fnc_adminAction has already checked the permission):

        ["bring", staff, position]       - moved next to the staff member
        ["freeze", staff, frozen]        - can't move while frozen
        ["message", staff, text]         - a private message from staff
        ["warn", staff, reason]          - a warning
        ["kick", staff, reason]          - shown the reason; the server then kicks them
                                           (or, without its password, this ends the mission)
        ["announce", staff, text]        - to everyone
*/

params [["_mode", "", [""]], ["_by", "", [""]], ["_value", ""]];

if (!hasInterface) exitWith {};
private _safe = { (_this splitString "<>&") joinString "" };

switch (_mode) do {
    case "bring": {
        if !(_value isEqualType []) exitWith {};
        if (vehicle player != player) then { moveOut player; };
        player setPosATL [(_value select 0) + 1.5, (_value select 1) + 1.5, _value select 2];
        [format ["%1 (staff) brought you to them.", _by call _safe], "info", "Staff"] call ALife_fnc_toast;
    };
    case "freeze": {
        if !(_value isEqualType true) exitWith {};
        player enableSimulation !_value;
        [[format ["%1 (staff) unfroze you.", _by call _safe], format ["%1 (staff) froze you.", _by call _safe]] select _value, "warning", "Staff"] call ALife_fnc_toast;
    };
    case "message": {
        [_value call _safe, "message", format ["Staff: %1", _by call _safe], 15] call ALife_fnc_toast;
    };
    case "warn": {
        [format ["%1<br/><t size='0.9' color='#8b98b3'>From %2. Repeated warnings lead to a ban.</t>", _value call _safe, _by call _safe], "error", "Warning from staff", 20] call ALife_fnc_toast;
    };
    case "kick": {
        if (dialog) then { closeDialog 0; };
        titleText [format ["<t size='1.6' font='RobotoCondensedBold' color='#fca5a5'>%1</t><br/><t size='1.1'>by %2</t>", _value call _safe, _by call _safe], "BLACK", 1, true, true];
        [] spawn {
            uiSleep 12; // the server's kick normally lands first (after 5 s)
            endMission "LOSER";
        };
    };
    case "announce": {
        [_value call _safe, "info", format ["Announcement - %1", _by call _safe], 20] call ALife_fnc_toast;
    };
};
