/*
    ALife_fnc_resourceProgress (client, called by the server)

    The progress bar (ui\progress.hpp) and a kneeling work move while the
    server gathers or processes for you (ALife_fnc_requestGather,
    ALife_fnc_requestProcess). The server keeps the time and decides the
    result; moving out of the area stops it there.

        ["start", label, seconds, move]
*/

params [["_mode", "", [""]], ["_label", "", [""]], ["_duration", 1, [0]], ["_anim", "", [""]]];

if (_mode != "start" || { !hasInterface } || { !isNil "ALife_using" }) exitWith {};
if (dialog) then { closeDialog 0; };

[_label, _duration max 0.5, _anim] spawn {
    params ["_label", "_duration", "_anim"];
    ALife_using = true;
    private _began = diag_tickTime;
    private _outSince = -1;
    "ALife_progress" cutRsc ["ALife_Progress", "PLAIN", 0, false];
    if (_anim != "" && { vehicle player == player }) then { player playMoveNow _anim; };
    while { diag_tickTime - _began < _duration && { alive player } } do {
        private _display = uiNamespace getVariable ["ALife_Progress", displayNull];
        if (!isNull _display) then {
            (_display displayCtrl 9201) ctrlSetText _label;
            (_display displayCtrl 9202) progressSetPosition ((diag_tickTime - _began) / _duration);
        };
        // Keep working (re-started only after 2 s out of the move).
        if (_anim != "" && { vehicle player == player }) then {
            if ((animationState player) find "ainvpknlmstpsnonwnondnon_medic" == 0) then { _outSince = -1; } else {
                if (_outSince < 0) then { _outSince = diag_tickTime; };
                if (diag_tickTime - _outSince > 2) then { player playMoveNow _anim; _outSince = diag_tickTime; };
            };
        };
        sleep 0.1;
    };
    "ALife_progress" cutText ["", "PLAIN"];
    if (_anim != "" && { alive player } && { vehicle player == player }) then { player playMoveNow "AinvPknlMstpSnonWnonDnon_medicEnd"; };
    ALife_using = nil;
};
