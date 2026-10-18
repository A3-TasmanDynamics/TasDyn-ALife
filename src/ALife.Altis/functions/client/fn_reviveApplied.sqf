/*
    ALife_fnc_reviveApplied (client, called by the server)

    Back on your feet where you fell, with the gear you had, at
    CfgALife >> reviveHealth. An empty position means the revive couldn't be
    completed (e.g. the server restarted) -- you go to the spawn menu
    instead.

    Arguments:
        0: ARRAY  - position (ATL)
        1: ARRAY  - loadout (getUnitLoadout of the body)
        2: STRING - medic's name
*/

params [["_pos", []], ["_loadout", []], ["_medic", ""]];

ALife_revivePending = nil;

if (_pos isEqualTo []) exitWith {
    [] spawn ALife_fnc_startSession;
};

if (_loadout isNotEqualTo []) then {
    player setUnitLoadout _loadout;
    [true] call ALife_fnc_applySkins;
};
player setPosATL _pos;
player enableSimulation true;
player setDamage (1 - (getNumber (missionConfigFile >> "CfgALife" >> "reviveHealth") max 0.05));

call ALife_fnc_setupUnit;
["hide"] call ALife_fnc_loading;
titleText [format ["<t size='1.4' font='RobotoCondensedBold'>Revived by %1</t>", (_medic splitString "<>") joinString ""], "PLAIN DOWN", 1.5, true, true];

[] spawn {
    sleep 3;
    player allowDamage true;
};
