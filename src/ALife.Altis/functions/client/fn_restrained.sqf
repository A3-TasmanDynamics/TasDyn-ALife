/*
    ALife_fnc_restrained (client, called by the server)

    You've been restrained (ALife_fnc_personAction): hands behind your back
    until you're released (alife_restrained goes false) or die. Hard cuffs
    (and zip ties) hold you still; a soft cuff (alife_softCuffed) lets you
    walk, slowly. While
    restrained you can't hold a weapon, drive, or use the phone, inventory
    or the Windows key (initPlayerLocal.sqf blocks the keys).

        ["start", by]  - by: who restrained you
*/

params [["_mode", "", [""]], ["_by", "", [""]]];

if (_mode != "start" || { !hasInterface }) exitWith {};
[format ["%1 restrained you.", _by], "warning"] call ALife_fnc_toast;
if (!isNil "ALife_restrainLoop" && { !scriptDone ALife_restrainLoop }) exitWith {};

ALife_restrainLoop = [] spawn {
    private _unit = player;
    private _anim = "AmovPercMstpSnonWnonDnon_Ease";
    private _outSince = -1;
    if (dialog) then { closeDialog 0; };
    _unit action ["SwitchWeapon", _unit, _unit, 100]; // weapon away
    private _soft = _unit getVariable ["alife_softCuffed", false];
    if (_soft) then {
        _unit forceWalk true;
        _unit setAnimSpeedCoef 0.6;
    } else {
        _unit playMoveNow _anim;
    };

    while { alive _unit && { _unit == player } && { _unit getVariable ["alife_restrained", false] } } do {
        // No driving, no menus (the fine prompt is allowed), no Arma inventory.
        if (vehicle _unit != _unit && { driver (vehicle _unit) == _unit }) then { moveOut _unit; };
        if (dialog && { isNull (findDisplay 5700) }) then { closeDialog 0; };
        if (!isNull (findDisplay 602)) then { (findDisplay 602) closeDisplay 2; };
        if (currentWeapon _unit != "") then { _unit action ["SwitchWeapon", _unit, _unit, 100]; };
        // Escorted by someone who went down (or left): free of them.
        private _escort = attachedTo _unit;
        if (!isNull _escort && { !alive _escort }) then { detach _unit; };
        // Hard cuffs: keep the hands behind the back (re-started after 2 s
        // out of it). A soft cuff: walking only.
        if (_soft) then { if (!isForcedWalk _unit) then { _unit forceWalk true; }; };
        if (!_soft && { vehicle _unit == _unit }) then {
            if ((animationState _unit) find "amovpercmstpsnonwnondnon_ease" == 0) then { _outSince = -1; } else {
                if (_outSince < 0) then { _outSince = diag_tickTime; };
                if (diag_tickTime - _outSince > 2) then { _unit playMoveNow _anim; _outSince = diag_tickTime; };
            };
        };
        sleep 0.25;
    };

    if (!isNull attachedTo _unit) then { detach _unit; };
    _unit forceWalk false;
    _unit setAnimSpeedCoef 1;
    if (!_soft && { alive _unit } && { vehicle _unit == _unit }) then { _unit playMoveNow "AmovPercMstpSnonWnonDnon_EaseOut"; };
};
