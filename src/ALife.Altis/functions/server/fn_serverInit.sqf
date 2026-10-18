/*
    ALife_fnc_serverInit (server)

    Runs once from initServer.sqf:
      - waits for the database (retrying), then tells clients the server is
        ready (ALife_serverReady / ALife_serverStatus are public, and late
        joiners receive them automatically)
      - saves a player's alive/position when they disconnect
      - marks a player dead in the database when they die
      - starts the periodic sync (ping + autosave)
      - logs FPS for the server manager's performance graph
*/

if (!isServer) exitWith {};

ALife_sessions = createHashMap; // uid -> loaded record (server only, never broadcast)
ALife_revives = createHashMap;  // uid -> pending revive [position, loadout, body, medic name]
ALife_online = createHashMap;   // uid -> true while connected (player_sessions row open)

ALife_serverReady = false;
publicVariable "ALife_serverReady";
ALife_serverStatus = "Connecting to the database...";
publicVariable "ALife_serverStatus";

// "[ALife][FPS] <n>" is read by the server manager (serverprocess.go).
[] spawn {
    while { true } do {
        diag_log format ["[ALife][FPS] %1", diag_fps];
        sleep 2;
    };
};

addMissionEventHandler ["HandleDisconnect", {
    params ["_unit", "_id", "_uid", "_name"];
    diag_log format ["[ALife] %1 (%2) disconnected", _name, _uid];
    // Close their session (the website's online list / last seen).
    if (_uid in ALife_online) then {
        ["session", ["end", _uid]] call ALife_fnc_extension;
        ALife_online deleteAt _uid;
    };
    _unit setVariable ["alife_downed", false, true]; // a body nobody's coming back to
    ALife_revives deleteAt _uid;
    // Leaving while incapacitated (on the death screen) is combat logging --
    // against the rules. Recorded as an anti-cheat flag + staff log entry,
    // which the website shows and posts to Discord #staff-log.
    if (!alive _unit && { !isNil { _unit getVariable "alife_downedAt" } }) then {
        (_unit getVariable ["alife_downedBy", ["", "", -1]]) params ["_by", "_weapon", "_distance"];
        private _faction = _unit getVariable ["alife_downedFaction", "civ"];
        private _seconds = round (time - (_unit getVariable "alife_downedAt"));
        private _result = ["combatlog", [_uid, _faction, _by, _weapon,
            ["", str _distance] select (_distance >= 0), str _seconds]] call ALife_fnc_extension;
        diag_log format ["[ALife][COMBAT LOG] %1 (%2) disconnected %3 s after going down -- recorded: %4",
            _name, _uid, _seconds, _result];
    };
    [_unit, _uid] call ALife_fnc_savePlayer;
    ALife_sessions deleteAt _uid;
    false // don't hand the body to AI
}];

addMissionEventHandler ["EntityKilled", {
    params ["_unit", "_killer", "_instigator"];
    private _faction = _unit getVariable ["alife_faction", ""];
    private _uid = _unit getVariable ["alife_uid", ""];
    if (_faction == "" || { _uid == "" }) exitWith {};

    // Dead: next join starts from the spawn menu, not "last position".
    _unit setVariable ["alife_faction", nil];
    private _result = ["save", [_uid, _faction + "_alive", "false", "-"]] call ALife_fnc_extension;
    ([_unit, _killer, _instigator] call ALife_fnc_killInfo) params ["_by", "_weapon", "_distance"];
    // Medics can revive the body until the player respawns or leaves (public).
    _unit setVariable ["alife_downed", true, true];
    // Kept on the body in case they disconnect before respawning (combat logging).
    _unit setVariable ["alife_downedAt", time];
    _unit setVariable ["alife_downedFaction", _faction];
    _unit setVariable ["alife_downedBy", [_by, _weapon, _distance]];
    diag_log format ["[ALife] %1 (%2) died as %3 -- killed by %4 with %5 at %6 m -- saved: %7",
        name _unit, _uid, _faction, ["nobody", _by] select (_by != ""), _weapon, _distance, _result];

    private _record = ALife_sessions getOrDefault [_uid, createHashMap];
    _record set [_faction + "_alive", false];
}];

// Respawning instead of being revived: the old body can't be revived any more.
addMissionEventHandler ["EntityRespawned", {
    params ["_new", "_old"];
    if (!isNull _old) then { _old setVariable ["alife_downed", false, true]; };
}];

// Wait for the database, then open the doors.
[] spawn {
    private _attempt = 0;
    while { true } do {
        _attempt = _attempt + 1;
        private _ping = ["ping"] call ALife_fnc_extension;
        if (_ping == "OK") exitWith {
            ALife_serverStatus = "Ready.";
            publicVariable "ALife_serverStatus";
            ALife_serverReady = true;
            publicVariable "ALife_serverReady";
            diag_log format ["[ALife] database connected (attempt %1) -- server ready", _attempt];
            // Nobody is connected yet: close sessions left open by the last run (e.g. a crash).
            ["session", ["reset"]] call ALife_fnc_extension;
        };
        ALife_serverStatus = format ["Waiting for the database: %1", _ping select [6]];
        publicVariable "ALife_serverStatus";
        diag_log format ["[ALife] database not ready (attempt %1): %2", _attempt, _ping];
        sleep 5;
    };

    [] call ALife_fnc_syncLoop;
};
