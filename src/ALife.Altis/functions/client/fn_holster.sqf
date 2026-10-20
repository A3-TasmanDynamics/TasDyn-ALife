/*
    ALife_fnc_holster (client, Left Shift + H -- CfgALife >> holsterKey)

    Holster: puts away the weapon in your hands (it stays on you). Press it
    again to take the same weapon back out. Not while restrained or
    surrendered, or in a vehicle.

    Returns:
        BOOL - true if the key press was used
*/

if (!alive player || { dialog } || { !(missionNamespace getVariable ["ALife_spawned", false]) }) exitWith { false };
if (vehicle player != player) exitWith { false };
if ((player getVariable ["alife_restrained", false]) || { player getVariable ["alife_surrendered", false] }) exitWith { true };

private _current = currentWeapon player;
if (_current != "") exitWith {
    ALife_holstered = _current;
    player action ["SwitchWeapon", player, player, 100];
    true
};

// Hands empty: the one you holstered, if you still have it.
private _weapon = missionNamespace getVariable ["ALife_holstered", ""];
if (_weapon == "" || { !(player hasWeapon _weapon) }) then {
    _weapon = [primaryWeapon player, handgunWeapon player] select (primaryWeapon player == "");
};
if (_weapon == "") exitWith { ["You don't have a weapon to draw.", "info"] call ALife_fnc_toast; true };
player selectWeapon _weapon;
ALife_holstered = "";
true
