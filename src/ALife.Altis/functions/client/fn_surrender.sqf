/*
    ALife_fnc_surrender (client, Left Shift + B -- CfgALife >> surrenderKey)

    Surrender: weapon away and hands on your head until you press it again,
    get restrained, die or end up in a vehicle. It's public
    (alife_surrendered), so others can see it in your person menu, and
    the hands stay up if something knocks you out of the pose.

    Walking: Arma has no walking hands-on-head move, so with the hands up
    the movement keys slide you along at walking pace in the pose (walls
    stop you, you follow the ground and floors). Sprinting (the Turbo key)
    while moving puts the hands down so you can run.

    Pressing it again puts the hands down: the game's move out of the pose,
    switched to on every machine (ALife_fnc_animSync) -- the pose can't be
    left with playMove. It also frees you from the pose if you're stuck in
    it without being surrendered.

    Arguments:
        "down" - (spawned) the move out of the pose; otherwise the key press

    Returns:
        BOOL - true if the key press was used
*/

#define POSE "amovpercmstpssurwnondnon"
#define POSE_OUT "AmovPercMstpSsurWnonDnon_AmovPercMstpSnonWnonDnon"

// The move out of the pose; if it doesn't take, stand normally.
if (_this isEqualTo "down") exitWith {
    private _unit = player;
    _unit switchMove POSE_OUT;
    [_unit, POSE_OUT] remoteExecCall ["ALife_fnc_animSync", 2];
    uiSleep 2.5;
    if (alive _unit && { (animationState _unit) == POSE } && { !(_unit getVariable ["alife_surrendered", false]) }) then {
        diag_log format ["[ALife] hands down: still in %1 after the move out -- standing", animationState _unit];
        _unit switchMove "AmovPercMstpSnonWnonDnon";
        [_unit, "AmovPercMstpSnonWnonDnon"] remoteExecCall ["ALife_fnc_animSync", 2];
    };
};

if (!alive player || { dialog } || { !(missionNamespace getVariable ["ALife_spawned", false]) }) exitWith { false };

// Surrendered (or stuck in the pose): hands down.
if ((player getVariable ["alife_surrendered", false]) || { (animationState player) == POSE }) exitWith {
    if (!isNil "ALife_surrenderLoop" && { !scriptDone ALife_surrenderLoop }) then { terminate ALife_surrenderLoop; };
    if (!isNil "ALife_surrenderWalk") then { removeMissionEventHandler ["EachFrame", ALife_surrenderWalk]; ALife_surrenderWalk = nil; };
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

// Walking in the pose: the movement keys slide you along, a frame at a time.
if (!isNil "ALife_surrenderWalk") then { removeMissionEventHandler ["EachFrame", ALife_surrenderWalk]; };
ALife_surrenderWalk = addMissionEventHandler ["EachFrame", {
    private _unit = player;
    if (!alive _unit || { !(_unit getVariable ["alife_surrendered", false]) } || { vehicle _unit != _unit }
        || { (animationState _unit) != POSE } || { dialog }) exitWith {};
    private _f = (inputAction "MoveForward") - (inputAction "MoveBack");
    private _s = ((inputAction "TurnRight") + (inputAction "MoveRight")) - ((inputAction "TurnLeft") + (inputAction "MoveLeft"));
    _f = (_f max -1) min 1;
    _s = (_s max -1) min 1;
    if (_f == 0 && { _s == 0 }) exitWith {};
    // Running: hands down.
    if (inputAction "Turbo" > 0) exitWith { [] call ALife_fnc_surrender; };
    // Walking pace (slower backwards / sideways).
    private _speed = [1.1, 0.7] select (_f < 0 || { _f == 0 });
    private _dir = getDir _unit;
    private _step = [
        ((sin _dir) * _f + (cos _dir) * _s),
        ((cos _dir) * _f - (sin _dir) * _s),
        0
    ];
    if (vectorMagnitude _step < 0.01) exitWith {};
    _step = (vectorNormalized _step) vectorMultiply (_speed * (diag_deltaTime min 0.1));
    private _pos = getPosASL _unit;
    private _next = _pos vectorAdd _step;
    // A wall / object in the way (knee and chest height, a little ahead).
    private _ahead = (vectorNormalized _step) vectorMultiply 0.45;
    if ((lineIntersectsSurfaces [_pos vectorAdd [0, 0, 0.5], _next vectorAdd _ahead vectorAdd [0, 0, 0.5], _unit, objNull, true, 1]) isNotEqualTo []
        || { (lineIntersectsSurfaces [_pos vectorAdd [0, 0, 1.3], _next vectorAdd _ahead vectorAdd [0, 0, 1.3], _unit, objNull, true, 1]) isNotEqualTo [] }) exitWith {};
    // The ground / floor under the next spot (small steps up or down only).
    private _hit = lineIntersectsSurfaces [_next vectorAdd [0, 0, 0.45], _next vectorAdd [0, 0, -1], _unit, objNull, true, 1];
    private _z = if (_hit isEqualTo []) then { getTerrainHeightASL _next } else { ((_hit select 0) select 0) select 2 };
    if (abs (_z - (_pos select 2)) > 0.45) exitWith {};
    _unit setPosASL [_next select 0, _next select 1, _z];
}];

ALife_surrenderLoop = [] spawn {
    private _unit = player;
    private _anim = "AmovPercMstpSsurWnonDnon";
    private _outSince = -1;
    _unit action ["SwitchWeapon", _unit, _unit, 100]; // weapon away
    _unit playMove _anim;
    ["You've surrendered: walk with your hands up, sprint or Left Shift + B to put them down.", "info"] call ALife_fnc_toast;

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
    if (!isNil "ALife_surrenderWalk") then { removeMissionEventHandler ["EachFrame", ALife_surrenderWalk]; ALife_surrenderWalk = nil; };
};
true
