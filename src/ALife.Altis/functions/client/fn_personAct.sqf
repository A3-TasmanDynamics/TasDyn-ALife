/*
    ALife_fnc_personAct (client, scheduled)

    Timed things you do to another player, with the progress bar
    (ui\progress.hpp) and a kneeling medic move; moving away, either of you
    dying, or them going down cancels it. Then the server checks it again
    (ALife_fnc_personAction).

        ["heal", unit]  - medics: patch a hurt player up (uses a Medkit)
*/

params [["_mode", "", [""]], ["_target", objNull, [objNull]]];

if (!hasInterface || { isNull _target } || { !isNil "ALife_using" } || { !isNil "ALife_reviving" }) exitWith {};
private _name = _target getVariable ["alife_name", name _target];
private _have = { ((missionNamespace getVariable ["ALife_phone", createHashMap]) getOrDefault ["items", []]) findIf { (_x select 0) == _this && { (_x select 1) > 0 } } >= 0 };

if (_mode != "heal") exitWith {};
if (!alive _target || { damage _target < 0.01 }) exitWith { [format ["%1 isn't hurt.", _name], "info"] call ALife_fnc_toast; };
if !("medkit" call _have) exitWith { ["You need a Medkit. Buy them at the hospital (Medical Supplies).", "warning"] call ALife_fnc_toast; };

ALife_using = true;
private _duration = getNumber (missionConfigFile >> "CfgALife" >> "healTime") max 1;
private _anim = "AinvPknlMstpSnonWnonDnon_medic4";
private _start = getPosATL player;
private _began = diag_tickTime;
private _outSince = -1;
private _done = false;

"ALife_progress" cutRsc ["ALife_Progress", "PLAIN", 0, false];
player playMoveNow _anim;
while { true } do {
    private _elapsed = diag_tickTime - _began;
    private _display = uiNamespace getVariable ["ALife_Progress", displayNull];
    if (!isNull _display) then {
        (_display displayCtrl 9201) ctrlSetText format ["Treating %1...", _name];
        (_display displayCtrl 9202) progressSetPosition (_elapsed / _duration);
    };
    if (_elapsed >= _duration) exitWith { _done = true; };
    if (!alive player || { !alive _target } || { player distance _start > 1.5 } || { player distance _target > 4 }) exitWith {};
    // Re-start the loop only after 2 s out of it (it jitters while blending in).
    if ((animationState player) find "ainvpknlmstpsnonwnondnon_medic" == 0) then { _outSince = -1; } else {
        if (_outSince < 0) then { _outSince = diag_tickTime; };
        if (diag_tickTime - _outSince > 2) then { player playMoveNow _anim; _outSince = diag_tickTime; };
    };
    sleep 0.1;
};
"ALife_progress" cutText ["", "PLAIN"];
if (alive player) then { player playMoveNow "AinvPknlMstpSnonWnonDnon_medicEnd"; };
ALife_using = nil;

if (_done) then {
    [player, "heal", _target] remoteExecCall ["ALife_fnc_personAction", 2];
} else {
    ["Cancelled.", "warning"] call ALife_fnc_toast;
};
