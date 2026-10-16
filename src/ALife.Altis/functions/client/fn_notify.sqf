/*
    ALife_fnc_notify (client, called by the server)

    A short notice in the hint box (purchase done, item received, ...).

    Arguments:
        0: STRING - text
*/

params [["_text", "", [""]]];

if (!hasInterface || { _text == "" }) exitWith {};
hint parseText format ["<t size='1.05'>%1</t>", (_text splitString "<>") joinString ""];
