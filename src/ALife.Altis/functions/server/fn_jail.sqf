/*
    ALife_fnc_jail (server)

    The jail (config\CfgCrimes.hpp >> CfgJail). The server keeps the clock:
    ALife_jail holds uid -> [release time (serverTime), bail]; the time left
    is saved to players.civ_jail (every 30 s, on disconnect via
    ALife_fnc_savePlayer, and on release) so logging out doesn't get anyone
    out early, and a jailed player spawns back in the jail
    (ALife_fnc_requestSpawn).

    Modes:
        ["arrest", officer, prisoner]  - police: a restrained, wanted civilian at the jail
        ["start", unit, seconds, bail] - put / keep them in jail (arrest, rejoin)
        ["bail", unit]                 - the prisoner pays bail (bank) to get out (called by the client)
        ["release", uid]               - time's up (or bail paid)
        ["remaining", uid]             - seconds left (0 = not jailed)
        ["loop"]                       - started once: releases and saves
*/

params [["_mode", "", [""]], ["_a", objNull], ["_b", objNull], ["_c", 0]];

if (!isServer) exitWith {};
if (isNil "ALife_jail") then { ALife_jail = createHashMap; };
private _cfg = missionConfigFile >> "CfgJail";
private _fnc_save = {
    params ["_uid", "_seconds", "_bail"];
    ["save", [_uid, "civ_jail", str (round (_seconds max 0)), "-"]] call ALife_fnc_extension;
    ["save", [_uid, "civ_bail", str (round (_bail max 0)), "-"]] call ALife_fnc_extension;
    private _record = ALife_sessions getOrDefault [_uid, createHashMap];
    _record set ["civ_jail", round (_seconds max 0)];
    _record set ["civ_bail", round (_bail max 0)];
};
private _fnc_unit = {
    private _i = allPlayers findIf { getPlayerUID _x == _this };
    if (_i < 0) then { objNull } else { allPlayers select _i }
};

