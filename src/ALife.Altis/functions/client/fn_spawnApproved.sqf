/*
    ALife_fnc_spawnApproved (client, called by the server)

    The server's answer to ALife_fnc_requestSpawn: move here and go, or
    (empty position) an error and back to the spawn menu.

    Arguments:
        0: ARRAY  - position (ATL), or [] if refused
        1: STRING - spawn point name
        2: STRING - error message when refused
        3: ARRAY  - saved loadout to put on ([] = none: the slot's kit
                    without weapons); a GPS is added (GroupMarkers spawnGps)
        4: ARRAY  - [food, water] (0-100) to start with (ALife_fnc_survival)
*/

params [["_pos", []], ["_name", ""], ["_error", ""], ["_loadout", []], ["_survival", [100, 100]]];

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
// Everyone spawns with a GPS (it shows your police / EMS / gang:
// ALife_fnc_groupMarkers), unless CfgALife >> GroupMarkers >> spawnGps is 0.
if (getNumber (missionConfigFile >> "CfgALife" >> "GroupMarkers" >> "spawnGps") == 1 && { !("ItemGPS" in assignedItems player) }) then {
    player linkItem "ItemGPS";
};

player setPosATL _pos;
// Arma's First Aid Kits / Medikits aren't used here (vItems instead).
{ player removeItems _x; } forEach ["FirstAidKit", "Medikit"];
ALife_food = _survival param [0, 100];
ALife_water = _survival param [1, 100];
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
