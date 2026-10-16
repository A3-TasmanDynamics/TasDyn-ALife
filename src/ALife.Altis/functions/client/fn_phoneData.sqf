/*
    ALife_fnc_phoneData (client, called by the server)

    The server's copy of what the phone shows. Kept in ALife_phone; an open
    phone or shop refreshes straight away.

    Arguments:
        0: STRING - faction
        1: ARRAY  - items [[class, count], ...]
        2: NUMBER - cash
        3: NUMBER - bank
        4: NUMBER - police level
        5: NUMBER - medic level
        6: STRING - staff rank key ("" = none)
        7: NUMBER - staff level
*/

params ["_faction", "_items", "_cash", "_bank", "_copLevel", "_medicLevel", "_staffRank", "_staffLevel"];

ALife_phone = createHashMapFromArray [
    ["faction", _faction],
    ["items", _items],
    ["cash", _cash],
    ["bank", _bank],
    ["copLevel", _copLevel],
    ["medicLevel", _medicLevel],
    ["staffRank", _staffRank],
    ["staffLevel", _staffLevel]
];

if (!isNull (findDisplay 5000)) then { ["refresh"] call ALife_fnc_phone; };
if (!isNull (findDisplay 5100)) then { ["refresh"] call ALife_fnc_shop; };
