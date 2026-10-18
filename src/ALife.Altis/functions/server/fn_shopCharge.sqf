/*
    ALife_fnc_shopCharge (server)

    Takes a shop payment: cash (a delta save with a one-off token) or bank
    (the extension's "bank purchase", through the ledger). On success the
    session record's cached balance follows.

    Arguments:
        0: STRING  - Steam UID
        1: STRING  - faction ("civ" / "cop" / "medic")
        2: HASHMAP - the player's session record (ALife_sessions)
        3: NUMBER  - amount, whole dollars
        4: STRING  - "cash" or "bank"
        5: STRING  - memo for the bank ledger, e.g. "Hatchback"

    Returns:
        STRING - "OK", "INSUFFICIENT", or the extension's error
*/

params ["_uid", "_faction", "_record", "_total", "_method", ["_memo", ""]];

private _token = format ["shop-%1-%2-%3", _uid, round (diag_tickTime * 1000), floor random 1e6];
private _amount = _total toFixed 0;

private _paid = if (_method == "bank") then {
    ["bank", ["purchase", _uid, _faction, _amount, _token, "", "", _memo]] call ALife_fnc_extension
} else {
    ["save", [_uid, _faction + "_cash", "-" + _amount, _token]] call ALife_fnc_extension
};

if (_paid == "OK") then {
    private _key = _faction + "_" + _method;
    _record set [_key, (_record getOrDefault [_key, 0]) - _total];
};
_paid
