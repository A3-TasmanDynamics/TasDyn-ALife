/*
    ALife_fnc_notify (client, called by the server)

    A notice from the server (purchase done, item received, ...), shown as
    one of our notifications (ALife_fnc_toast).

    Arguments:
        0: STRING - text
        1: STRING - "info", "success", "warning", "error" or "message" (default "info")
        2: STRING - title (optional)
*/

params [["_text", "", [""]], ["_type", "info", [""]], ["_title", "", [""]]];

if (!hasInterface || { _text == "" }) exitWith {};
[(_text splitString "<>") joinString "", _type, _title] call ALife_fnc_toast;
