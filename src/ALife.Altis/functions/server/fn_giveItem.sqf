/*
    ALife_fnc_giveItem (server, called by the client)

    Gives virtual items to another player standing nearby, or discards them
    (no target).

    Arguments:
        0: OBJECT - the giver's unit
        1: OBJECT - the receiver's unit (objNull = discard)
        2: STRING - item class
        3: NUMBER - quantity
*/

params [["_unit", objNull, [objNull]], ["_target", objNull, [objNull]], ["_class", "", [""]], ["_qty", 0, [0]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { [_this] remoteExecCall ["ALife_fnc_notify", _owner]; };

private _uid = getPlayerUID _unit;
private _faction = _unit getVariable ["alife_faction", ""];
_qty = round _qty;
if (_faction == "" || { !alive _unit } || { _qty < 1 } || { !isClass (missionConfigFile >> "CfgItems" >> _class) }) exitWith {};
private _name = getText (missionConfigFile >> "CfgItems" >> _class >> "displayName");

if (isNull _target) exitWith {
    if ([_uid, _faction, _class, -_qty] call ALife_fnc_itemsChange) then {
        (format ["Discarded %1 x%2.", _name, _qty]) call _fnc_reply;
        [_unit] call ALife_fnc_requestPhoneData;
    } else {
        (format ["You don't have %1 x%2.", _name, _qty]) call _fnc_reply;
    };
};

private _targetUid = getPlayerUID _target;
private _targetFaction = _target getVariable ["alife_faction", ""];
private _maxDistance = getNumber (missionConfigFile >> "CfgALife" >> "giveDistance");
if (_target == _unit || { !isPlayer _target } || { !alive _target } || { _targetFaction == "" }) exitWith {
    "They can't receive items right now." call _fnc_reply;
};
if (_unit distance _target > _maxDistance) exitWith {
    (format ["Get within %1 m of them first.", _maxDistance]) call _fnc_reply;
};
if (!([_uid, _faction, _class, -_qty] call ALife_fnc_itemsChange)) exitWith {
    (format ["You don't have %1 x%2.", _name, _qty]) call _fnc_reply;
};
if (!([_targetUid, _targetFaction, _class, _qty] call ALife_fnc_itemsChange)) exitWith {
    [_uid, _faction, _class, _qty] call ALife_fnc_itemsChange; // give them back
    "That didn't go through. Try again." call _fnc_reply;
};

diag_log format ["[ALife] %1 gave %2 x%3 to %4", _uid, _class, _qty, _targetUid];
(format ["Gave %1 x%2 to %3.", _name, _qty, name _target]) call _fnc_reply;
[format ["%1 gave you %2 x%3.", name _unit, _name, _qty]] remoteExecCall ["ALife_fnc_notify", owner _target];
[_unit] call ALife_fnc_requestPhoneData;
[_target] call ALife_fnc_requestPhoneData;
