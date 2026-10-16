/*
    ALife_fnc_revived (downed player's client, called by the server)

    A medic revived you. Leaves the death screen and respawns straight away;
    onPlayerRespawn.sqf then asks the server to put you back where you fell
    (ALife_fnc_completeRevive -> ALife_fnc_reviveApplied).

    Arguments:
        0: STRING - medic's name
*/

params [["_medic", "", [""]]];

if (alive player) exitWith {};

ALife_revivePending = _medic;
ALife_respawnRequested = true; // stops the death screen reopening
closeDialog 0;
["show", format ["%1 is reviving you...", (_medic splitString "<>") joinString ""]] call ALife_fnc_loading;
setPlayerRespawnTime 0;
