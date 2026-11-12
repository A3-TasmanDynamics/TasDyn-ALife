/*
    ALife_fnc_anticheat (server)

    Logs every economy action to the player's event log (player_log, shown
    on the website's player history) and flags what isn't normal for staff
    to review (anti_cheat_flags, posted to #staff-log): docs/ANTI_CHEAT.md.
    Settings: CfgALife >> AntiCheat. Nothing here kicks or bans -- staff
    decide, from the flag.

    Flags:
        rate_limit       - the same action fired faster than a person can
                           (e.g. several sells within milliseconds); past the
                           limit the extra requests are refused
        invalid_request  - a request sent for someone else's character (high)
        movement         - moved faster than anything they're in can go, or
                           teleported, outside a server teleport (spawn,
                           jail, revive, admin)
        cheat_state      - god mode, invisible or sped up, and not staff with
                           those powers
        gear             - carrying gear no shop or starting kit gives
        dupe             - a pile of the same weapon / attachment (CfgALife >>
                           AntiCheat >> dupeCount or more)
    Each flag is raised once per player and kind (and item) per
    flagCooldown seconds, so one cheater doesn't flood the panel.

    Staff exemptions (the website's permissions, or CfgALife >> owners[]):
    game.teleport -- movement; game.self -- god mode / invisible;
    game.spawn -- gear (they have the arsenal).

    Modes:
        ["start"]                                   - (serverInit) the checks loop
        ["log", unit|uid, event, [[key, value], ...]]
        ["flag", unit|uid, type, "high"|"medium", [[key, value], ...](, dedupKey)]
        ["rate", unit, action]                      - BOOL: true = refuse it
        ["spoof", unit, function, callerOwner]      - a request for someone else
        ["grace", unit(, seconds)]                  - a server teleport is coming

    Returns:
        BOOL for "rate"; nothing otherwise
*/

params [["_mode", "", [""]], ["_who", objNull, [objNull, ""]], ["_a", ""], ["_b", ""], ["_c", []], ["_d", ""]];

if (!isServer) exitWith { false };
private _cfg = missionConfigFile >> "CfgALife" >> "AntiCheat";
private _uidOf = { if (_this isEqualType "") then { _this } else { getPlayerUID _this } };
private _json = { toJSON createHashMapFromArray (_this select { _x isEqualType [] && { count _x == 2 } }) };
// Staff with a permission (or a server owner).
private _staff = {
    params ["_unit", "_perm"];
    private _uid = getPlayerUID _unit;
    if (_uid in getArray (missionConfigFile >> "CfgALife" >> "owners")) exitWith { true };
    _perm in ((((missionNamespace getVariable ["ALife_sessions", createHashMap]) getOrDefault [_uid, createHashMap]) getOrDefault ["staff_perms", []]) select { _x isEqualType "" })
};

