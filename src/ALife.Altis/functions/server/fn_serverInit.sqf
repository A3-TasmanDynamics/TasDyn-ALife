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
    // In jail: the time left is saved and the clock stops until they're back.
    if (!isNil "ALife_jail" && { _uid in ALife_jail }) then {
        private _left = ["remaining", _uid] call ALife_fnc_jail;
        ["save", [_uid, "civ_jail", str (round _left), "-"]] call ALife_fnc_extension;
        ALife_jail deleteAt _uid;
    };
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
    // Died and respawned (not revived): the gear stayed on the body.
    private _uid = getPlayerUID _new;
    private _faction = _old getVariable ["alife_faction", ""];
    if (_uid != "" && { _faction != "" }) then {
        (ALife_sessions getOrDefault [_uid, createHashMap]) set [_faction + "_gear", []];
        ["save", [_uid, _faction + "_gear", "[]", "-"]] call ALife_fnc_extension;
    };
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
            // Current market prices (saved across restarts) before anyone can shop.
            ["init"] call ALife_fnc_market;
            ["loop"] call ALife_fnc_jail; // releases prisoners when their time's up
            // The people you deal with: shop NPCs placed in Eden (CfgShops
            // npcs[]) and a worker at each processor marker (CfgProcessing).
            // They stand still and can't be hurt.
            private _fnc_static = {
                { _this disableAI _x; } forEach ["MOVE", "PATH", "FSM", "TARGET", "AUTOTARGET", "AUTOCOMBAT"];
                _this allowDamage false;
                removeAllWeapons _this;
                _this setUnitPos "UP";
            };
            {
                {
                    private _npc = missionNamespace getVariable [_x, objNull];
                    if (!isNull _npc) then { _npc call _fnc_static; };
                } forEach getArray (_x >> "npcs");
            } forEach ("true" configClasses (missionConfigFile >> "CfgShops"));
            private _workerClass = getText (missionConfigFile >> "CfgProcessing" >> "npcClass");
            {
                private _process = configName _x;
                {
                    if (_x in allMapMarkers) then {
                        private _worker = createAgent [_workerClass, getMarkerPos _x, [], 0, "CAN_COLLIDE"];
                        _worker setDir markerDir _x;
                        _worker call _fnc_static;
                        _worker setVariable ["alife_process", _process, true];
                    };
                } forEach getArray (_x >> "markers");
            } forEach ("true" configClasses (missionConfigFile >> "CfgProcessing"));
        };
        ALife_serverStatus = format ["Waiting for the database: %1", _ping select [6]];
        publicVariable "ALife_serverStatus";
        diag_log format ["[ALife] database not ready (attempt %1): %2", _attempt, _ping];
        sleep 5;
    };

    [] call ALife_fnc_syncLoop;
};
