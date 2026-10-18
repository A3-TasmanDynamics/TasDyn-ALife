/*
    ALife_fnc_spawnApproved (client, called by the server)

    The server's answer to ALife_fnc_requestSpawn: move here and go, or
    (empty position) an error and back to the spawn menu.

    Arguments:
        0: ARRAY  - position (ATL), or [] if refused
        1: STRING - spawn point name
        2: STRING - error message when refused
        3: ARRAY  - saved loadout to put on ([] = none: the slot's kit
                    without weapons)
*/

params [["_pos", []], ["_name", ""], ["_error", ""], ["_loadout", []]];

ALife_spawnPending = false;

if (_pos isEqualTo []) exitWith {
    [_error, "warning"] call ALife_fnc_toast;
    ["open"] call ALife_fnc_spawnMenu;
};

if (_loadout isNotEqualTo []) then {
    player setUnitLoadout _loadout;
} else {
    // Weapons from the slot's default kit aren't part of Altis Life loadouts.
    removeAllWeapons player;
};

player setPosATL _pos;
player enableSimulation true;

// Death screen, revive/shop actions, settings (once per unit).
call ALife_fnc_setupUnit;
[true] call ALife_fnc_applySkins; // the loadout was just (re)applied

["hide"] call ALife_fnc_loading;
titleText [format ["<t size='1.6' font='RobotoCondensedBold'>%1</t>", _name], "PLAIN DOWN", 1.5, true, true];

[] spawn {
    sleep 3; // a short grace period after landing
    player allowDamage true;
};