switch (_mode) do {
    case "remaining": {
        private _entry = ALife_jail getOrDefault [_a, []];
        if (_entry isEqualTo []) exitWith { 0 };
        ((_entry select 0) - serverTime) max 0
    };

    case "start": {
        private _unit = _a;
        private _seconds = _b;
        private _bail = _c;
        if (isNull _unit || { _seconds <= 0 }) exitWith {};
        private _uid = getPlayerUID _unit;
        ALife_jail set [_uid, [serverTime + _seconds, _bail]];
        _unit setVariable ["alife_jailed", true, true];
        [_uid, _seconds, _bail] call _fnc_save;
        ["start", _seconds, _bail, getMarkerPos (getArray (_cfg >> "markers") param [0, ""])] remoteExecCall ["ALife_fnc_jailClient", owner _unit];
        diag_log format ["[ALife] %1 jailed for %2 s, bail $%3", _uid, round _seconds, _bail];
    };

    case "release": {
        private _uid = _a;
        ALife_jail deleteAt _uid;
        [_uid, 0, 0] call _fnc_save;
        private _unit = _uid call _fnc_unit;
        if (!isNull _unit) then {
            _unit setVariable ["alife_jailed", false, true];
            ["release", 0, 0, getMarkerPos getText (_cfg >> "release")] remoteExecCall ["ALife_fnc_jailClient", owner _unit];
        };
        diag_log format ["[ALife] %1 released from jail", _uid];
    };

    case "bail": {
        private _unit = _a;
        if (isNull _unit || { isRemoteExecuted && { remoteExecutedOwner != owner _unit } }) exitWith {};
        private _uid = getPlayerUID _unit;
        private _entry = ALife_jail getOrDefault [_uid, []];
        if (_entry isEqualTo []) exitWith {};
        private _bail = _entry select 1;
        private _reply = { [_this, "warning"] remoteExecCall ["ALife_fnc_notify", owner _unit]; };
        if (_bail <= 0) exitWith { "There's no bail for your sentence: you have to serve it." call _reply; };
        private _record = ALife_sessions getOrDefault [_uid, createHashMap];
        private _paid = [_uid, "civ", _record, _bail, "bank", "Bail"] call ALife_fnc_shopCharge;
        if (_paid != "OK") exitWith { format ["Bail is $%1 from your bank. You don't have enough.", [_bail] call BIS_fnc_numberText] call _reply; };
        diag_log format ["[ALife] %1 paid $%2 bail", _uid, _bail];
        ["release", _uid] call ALife_fnc_jail;
        [format ["You paid $%1 bail. You're free to go.", [_bail] call BIS_fnc_numberText], "success"] remoteExecCall ["ALife_fnc_notify", owner _unit];
        [_unit] call ALife_fnc_requestPhoneData;
    };

    case "arrest": {
        private _cop = _a;
        private _prisoner = _b;
        private _reply = { [_this, "warning"] remoteExecCall ["ALife_fnc_notify", owner _cop]; };
        private _them = _prisoner getVariable ["alife_name", name _prisoner];
        if ((_prisoner getVariable ["alife_faction", ""]) != "civ") exitWith { "Only civilians can be jailed." call _reply; };
        if !(_prisoner getVariable ["alife_restrained", false]) exitWith { format ["Restrain %1 first.", _them] call _reply; };
        if (!([_cop, _cfg] call ALife_fnc_atPlace)) exitWith { "Bring them to the jail to arrest them." call _reply; };
        private _theirUid = getPlayerUID _prisoner;
        private _raw = ["wanted", ["list", _theirUid]] call ALife_fnc_extension;
        if (_raw select [0, 6] == "ERROR:") exitWith { diag_log format ["[ALife] arrest: wanted list for %1 failed: %2", _theirUid, _raw]; "Couldn't read their record. Try again." call _reply; };
        private _crimes = parseSimpleArray _raw;
        if (_crimes isEqualTo []) exitWith { format ["%1 isn't wanted. Add their crimes first (Wanted).", _them] call _reply; };
        private _bounty = 0;
        private _minutes = 0;
        {
            _x params ["", "_key", "_amount"];
            _bounty = _bounty + _amount;
            _minutes = _minutes + getNumber (missionConfigFile >> "CfgCrimes" >> _key >> "jail");
        } forEach _crimes;
        _minutes = (_minutes max getNumber (_cfg >> "minMinutes")) min getNumber (_cfg >> "maxMinutes");
        private _bail = round (_bounty * getNumber (_cfg >> "bailRate"));

        private _cleared = ["wanted", ["clear", _theirUid]] call ALife_fnc_extension;
        if (_cleared select [0, 3] != "OK:") exitWith { diag_log format ["[ALife] arrest: clearing %1 failed: %2", _theirUid, _cleared]; "Couldn't process the arrest. Try again." call _reply; };
        _prisoner setVariable ["alife_bounty", 0, true];
        (ALife_sessions getOrDefault [_theirUid, createHashMap]) set ["civ_bounty", 0];

        // The officer's share of the bounty, in cash.
        private _reward = round (_bounty * getNumber (_cfg >> "copReward"));
        private _copUid = getPlayerUID _cop;
        if (_reward > 0) then {
            private _token = format ["arrest-%1-%2-%3", _copUid, round (diag_tickTime * 1000), floor random 1e6];
            if ((["save", [_copUid, "cop_cash", str _reward, _token]] call ALife_fnc_extension) == "OK") then {
                private _copRecord = ALife_sessions getOrDefault [_copUid, createHashMap];
                _copRecord set ["cop_cash", (_copRecord getOrDefault ["cop_cash", 0]) + _reward];
            } else { _reward = 0; };
        };

        // Out of the cuffs and into the cell.
        if (!isNull attachedTo _prisoner) then { detach _prisoner; };
        _prisoner setVariable ["alife_restrained", false, true];
        _prisoner setVariable ["alife_restrainedBy", "", true];
        ["start", _prisoner, _minutes * 60, _bail] call ALife_fnc_jail;
        diag_log format ["[ALife] %1 arrested %2: %3 crimes, $%4 bounty, %5 min, bail $%6, reward $%7", _copUid, _theirUid, count _crimes, _bounty, _minutes, _bail, _reward];
        [format ["Arrested %1: %2 minutes, bail $%3.%4", _them, _minutes, [_bail] call BIS_fnc_numberText,
            ["", format [" You get $%1 of the bounty.", [_reward] call BIS_fnc_numberText]] select (_reward > 0)], "success"] remoteExecCall ["ALife_fnc_notify", owner _cop];
        [_cop] call ALife_fnc_requestPhoneData;
    };

    case "loop": {
        [] spawn {
            private _lastSave = time;
            while { true } do {
                sleep 5;
                {
                    private _uid = _x;
                    _y params ["_until", "_bail"];
                    if (serverTime >= _until) then { ["release", _uid] call ALife_fnc_jail; };
                } forEach +ALife_jail;
                if (time - _lastSave > 30) then {
                    _lastSave = time;
                    {
                        _y params ["_until", "_bail"];
                        ["save", [_x, "civ_jail", str (round ((_until - serverTime) max 0)), "-"]] call ALife_fnc_extension;
                    } forEach ALife_jail;
                };
            };
        };
    };
};
