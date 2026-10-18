/*
    ALife_fnc_requestSession (server, called by the client)

    A player has joined (or respawned) and wants their character. Loads the
    record, then answers the client with ALife_fnc_sessionReceived:
        ["OK", "", faction, spawnOptions, [name, cash, bank]]
        ["BANNED", message]
        ["ERROR", message]

    The unit stays hidden until they pick a spawn point.

    Arguments:
        0: OBJECT - the player's unit
*/

params [["_unit", objNull, [objNull]]];

if (!isServer || { isNull _unit }) exitWith {};

// Only a player can ask for their own character.
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {
    diag_log format ["[ALife] requestSession refused: client %1 asked for a unit it doesn't own (%2)", remoteExecutedOwner, _unit];
};

private _owner = owner _unit;
private _uid = getPlayerUID _unit;
private _name = name _unit;
private _faction = [side group _unit] call ALife_fnc_sideToFaction;

_unit hideObjectGlobal true;
_unit setVariable ["alife_faction", nil];
_unit setVariable ["alife_uid", _uid];

diag_log format ["[ALife] %1 (%2) requesting session as %3", _name, _uid, _faction];

// Give the database up to a minute if the server has only just started.
private _deadline = diag_tickTime + 60;
waitUntil { sleep 0.5; ALife_serverReady || { diag_tickTime > _deadline } };
if (!ALife_serverReady) exitWith {
    ["ERROR", "The server can't reach the database. " + ALife_serverStatus] remoteExecCall ["ALife_fnc_sessionReceived", _owner];
};

private _record = [_uid] call ALife_fnc_loadRecord;
private _status = _record getOrDefault ["status", "ERROR"];

if (_status == "BANNED") exitWith {
    private _until = _record getOrDefault ["until", ""];
    diag_log format ["[ALife] %1 (%2) is banned -- refused", _name, _uid];
    ["BANNED", format ["You are banned from this server %1.<br/>Reason: %2<br/>You can appeal on the website.",
        if (_until == "") then { "permanently" } else { "until " + _until },
        _record getOrDefault ["reason", "not given"]]] remoteExecCall ["ALife_fnc_sessionReceived", _owner];
};

if (_status != "OK") exitWith {
    diag_log format ["[ALife] %1 (%2) load failed: %3", _name, _uid, _record getOrDefault ["reason", ""]];
    ["ERROR", "Your character couldn't be loaded: " + (_record getOrDefault ["reason", "unknown error"])] remoteExecCall ["ALife_fnc_sessionReceived", _owner];
};

// Whitelisted police slots: past the first copPublicSlots police slots,
// you need cop_level >= copWhitelistLevel.
private _slot = vehicleVarName _unit;
private _slotNumber = parseNumber (_slot select [count "police_"]);
private _publicSlots = getNumber (missionConfigFile >> "CfgALife" >> "copPublicSlots");
private _minLevel = getNumber (missionConfigFile >> "CfgALife" >> "copWhitelistLevel");
private _copLevel = _record getOrDefault ["cop_level", 0];
private _whitelistDenied = _faction == "cop" && { (_slot find "police_") == 0 } && { _slotNumber > _publicSlots } && { _copLevel < _minLevel };
if (_whitelistDenied) exitWith {
    diag_log format ["[ALife] %1 (%2) refused whitelisted slot %3 (cop_level %4, needs %5)", _name, _uid, _slot, _copLevel, _minLevel];
    ["WHITELIST", format ["This is a whitelisted Police slot (level %1+).<br/>Pick one of the Police (Public) slots instead.", _minLevel]]
        remoteExecCall ["ALife_fnc_sessionReceived", _owner];
};

// The player may have changed their in-game name since last time.
if ((_record getOrDefault ["name", ""]) != _name) then {
    ["save", [_uid, "name", _name, "-"]] call ALife_fnc_extension;
    _record set ["name", _name];
};

ALife_sessions set [_uid, _record];

// They're online (player_sessions -- the website's online list). Once per
// connection, not on every respawn.
if (!(_uid in ALife_online)) then {
    private _result = ["session", ["start", _uid]] call ALife_fnc_extension;
    if (_result == "OK") then { ALife_online set [_uid, true]; } else { diag_log format ["[ALife] session start for %1 failed: %2", _uid, _result]; };
};

private _options = [_record, _faction] call ALife_fnc_spawnOptions;
private _info = [_name, _record getOrDefault [_faction + "_cash", 0], _record getOrDefault [_faction + "_bank", 0]];

diag_log format ["[ALife] %1 (%2) loaded as %3 -- %4 spawn option(s)", _name, _uid, _faction, count _options];

["OK", "", _faction, _options, _info] remoteExecCall ["ALife_fnc_sessionReceived", _owner];
