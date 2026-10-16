/*
    ALife_fnc_atmAction (server, called by the client from the ATM menu)

    Deposit cash into, or withdraw cash from, the bank account of the
    faction the player is playing. Only at an ATM: the server checks there's
    one next to them (map objects, by model -- ALife_fnc_isAtm). The money
    moves in one database transaction (extension `bank` deposit/withdraw).

    Arguments:
        0: OBJECT - the player
        1: STRING - "deposit" or "withdraw"
        2: NUMBER - amount (whole dollars)
*/

params [["_unit", objNull, [objNull]], ["_op", "", [""]], ["_amount", 0, [0]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};
if (!(_op in ["deposit", "withdraw"])) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;
private _faction = _unit getVariable ["alife_faction", ""];
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
if (!alive _unit || { _faction == "" } || { (_record getOrDefault ["status", ""]) != "OK" }) exitWith {};

private _range = getNumber (missionConfigFile >> "CfgALife" >> "atmRange") + 2;
private _near = (nearestTerrainObjects [_unit, [], _range, false]) + (nearestObjects [_unit, [], _range]);
if ((_near findIf { [_x] call ALife_fnc_isAtm }) < 0) exitWith { "You need to be at an ATM." call _fnc_reply; };

_amount = floor _amount;
if (_amount < 1 || { _amount > 100000000 }) exitWith { "Enter an amount." call _fnc_reply; };

private _token = format ["atm-%1-%2-%3", _uid, round (diag_tickTime * 1000), floor random 1e6];
private _result = ["bank", [_op, _uid, _faction, _amount toFixed 0, _token, "", "", ""]] call ALife_fnc_extension;

if (_result != "OK") exitWith {
    diag_log format ["[ALife] ATM %1 of %2 for %3 refused: %4", _op, _amount, _uid, _result];
    (switch (_result) do {
        case "INSUFFICIENT": { ["You don't have that much in the bank.", "You don't have that much cash on you."] select (_op == "deposit") };
        default { "The ATM couldn't do that. Try again." };
    }) call _fnc_reply;
};

private _change = [_amount, -_amount] select (_op == "deposit"); // to the cash in your pocket
_record set [_faction + "_cash", (_record getOrDefault [_faction + "_cash", 0]) + _change];
_record set [_faction + "_bank", (_record getOrDefault [_faction + "_bank", 0]) - _change];

diag_log format ["[ALife] ATM: %1 %2 $%3 (%4)", _uid, ["withdrew", "deposited"] select (_op == "deposit"), _amount, _faction];
[format ["%1 $%2.", ["Withdrew", "Deposited"] select (_op == "deposit"), [_amount] call BIS_fnc_numberText], "success"] call _fnc_reply;
[_unit] call ALife_fnc_requestPhoneData;
