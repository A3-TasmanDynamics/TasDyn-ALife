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
        8: ARRAY  - licence keys for this faction (CfgLicences)
        9: ARRAY  - staff permission keys (website ranks; the Admin app)
*/

params ["_faction", "_items", "_cash", "_bank", "_copLevel", "_medicLevel", "_staffRank", "_staffLevel", ["_licences", []], ["_perms", []]];

ALife_phone = createHashMapFromArray [
    ["faction", _faction],
    ["items", _items],
    ["cash", _cash],
    ["bank", _bank],
    ["copLevel", _copLevel],
    ["medicLevel", _medicLevel],
    ["staffRank", _staffRank],
    ["staffLevel", _staffLevel],
    ["licences", _licences],
    ["perms", _perms]                 // staff permissions (website ranks): the Admin app
];

if (!isNull (findDisplay 5000)) then { ["refresh"] call ALife_fnc_phone; };
if (!isNull (findDisplay 5100)) then { ["refresh"] call ALife_fnc_shop; };
if (!isNull (findDisplay 5500)) then { ["refresh"] call ALife_fnc_clothingShop; };
if (!isNull (findDisplay 5600)) then { ["refresh"] call ALife_fnc_vehicleShop; };
if (!isNull (findDisplay 5300)) then { ["refresh"] call ALife_fnc_transfer; };
if (!isNull (findDisplay 5400)) then { ["refresh"] call ALife_fnc_atm; };
