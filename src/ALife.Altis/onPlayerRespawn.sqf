// After dying and respawning, go back through the spawn menu.
// (respawnOnStart = -1 in description.ext, so this doesn't run on join.)

if (!hasInterface) exitWith {};

[] spawn ALife_fnc_startSession;
