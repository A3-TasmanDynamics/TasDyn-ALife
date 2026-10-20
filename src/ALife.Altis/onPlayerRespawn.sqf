// After dying and respawning, go back through the spawn menu.
// (respawnOnStart = -1 in description.ext, so this doesn't run on join.)

if (!hasInterface) exitWith {};

// Arma copies the old unit's variables onto this one: a new life starts
// up, unrestrained, hands down and unfrozen (the server clears the rest,
// ALife_fnc_serverInit's EntityRespawned; jail is the server's to restart).
{ player setVariable [_x select 0, _x select 1, true]; } forEach [
    ["alife_downed", false], ["alife_restrained", false], ["alife_restrainedBy", ""], ["alife_surrendered", false], ["alife_frozen", false]
];

["cameraOff"] call ALife_fnc_deathScreen; // back from the bird's-eye view
if (!isNull (findDisplay 4900)) then { (findDisplay 4900) closeDisplay 2; }; // death screen

// Revived by a medic: straight back where you fell, no spawn menu.
if (!isNil "ALife_revivePending") exitWith {
    player allowDamage false;
    player enableSimulation false;
    [player] remoteExec ["ALife_fnc_completeRevive", 2];
};

[] spawn ALife_fnc_startSession;
