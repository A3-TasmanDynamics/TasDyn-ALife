/*
    ALife_fnc_surrender (client, Left Shift + B -- CfgALife >> surrenderKey)

    Surrender: weapon away and hands on your head until you press it again,
    get restrained, die or end up in a vehicle. It's public
    (alife_surrendered), so others can see it in your person menu, and
    the hands stay up if something knocks you out of the pose.

    Pressing it again puts the hands down -- done here, not by the loop, so
    it works even if the loop is gone (and it frees you from the pose if
    you're stuck in it without being surrendered).

    Arguments:
        "down" - (spawned) the move out of the pose; otherwise the key press

    Returns:
        BOOL - true if the key press was used
*/

#define POSE "amovpercmstpssurwnondnon"

// The move out of the pose; if it doesn't take, stand normally.
if (_this isEqualTo "down") exitWith {
    private _unit = player;
    _unit playMoveNow "AmovPercMstpSsurWnonDnon_AmovPercMstpSnonWnonDnon";
    uiSleep 2;
    if (alive _unit && { (animationState _unit) == POSE } && { !(_unit getVariable ["alife_surrendered", false]) }) then {
        diag_log format ["[ALife] hands down: still in %1 after the move out -- switchMove", animationState _unit];
        _unit switchMove "AmovPercMstpSnonWnonDnon";
    };
};

if (!alive player || { dialog } || { !(missionNamespace getVariable ["ALife_spawned", false]) }) exitWith { false };

// Surrendered (or stuck in the pose): hands down.
if ((player getVariable ["alife_surrendered", false]) || { (animationState player) == POSE }) exitWith {
    if (!isNil "ALife_surrenderLoop" && { !scriptDone ALife_surrenderLoop }) then { terminate ALife_surrenderLoop; };
    diag_log format ["[ALife] hands down (surrendered %1, anim %2)", player getVariable ["alife_surrendered", false], animationState player];
    player setVariable ["alife_surrendered", false, true];
    // (Restrained for real: their pose takes over again straight after.)
    if (vehicle player == player) then { ALife_surrenderLoop = "down" spawn ALife_fnc_surrender; };
    true
};

if (player getVariable ["alife_restrained", false]) exitWith { false };

if (vehicle player != player) exitWith { ["You can't surrender in a vehicle.", "warning"] call ALife_fnc_toast; true };
if (surfaceIsWater getPosASL player && { (getPosASL player select 2) < 0.5 }) exitWith { false };

// A loop still finishing from the last time is replaced by a new one.
if (!isNil "ALife_surrenderLoop" && { !scriptDone ALife_surrenderLoop }) then { terminate ALife_surrenderLoop; };
player setVariable ["alife_surrendered", true, true];

ALife_surrenderLoop = [] spawn {
    private _unit = player;
    private _anim = "AmovPercMstpSsurWnonDnon";
    private _outSince = -1;
    _unit action ["SwitchWeapon", _unit, _unit, 100]; // weapon away
    _unit playMove _anim;
    ["You've surrendered. Left Shift + B to put your hands down.", "info"] call ALife_fnc_toast;

    while { alive _unit && { _unit == player } && { _unit getVariable ["alife_surrendered", false] } } do {
        // Restrained or in a vehicle: no longer surrendered.
        if (vehicle _unit != _unit || { _unit getVariable ["alife_restrained", false] }) exitWith {
            _unit setVariable ["alife_surrendered", false, true];
        };
        if (currentWeapon _unit != "") then { _unit action ["SwitchWeapon", _unit, _unit, 100]; };
        // Keep the hands up (re-started after 2 s out of the pose).
        if ((animationState _unit) == POSE) then { _outSince = -1; } else {
            if (_outSince < 0) then { _outSince = diag_tickTime; };
            if (diag_tickTime - _outSince > 2) then { _unit playMoveNow _anim; _outSince = diag_tickTime; };
        };
        uiSleep 0.2;
    };
    // Ended by the key (it lowers the hands), restraints, death or a vehicle.
};
true
