/*
    ALife_fnc_requestRevive (server, called by the medic's client)

    A medic finished the Revive action on a downed player. Checks the medic
    (spawned as a medic, alive, close by, has a medkit) and the body (still
    downed, its player still connected), uses one medkit, and tells the
    downed player's machine to get up (ALife_fnc_revived). The player's
    machine then respawns them and ALife_fnc_completeRevive puts them back
    where they fell, with their gear.

    Arguments:
        0: OBJECT - the medic
        1: OBJECT - the downed body
*/

params [["_medic", objNull, [objNull]], ["_body", objNull, [objNull]]];

if (!isServer || { isNull _medic } || { isNull _body }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _medic }) exitWith {};

private _owner = owner _medic;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _medicUid = getPlayerUID _medic;

if ((_medic getVariable ["alife_faction", ""]) != "medic" || { !alive _medic }) exitWith {};
if (alive _body || { !(_body getVariable ["alife_downed", false]) }) exitWith { "They can't be revived." call _fnc_reply; };
if (_medic distance _body > 5) exitWith { "Get closer to them." call _fnc_reply; };

// Until they respawn, the body still belongs to the player's machine; if
// they've left, it belongs to the server.
private _uid = _body getVariable ["alife_uid", ""];
private _patientClient = owner _body;
if (_uid == "" || { _patientClient <= 2 }) exitWith { "They've left the server." call _fnc_reply; };

if (!([_medicUid, "medic", "medkit", -1] call ALife_fnc_itemsChange)) exitWith {
    "You need a medkit. Buy them at the hospital (Medical Supplies)." call _fnc_reply;
};

// Claim the body so a second medic can't revive it at the same time.
_body setVariable ["alife_downed", false, true];

if (isNil "ALife_revives") then { ALife_revives = createHashMap; };
ALife_revives set [_uid, [getPosATL _body, getUnitLoadout _body, _body, name _medic]];

diag_log format ["[ALife] %1 (%2) revived %3 with a medkit", name _medic, _medicUid, _uid];
[name _medic] remoteExecCall ["ALife_fnc_revived", _patientClient];
[format ["You revived %1.", _body getVariable ["alife_name", "them"]], "success"] call _fnc_reply;
[_medic] call ALife_fnc_requestPhoneData;