switch (_mode) do {
    case "log": {
        private _uid = _who call _uidOf;
        if (_uid == "") exitWith {};
        private _r = ["ac", ["log", _uid, _a, _b call _json]] call ALife_fnc_extension;
        if (_r != "OK") then { diag_log format ["[ALife][AC] log %1 for %2 failed: %3", _a, _uid, _r]; };
    };

    case "flag": {
        private _uid = _who call _uidOf;
        if (_uid == "") exitWith {};
        // Once per player, kind and key per cooldown.
        if (isNil "ALife_acFlagged") then { ALife_acFlagged = createHashMap; };
        private _key = format ["%1|%2|%3", _uid, _a, _d];
        if (time - (ALife_acFlagged getOrDefault [_key, -1e9]) < getNumber (_cfg >> "flagCooldown")) exitWith {};
        ALife_acFlagged set [_key, time];
        private _details = _c + [["name", if (_who isEqualType objNull) then { name _who } else { "" }]];
        if (_who isEqualType objNull && { !isNull _who }) then { _details pushBack ["position", (getPosATL _who) apply { round _x }]; };
        diag_log format ["[ALife][AC] FLAG %1 (%2) %3: %4", _uid, _b, _a, _details];
        private _r = ["ac", ["flag", _uid, _a, _b, _details call _json]] call ALife_fnc_extension;
        if (_r != "OK") then { diag_log format ["[ALife][AC] flag %1 for %2 failed: %3", _a, _uid, _r]; };
    };

    case "rate": {
        // Requests of one action, per player, in the last rateWindow seconds.
        if (isNull _who) exitWith { false };
        if (isNil "ALife_acRate") then { ALife_acRate = createHashMap; };
        private _key = (getPlayerUID _who) + "|" + _a;
        private _now = diag_tickTime;
        private _window = getNumber (_cfg >> "rateWindow");
        private _times = (ALife_acRate getOrDefault [_key, []]) select { _now - _x < _window };
        private _gap = if (_times isEqualTo []) then { 1e9 } else { _now - (_times select -1) };
        _times pushBack _now;
        ALife_acRate set [_key, _times];
        // Faster than a person can click: flagged.
        if (_gap < getNumber (_cfg >> "burstGap")) then {
            ["flag", _who, "rate_limit", "medium", [["action", _a], ["gap_ms", round (_gap * 1000)], ["kind", "burst"]], _a + "|burst"] call ALife_fnc_anticheat;
        };
        // Past the limit: flagged and refused.
        if (count _times > getNumber (_cfg >> "rateMax")) exitWith {
            ["flag", _who, "rate_limit", "medium", [["action", _a], ["requests", count _times], ["window_s", _window], ["kind", "flood"]], _a + "|flood"] call ALife_fnc_anticheat;
            ["log", _who, "anticheat.refused", [["action", _a], ["requests", count _times]]] call ALife_fnc_anticheat;
            ["Slow down.", "warning"] remoteExecCall ["ALife_fnc_notify", owner _who];
            true
        };
        false
    };

    case "spoof": {
        // _who = the character the request was for, _a = the function, _b = who really sent it.
        private _sender = allPlayers select { owner _x == _b };
        private _target = if (_sender isEqualTo []) then { getPlayerUID _who } else { getPlayerUID (_sender select 0) };
        if (_target == "") exitWith { diag_log format ["[ALife][AC] request for %1 to %2 from unknown owner %3", name _who, _a, _b]; };
        ["flag", _target, "invalid_request", "high", [["function", _a], ["for", getPlayerUID _who], ["for_name", name _who]], _a] call ALife_fnc_anticheat;
    };

    case "grace": {
        if (isNull _who) exitWith {};
        _who setVariable ["alife_acGrace", time + ([_a, getNumber (_cfg >> "grace")] select (_a isEqualType ""))];
        if (!isNil "ALife_acPos") then { ALife_acPos deleteAt (getPlayerUID _who); };
    };

    case "start": {
        [] spawn {
            private _cfg = missionConfigFile >> "CfgALife" >> "AntiCheat";
            private _every = getNumber (_cfg >> "checkEvery") max 2;
            ALife_acPos = createHashMap;     // uid -> [position 2D, time]
            ALife_acAllowed = createHashMap; // lower-case classes gear can legitimately be
            ALife_acTypes = [];              // unit classes whose starting kit is in it
            // Gear any shop sells, and what comes on / with it.
            private _allow = {
                private _c = toLower _this;
                if (_c == "" || { _c in ALife_acAllowed }) exitWith {};
                ALife_acAllowed set [_c, true];
                private _w = configFile >> "CfgWeapons" >> _this;
                if (isClass _w) then {
                    private _base = getText (_w >> "baseWeapon");
                    if (_base != "") then { _base call _allow; };
                    { (getText (_x >> "item")) call _allow; } forEach ("true" configClasses (_w >> "LinkedItems"));
                };
            };
            {
                private _shop = _x;
                { { (_x param [0, ""]) call _allow; } forEach getArray (_shop >> _x); } forEach ["items", "mags", "accs"];
            } forEach ("true" configClasses (missionConfigFile >> "WeaponShops"));
            {
                private _shop = _x;
                { { (_x param [0, ""]) call _allow; } forEach getArray (_shop >> _x); } forEach ["uniforms", "headgear", "goggles", "vests", "backpacks"];
            } forEach ("true" configClasses (missionConfigFile >> "ClothingShops"));
            { _x call _allow; } forEach getArray (_cfg >> "alwaysAllowed");
            // Every class name in a loadout (getUnitLoadout), nested.
            private _classes = {
                private _out = [];
                private _walk = {
                    {
                        if (_x isEqualType "") then { if (_x != "") then { _out pushBack _x; }; } else {
                            if (_x isEqualType []) then { _x call _walk; };
                        };
                    } forEach _this;
                };
                _this call _walk;
                _out select { isClass (configFile >> "CfgWeapons" >> _x) || { isClass (configFile >> "CfgMagazines" >> _x) } || { isClass (configFile >> "CfgVehicles" >> _x) } || { isClass (configFile >> "CfgGlasses" >> _x) } }
            };
            private _tick = 0;
            while { true } do {
                sleep _every;
                _tick = _tick + 1;
                private _gearTurn = _tick % ((round (getNumber (_cfg >> "gearEvery") / _every)) max 1) == 0;
                {
                    private _unit = _x;
                    private _uid = getPlayerUID _unit;
                    private _faction = _unit getVariable ["alife_faction", ""];
                    // Spawned, alive, up, not just teleported by the server.
                    if (_uid != "" && { _faction != "" } && { alive _unit } && { !(_unit getVariable ["alife_downed", false]) }
                        && { time > (_unit getVariable ["alife_acGrace", 0]) }) then {
                        // ---- Movement ----------------------------------------------
                        private _pos = (getPosATL _unit) select [0, 2];
                        private _last = ALife_acPos getOrDefault [_uid, []];
                        ALife_acPos set [_uid, [_pos, time]];
                        // (Not while escorted, or jailed -- the cell pulls them back.)
                        if (_last isNotEqualTo [] && { isNull attachedTo _unit } && { !(_unit getVariable ["alife_jailed", false]) } && { !([_unit, "game.teleport"] call _staff) }) then {
                            _last params ["_from", "_at"];
                            private _dt = (time - _at) max 0.5;
                            private _dist = _from distance2D _pos;
                            private _veh = vehicle _unit;
                            private _limit = if (_veh == _unit) then { getNumber (_cfg >> "footSpeed") } else {
                                (getNumber (configFile >> "CfgVehicles" >> typeOf _veh >> "maxSpeed") / 3.6) * getNumber (_cfg >> "vehicleFactor") + 10
                            };
                            if (_dist / _dt > _limit && { _dist > 25 }) then {
                                ["flag", _unit, "movement", "medium", [
                                    ["kind", ["too fast", "teleport"] select (_dist > getNumber (_cfg >> "teleportDistance"))],
                                    ["from", _from apply { round _x }], ["to", _pos apply { round _x }], ["metres", round _dist], ["seconds", _dt toFixed 1],
                                    ["speed_ms", round (_dist / _dt)], ["limit_ms", round _limit], ["vehicle", [typeOf _veh, ""] select (_veh == _unit)]
                                ], "move"] call ALife_fnc_anticheat;
                            };
                        };
                        // ---- God mode / invisible / sped up --------------------------
                        if (!([_unit, "game.self"] call _staff)) then {
                            private _what = [];
                            if (!isDamageAllowed _unit) then { _what pushBack "god mode"; };
                            if (isObjectHidden _unit) then { _what pushBack "invisible"; };
                            if (getAnimSpeedCoef _unit > 1.15) then { _what pushBack format ["speed x%1", (getAnimSpeedCoef _unit) toFixed 2]; };
                            if (_what isNotEqualTo []) then {
                                ["flag", _unit, "cheat_state", "medium", [["state", _what joinString ", "]], _what joinString ","] call ALife_fnc_anticheat;
                            };
                        };
                        // ---- Gear: where it could have come from, and piles ----------
                        if (_gearTurn && { !([_unit, "game.spawn"] call _staff) }) then {
                            // Their unit class's starting kit is legitimate too.
                            if !((typeOf _unit) in ALife_acTypes) then {
                                ALife_acTypes pushBack typeOf _unit;
                                { _x call _allow; } forEach ((getUnitLoadout (typeOf _unit)) call _classes);
                            };
                            private _all = (getUnitLoadout _unit) call _classes;
                            private _unknown = [];
                            { if !(toLower _x in ALife_acAllowed) then { _unknown pushBackUnique _x; }; } forEach _all;
                            if (_unknown isNotEqualTo []) then {
                                ["flag", _unit, "gear", "medium", [["items", _unknown], ["faction", _faction]], _unknown joinString ","] call ALife_fnc_anticheat;
                            };
                            // Piles: N or more of one weapon / attachment / piece of gear.
                            private _counts = createHashMap;
                            { if (isClass (configFile >> "CfgWeapons" >> _x)) then { _counts set [_x, (_counts getOrDefault [_x, 0]) + 1]; }; } forEach _all;
                            private _piles = (keys _counts) select { (_counts get _x) >= getNumber (_cfg >> "dupeCount") };
                            if (_piles isNotEqualTo []) then {
                                ["flag", _unit, "dupe", "medium", [["items", _piles apply { [_x, _counts get _x] }]], _piles joinString ","] call ALife_fnc_anticheat;
                            };
                        };
                    };
                } forEach allPlayers;
                // Forget players who've left.
                { if !(_x in (allPlayers apply { getPlayerUID _x })) then { ALife_acPos deleteAt _x; }; } forEach keys ALife_acPos;
            };
        };
    };
};
