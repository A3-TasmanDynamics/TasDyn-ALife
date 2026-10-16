/*
    ALife_fnc_loadRecord (server)

    Loads a player's record from the database (creating a blank one for a
    first-time player). Tries 3 times, 2 seconds apart.

    Arguments:
        0: STRING - Steam UID

    Returns:
        HASHMAP - always has "status": "OK" (the full record), "BANNED"
        ("reason", "until"), or "ERROR" ("reason").
*/

params ["_uid"];

private _record = createHashMapFromArray [["status", "ERROR"], ["reason", "not tried"]];

for "_attempt" from 1 to 3 do {
    private _raw = ["load", [_uid]] call ALife_fnc_extension;

    _record = if ((_raw select [0, 1]) == "[") then {
        private _pairs = parseSimpleArray _raw;
        if (_pairs isEqualTo []) then {
            createHashMapFromArray [["status", "ERROR"], ["reason", "the database answer couldn't be read"]]
        } else {
            createHashMapFromArray _pairs
        };
    } else {
        createHashMapFromArray [["status", "ERROR"], ["reason", _raw]]
    };

    if ((_record getOrDefault ["status", "ERROR"]) in ["OK", "BANNED"]) exitWith {};

    diag_log format ["[ALife] load %1 failed (attempt %2/3): %3", _uid, _attempt, (_raw select [0, 300])];
    if (_attempt < 3) then { sleep 2; };
};

_record
