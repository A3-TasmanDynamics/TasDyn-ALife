/*
    ALife_fnc_jump (client, Left Shift + Space -- CfgALife >> Jump)

    A jump while running on foot: a push up (and a little forward), with
    the vault animation when you're carrying your rifle (the game only has
    that one; it shows for everyone). Not standing still, crouched, in the
    air, in a vehicle, restrained, surrendered, downed or while placing a
    police object; once every `cooldown` seconds.

    Returns:
        BOOL - true if the key press was used
*/

private _cfg = missionConfigFile >> "CfgALife" >> "Jump";
if (!alive player || { dialog } || { vehicle player != player } || { !isNil "ALife_placing" }) exitWith { false };
if (!(missionNamespace getVariable ["ALife_spawned", false])) exitWith { false };
if ((player getVariable ["alife_restrained", false]) || { player getVariable ["alife_surrendered", false] } || { player getVariable ["alife_downed", false] }) exitWith { false };
if (!isTouchingGround player || { stance player != "STAND" } || { speed player < getNumber (_cfg >> "minSpeed") } || { ((velocity player) select 2) > 1 }) exitWith { false };
if (time - (missionNamespace getVariable ["ALife_jumpAt", -100]) < getNumber (_cfg >> "cooldown")) exitWith { true };
ALife_jumpAt = time;

private _v = velocity player;
private _dir = getDir player;
private _forward = getNumber (_cfg >> "forward");
player setVelocity [(_v select 0) + (sin _dir) * _forward, (_v select 1) + (cos _dir) * _forward, getNumber (_cfg >> "up")];
if (currentWeapon player != "" && { currentWeapon player == primaryWeapon player }) then {
    player playMoveNow "AovrPercMrunSrasWrflDf";
};
true
