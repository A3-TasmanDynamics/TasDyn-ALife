/*
    ALife_fnc_spawnApproved (client, called by the server)

    The server's answer to ALife_fnc_requestSpawn: move here and go, or
    (empty position) an error and back to the spawn menu.

    Arguments:
        0: ARRAY  - position (ATL), or [] if refused
        1: STRING - spawn point name
        2: STRING - error message when refused
*/

params [["_pos", []], ["_name", ""], ["_error", ""]];

ALife_spawnPending = false;

if (_pos isEqualTo []) exitWith {
    hint _error;
    ["open"] call ALife_fnc_spawnMenu;
};

// Weapons from the slot's default kit aren't part of Altis Life loadouts.
removeAllWeapons player;

player setPosATL _pos;
player enableSimulation true;

// Death screen when this body dies (once per unit -- a new unit after
// every respawn).
if (isNil { player getVariable "alife_killedEH" }) then {
    player setVariable ["alife_killedEH", player addEventHandler ["Killed", {
        params ["_unit", "_killer", "_instigator"];
        ["start", _unit, _killer, _instigator] call ALife_fnc_deathScreen;
    }]];
};

["hide"] call ALife_fnc_loading;
titleText [format ["<t size='1.6' font='RobotoCondensedBold'>%1</t>", _name], "PLAIN DOWN", 1.5, true, true];

[] spawn {
    sleep 3; // a short grace period after landing
    player allowDamage true;
};
