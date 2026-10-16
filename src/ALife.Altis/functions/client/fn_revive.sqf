/*
    ALife_fnc_revive (medic's client, scheduled)

    The Revive action: kneel over the body for CfgALife >> reviveTime
    seconds with a progress bar (ui\progress.hpp). Moving away, dying, or
    the body being revived/respawned by then cancels it. When it finishes,
    the server checks everything again and uses a medkit
    (ALife_fnc_requestRevive).

    Arguments:
        ["start", body]
*/

params ["_mode", ["_body", objNull]];

if (_mode != "start" || { isNull _body } || { !isNil "ALife_reviving" }) exitWith {};

private _medkits = 0;
{
    if ((_x select 0) == "medkit") then { _medkits = _x select 1; };
} forEach ((missionNamespace getVariable ["ALife_phone", createHashMap]) getOrDefault ["items", []]);
if (_medkits < 1) exitWith {
    hint "You need a medkit. Buy them at the hospital (Medical Supplies).";
};

ALife_reviving = true;
private _name = _body getVariable ["alife_name", "them"];
private _duration = getNumber (missionConfigFile >> "CfgALife" >> "reviveTime") max 1;
private _start = getPosATL player;
private _began = diag_tickTime;
private _done = false;

"ALife_progress" cutRsc ["ALife_Progress", "PLAIN", 0, false];
player playMoveNow "AinvPknlMstpSnonWnonDnon_medic_1";

while { true } do {
    private _elapsed = diag_tickTime - _began;
    private _display = uiNamespace getVariable ["ALife_Progress", displayNull];
    if (!isNull _display) then {
        (_display displayCtrl 9201) ctrlSetText format ["Reviving %1...", _name];
        (_display displayCtrl 9202) progressSetPosition (_elapsed / _duration);
    };

    if (_elapsed >= _duration) exitWith { _done = true; };
    if (!alive player || { player distance _start > 1.5 } || { player distance _body > 4 }
        || { !(_body getVariable ["alife_downed", false]) }) exitWith {};

    // Keep the medic animation going for the whole revive.
    if (animationState player != "ainvpknlmstpsnonwnondnon_medic_1") then {
        player playMoveNow "AinvPknlMstpSnonWnonDnon_medic_1";
    };
    sleep 0.1;
};

"ALife_progress" cutText ["", "PLAIN"];
if (alive player) then { player switchMove ""; };
ALife_reviving = nil;

if (_done) then {
    [player, _body] remoteExec ["ALife_fnc_requestRevive", 2];
} else {
    hint "Revive cancelled.";
};
