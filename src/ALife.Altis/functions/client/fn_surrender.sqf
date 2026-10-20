/*
    ALife_fnc_surrender (client, Left Shift + B -- CfgALife >> surrenderKey)

    Surrender: weapon away and hands on your head until you press it again,
    get restrained, die or end up in a vehicle. It's public
    (alife_surrendered), so others can see it in your person menu, and
    the hands stay up if something knocks you out of the pose.

    Returns:
        BOOL - true if the key press was used
*/

if (!alive player || { dialog } || { !(missionNamespace getVariable ["ALife_spawned", false]) }) exitWith { false };
if (player getVariable ["alife_restrained", false]) exitWith { false };

// Already surrendered: hands down (the loop below sees it and lowers them).
if (player getVariable ["alife_surrendered", false]) exitWith {
    player setVariable ["alife_surrendered", false, true];
    true
};

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
        if ((animationState _unit) find "amovpercmstpssurwnondnon" == 0) then { _outSince = -1; } else {
            if (_outSince < 0) then { _outSince = diag_tickTime; };
            if (diag_tickTime - _outSince > 2) then { _unit playMoveNow _anim; _outSince = diag_tickTime; };
        };
        uiSleep 0.2;
    };

    // Hands down -- unless restrained (that pose takes over), dead or in a
    // vehicle. If the move out of the pose doesn't take, stand normally.
    if (!alive _unit || { vehicle _unit != _unit } || { _unit getVariable ["alife_restrained", false] }) exitWith {};
    if (_unit getVariable ["alife_surrendered", false]) exitWith {};
    private _stuck = { alive _unit && { (animationState _unit) find "amovpercmstpssurwnondnon" == 0 } && { !(_unit getVariable ["alife_surrendered", false]) } };
    _unit playMoveNow "AmovPercMstpSsurWnonDnon_AmovPercMstpSnonWnonDnon";
    uiSleep 2.5;
    if (call _stuck) then { _unit playMoveNow "AmovPercMstpSnonWnonDnon"; uiSleep 1.5; };
    if (call _stuck) then { _unit switchMove ""; }; // last resort, like after a revive
};
true
