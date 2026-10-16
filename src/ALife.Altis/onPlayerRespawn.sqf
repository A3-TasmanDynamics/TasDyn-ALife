// After dying and respawning, go back through the spawn menu.
// (respawnOnStart = -1 in description.ext, so this doesn't run on join.)

if (!hasInterface) exitWith {};

["cameraOff"] call ALife_fnc_deathScreen; // back from the bird's-eye view
if (!isNull (findDisplay 4900)) then { (findDisplay 4900) closeDisplay 2; }; // death screen

[] spawn ALife_fnc_startSession;
