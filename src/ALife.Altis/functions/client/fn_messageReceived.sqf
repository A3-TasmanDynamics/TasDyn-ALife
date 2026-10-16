/*
    ALife_fnc_messageReceived (client, called by the server)

    A phone message arrived. Kept in the Messages app (last 50, this
    session only) with a notification.

    Arguments:
        0: STRING - sender name ("" = anonymous)
        1: STRING - "police", "ems" or "player"
        2: STRING - text
        3: STRING - time sent (in-game, HH:MM)
*/

params ["_from", "_channel", "_text", "_time"];

if (isNil "ALife_inbox") then { ALife_inbox = []; };
private _sender = ["Anonymous", _from] select (_from != "");
private _to = switch (_channel) do {
    case "police": { " (to Police)" };
    case "ems": { " (to EMS)" };
    default { "" };
};

ALife_inbox = ([[_sender + _to, _text, _time]] + ALife_inbox) select [0, 50];
ALife_unread = (missionNamespace getVariable ["ALife_unread", 0]) + 1;

[(_text select [0, 140]) splitString "<>" joinString "", "message", "Message from " + ((_sender + _to) splitString "<>" joinString "")] call ALife_fnc_toast;

if (!isNull (findDisplay 5000)) then { ["refresh"] call ALife_fnc_phone; };
