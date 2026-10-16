/*
    ALife_fnc_bankTransfer (server, called by the client)

    Sends money from the sender's bank (the faction they're playing) to
    another player's bank (the faction they're playing). Any distance.

    Arguments:
        0: OBJECT - sender's unit
        1: OBJECT - recipient's unit
        2: NUMBER - amount (whole dollars)
*/

params [["_unit", objNull, [objNull]], ["_target", objNull, [objNull]], ["_amount", 0, [0]]];

if (!isServer || { isNull _unit } || { isNull _target }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };

private _uid = getPlayerUID _unit;
private _faction = _unit getVariable ["alife_faction", ""];
private _targetUid = getPlayerUID _target;
private _targetFaction = _target getVariable ["alife_faction", ""];
_amount = round _amount;

if (_faction == "" || { !alive _unit }) exitWith {};
if (_target == _unit || { !isPlayer _target } || { _targetFaction == "" }) exitWith {
    "They can't receive money right now." call _fnc_reply;
};
if (_amount < 1 || { _amount > 10000000 }) exitWith { "Enter an amount between $1 and $10,000,000." call _fnc_reply; };

private _token = format ["xfer-%1-%2-%3", _uid, round (diag_tickTime * 1000), floor random 1e6];
private _result = ["bank", ["transfer", _uid, _faction, _amount toFixed 0, _token, _targetUid, _targetFaction, "Phone transfer"]] call ALife_fnc_extension;

if (_result != "OK") exitWith {
    diag_log format ["[ALife] transfer %1 -> %2 of %3 refused: %4", _uid, _targetUid, _amount, _result];
    (["The transfer didn't go through. Try again.", "You don't have that much in the bank."] select (_result == "INSUFFICIENT")) call _fnc_reply;
};

{
    _x params ["_who", "_fac", "_change"];
    private _rec = ALife_sessions getOrDefault [_who, createHashMap];
    if ((_rec getOrDefault ["status", ""]) == "OK") then {
        _rec set [_fac + "_bank", (_rec getOrDefault [_fac + "_bank", 0]) + _change];
    };
} forEach [[_uid, _faction, -_amount], [_targetUid, _targetFaction, _amount]];

diag_log format ["[ALife] %1 sent $%2 to %3", _uid, _amount, _targetUid];
[format ["Sent $%1 to %2.", [_amount] call BIS_fnc_numberText, name _target], "success"] call _fnc_reply;
[format ["%1 sent you $%2 (bank).", name _unit, [_amount] call BIS_fnc_numberText], "success"] remoteExecCall ["ALife_fnc_notify", owner _target];
[_unit] call ALife_fnc_requestPhoneData;
[_target] call ALife_fnc_requestPhoneData;
