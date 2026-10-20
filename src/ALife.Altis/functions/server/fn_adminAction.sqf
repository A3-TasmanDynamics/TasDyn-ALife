/*
    ALife_fnc_adminAction (server, called by the client: the admin menu)

    Every admin-menu action that touches anyone but the staff member, or
    the database. The staff member's permissions are the ones loaded with
    their character (record "staff_perms": the website's rank permissions
    and overrides, only while they're active staff -- the extension
    resolves them like the website does). Server owners (CfgALife >>
    owners[]) may do everything, including compensating themselves.
    Anything without the permission is refused and logged.

    Logged to the staff log (source 'game', posted to #staff-log): bring,
    heal, freeze / unfreeze, warn, kick, ban, compensate, give item, give
    cash, arsenal, god mode, invisibility, announce, spawn vehicle, delete
    vehicle.

    Arguments:
        0: OBJECT - the staff member's unit
        1: STRING - action (see ALife_fnc_admin)
        2: OBJECT - the player it's done to (objNull for self / server actions)
        3: ARRAY  - the action's arguments (reason, amount, ...)
*/

params [["_unit", objNull, [objNull]], ["_action", "", [""]], ["_target", objNull, [objNull]], ["_args", [], [[]]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _perms = (_record getOrDefault ["staff_perms", []]) select { _x isEqualType "" };
private _me = _unit getVariable ["alife_name", name _unit];
if ((_record getOrDefault ["status", ""]) != "OK") exitWith {};
private _isOwner = _uid in getArray (missionConfigFile >> "CfgALife" >> "owners");

private _needs = switch (_action) do {
    case "bring": { "game.teleport" };
    case "heal": { "game.heal" };
    case "freeze": { "game.freeze" };
    case "message": { "game.admin" };
    case "warn": { "game.warn" };
    case "kick": { "game.kick" };
    case "ban": { "bans.issue" };
    case "compensate": { "players.compensate" };
    case "give";
    case "giveCash";
    case "arsenal";
    case "spawnVehicle": { "game.spawn" };
    case "god";
    case "invisible": { "game.self" };
    case "announce": { "announce.post" };
    case "repair";
    case "refuel";
    case "deleteVeh": { "game.vehicles" };
    default { "" };
};
if (_needs == "" || { !_isOwner && { !(_needs in _perms) } }) exitWith {
    diag_log format ["[ALife] admin: %1 (%2) tried %3 without %4", _uid, _me, _action, _needs];
    "You don't have permission for that." call _fnc_reply;
};

// The player it's done to.
private _theirUid = if (isNull _target) then { "" } else { getPlayerUID _target };
private _them = if (isNull _target) then { "" } else { _target getVariable ["alife_name", name _target] };
private _targeted = _action in ["bring", "heal", "freeze", "message", "warn", "kick", "ban", "compensate", "give", "giveCash"];
if (_targeted && { _theirUid == "" }) exitWith { "That player has left." call _fnc_reply; };
private _reason = _args param [0, "", [""]];
_reason = (_reason splitString "<>") joinString "";
if (count _reason > 300) then { _reason = _reason select [0, 300]; };
// Off the server: they see why for a few seconds, then a real kick
// (serverCommand "#kick" with the server's command password, which the
// Server Manager gives the extension). Without the password, their own
// machine ends the mission instead (ALife_fnc_adminApply "kick").
private _fnc_kick = {
    params ["_who", "_why"];
    ["kick", _me, _why] remoteExecCall ["ALife_fnc_adminApply", owner _who];
    [_who] spawn {
        params ["_who"];
        sleep 5;
        if (isNull _who) exitWith {};
        private _pw = ["server", ["password"]] call ALife_fnc_extension;
        if (_pw select [0, 6] == "ERROR:") exitWith { diag_log "[ALife] kick: no server command password -- the player's own machine ends the mission"; };
        if !(_pw serverCommand format ["#kick %1", getPlayerID _who]) then { diag_log "[ALife] kick: serverCommand #kick was refused"; };
    };
};
// The staff log (and #staff-log).
private _fnc_log = {
    params ["_logAction", ["_logReason", ""], ["_after", ""]];
    private _r = ["admin", ["log", _uid, _theirUid, _logAction, _logReason, _after]] call ALife_fnc_extension;
    if (_r != "OK") then { diag_log format ["[ALife] admin log %1 failed: %2", _logAction, _r]; };
};

switch (_action) do {
    case "bring": {
        ["bring", _me, getPosATL _unit] remoteExecCall ["ALife_fnc_adminApply", owner _target];
        ["game.bring", format ["to %1", mapGridPosition _unit]] call _fnc_log;
        format ["Brought %1 to you.", _them] call _fnc_reply;
    };

    case "heal": {
        if (alive _target) exitWith {
            ["heal", _target, 1, true] remoteExecCall ["ALife_fnc_itemUsed", owner _target];
            ["game.heal"] call _fnc_log;
            [format ["Healed %1.", _them], "success"] call _fnc_reply;
        };
        // Downed: revive them where they fell, like a medic would (no medkit).
        if !(_target getVariable ["alife_downed", false]) exitWith { "They can't be revived." call _fnc_reply; };
        private _patient = owner _target;
        if (_patient <= 2) exitWith { "They've left the server." call _fnc_reply; };
        _target setVariable ["alife_downed", false, true];
        if (isNil "ALife_revives") then { ALife_revives = createHashMap; };
        ALife_revives set [_theirUid, [getPosATL _target, getUnitLoadout _target, _target, "Staff"]];
        ["Staff"] remoteExecCall ["ALife_fnc_revived", _patient];
        ["game.heal", "revived"] call _fnc_log;
        [format ["Revived %1.", _them], "success"] call _fnc_reply;
    };

    case "freeze": {
        private _frozen = !(_target getVariable ["alife_frozen", false]);
        _target setVariable ["alife_frozen", _frozen, true];
        ["freeze", _me, _frozen] remoteExecCall ["ALife_fnc_adminApply", owner _target];
        [["game.unfreeze", "game.freeze"] select _frozen] call _fnc_log;
        [format ["%1 %2.", _them, ["unfrozen", "frozen"] select _frozen], "success"] call _fnc_reply;
    };

    case "message": {
        ["message", _me, _reason] remoteExecCall ["ALife_fnc_adminApply", owner _target];
        [format ["Sent to %1.", _them], "success"] call _fnc_reply;
    };

    case "warn": {
        ["warn", _me, _reason] remoteExecCall ["ALife_fnc_adminApply", owner _target];
        ["game.warn", _reason] call _fnc_log;
        [format ["Warned %1.", _them], "success"] call _fnc_reply;
    };

    case "kick": {
        private _r = ["admin", ["kick", _uid, _theirUid, _reason]] call ALife_fnc_extension;
        if (_r != "OK") exitWith { diag_log format ["[ALife] admin kick log failed: %1", _r]; "The kick wasn't recorded. Try again." call _fnc_reply; };
        [_target, format ["Kicked: %1", _reason]] call _fnc_kick;
        [format ["Kicked %1.", _them], "success"] call _fnc_reply;
    };

    case "ban": {
        private _minutes = _args param [1, 1440, [0]];
        if (!(_minutes in [0, 60, 1440, 4320, 10080, 43200])) exitWith {};
        if (_minutes == 0 && { !_isOwner } && { !("bans.permanent" in _perms) }) exitWith { "Permanent bans need bans.permanent." call _fnc_reply; };
        private _r = ["admin", ["ban", _uid, _theirUid, str _minutes, _reason]] call ALife_fnc_extension;
        if (_r == "ALREADY") exitWith { format ["%1 is already banned.", _them] call _fnc_reply; };
        if (_r != "OK") exitWith { diag_log format ["[ALife] admin ban failed: %1", _r]; "The ban didn't save. Try again." call _fnc_reply; };
        private _length = switch (_minutes) do { case 0: { "permanently" }; case 60: { "for 1 hour" }; case 1440: { "for 1 day" }; default { format ["for %1 days", _minutes / 1440] }; };
        [_target, format ["Banned %1: %2", _length, _reason]] call _fnc_kick;
        [format ["Banned %1 %2.", _them, _length], "success"] call _fnc_reply;
    };

    case "compensate": {
        private _amount = round (_args param [1, 0, [0]]);
        private _account = _args param [2, "civ", [""]];
        if (_amount < 1 || { _amount > 100000000 } || { !(_account in ["civ", "cop", "medic"]) }) exitWith { "Enter an amount ($1 or more)." call _fnc_reply; };
        if (_amount > 10000 && { !_isOwner } && { !("players.compensate_large" in _perms) }) exitWith { "Over $10,000 needs players.compensate_large." call _fnc_reply; };
        if (_theirUid == _uid && { !_isOwner }) exitWith { "You can't compensate yourself." call _fnc_reply; };
        private _r = ["admin", ["compensate", _uid, _theirUid, _account, str _amount, _reason, ["", "self"] select (_theirUid == _uid)]] call ALife_fnc_extension;
        if (_r == "NO_ACCOUNT") exitWith { format ["%1 has no %2 bank account yet.", _them, _account] call _fnc_reply; };
        if (_r select [0, 3] != "OK:") exitWith { diag_log format ["[ALife] admin compensate failed: %1", _r]; "The compensation didn't go through." call _fnc_reply; };
        // Their cached balance follows the ledger.
        private _theirRecord = ALife_sessions getOrDefault [_theirUid, createHashMap];
        _theirRecord set [_account + "_bank", parseNumber (_r select [3])];
        [_target] call ALife_fnc_requestPhoneData;
        [format ["Staff paid $%1 into your %2 bank account: %3", [_amount] call BIS_fnc_numberText, _account, _reason], "success", "Compensation"] remoteExecCall ["ALife_fnc_notify", owner _target];
        [format ["Paid %1 $%2.", _them, [_amount] call BIS_fnc_numberText], "success"] call _fnc_reply;
    };

    case "give": {
        private _class = _args param [0, "", [""]];
        private _qty = round (_args param [1, 1, [0]]);
        if (!isClass (missionConfigFile >> "CfgItems" >> _class) || { _qty < 1 } || { _qty > 100 }) exitWith { "Pick an item and a quantity (1-100)." call _fnc_reply; };
        private _faction = _target getVariable ["alife_faction", ""];
        if (_faction == "" || { !([_theirUid, _faction, _class, _qty] call ALife_fnc_itemsChange) }) exitWith { "Couldn't give that (are they spawned?)." call _fnc_reply; };
        private _name = getText (missionConfigFile >> "CfgItems" >> _class >> "displayName");
        ["game.give_item", format ["%1 x%2", _name, _qty], format ['{"item": "%1", "qty": %2}', _class, _qty]] call _fnc_log;
        [_target] call ALife_fnc_requestPhoneData;
        [format ["Staff gave you %1 x%2.", _name, _qty], "success"] remoteExecCall ["ALife_fnc_notify", owner _target];
        [format ["Gave %1 %2 x%3.", _them, _name, _qty], "success"] call _fnc_reply;
    };

    case "giveCash": {
        // Cash in hand, for the faction they're playing (a delta save with
        // a one-off token, like any other cash change).
        private _amount = round (_args param [1, 0, [0]]);
        if (_amount < 1 || { _amount > 1000000 }) exitWith { "Enter an amount ($1 - $1,000,000)." call _fnc_reply; };
        private _faction = _target getVariable ["alife_faction", ""];
        if (_faction == "") exitWith { "They need to be spawned in." call _fnc_reply; };
        private _token = format ["admincash-%1-%2-%3", _theirUid, round (diag_tickTime * 1000), floor random 1e6];
        private _r = ["save", [_theirUid, _faction + "_cash", str _amount, _token]] call ALife_fnc_extension;
        if (_r != "OK") exitWith { diag_log format ["[ALife] admin give cash failed: %1", _r]; "That didn't save. Try again." call _fnc_reply; };
        private _theirRecord = ALife_sessions getOrDefault [_theirUid, createHashMap];
        _theirRecord set [_faction + "_cash", (_theirRecord getOrDefault [_faction + "_cash", 0]) + _amount];
        ["game.give_cash", format ["$%1 %2 cash%3", [_amount] call BIS_fnc_numberText, _faction, ["", ": " + _reason] select (_reason != "")],
            format ['{"faction": "%1", "amount": %2}', _faction, _amount]] call _fnc_log;
        [_target] call ALife_fnc_requestPhoneData;
        if (_target != _unit) then { [format ["Staff gave you $%1 cash.", [_amount] call BIS_fnc_numberText], "success"] remoteExecCall ["ALife_fnc_notify", owner _target]; };
        [format ["Gave %1 $%2 cash.", _them, [_amount] call BIS_fnc_numberText], "success"] call _fnc_reply;
    };

    case "arsenal": { ["game.arsenal"] call _fnc_log; };

    case "god": { ["game.godmode"] call _fnc_log; };

    case "invisible": {
        private _hide = !isObjectHidden _unit;
        _unit hideObjectGlobal _hide;
        if (_hide) then { ["game.invisible"] call _fnc_log; };
        [format ["Invisible %1.", ["off", "on"] select _hide], "info"] call _fnc_reply;
    };

    case "announce": {
        ["announce", _me, _reason] remoteExecCall ["ALife_fnc_adminApply", 0];
        ["game.announce", _reason] call _fnc_log;
    };

    case "spawnVehicle": {
        private _class = _args param [0, "", [""]];
        private _cfg = configFile >> "CfgVehicles" >> _class;
        if (!isClass _cfg || { getNumber (_cfg >> "scope") < 2 } || { !(_class isKindOf "LandVehicle" || { _class isKindOf "Air" } || { _class isKindOf "Ship" }) }) exitWith {
            format ["%1 isn't a vehicle class.", _class] call _fnc_reply;
        };
        private _pos = _unit modelToWorld [0, 8, 0];
        private _vehicle = createVehicle [_class, [_pos select 0, _pos select 1, 0], [], 0, "NONE"];
        _vehicle setDir getDir _unit;
        ["game.spawn_vehicle", getText (_cfg >> "displayName"), format ['{"class": "%1"}', _class]] call _fnc_log;
        [format ["Spawned a %1.", getText (_cfg >> "displayName")], "success"] call _fnc_reply;
    };

    case "repair";
    case "refuel";
    case "deleteVeh": {
        private _vehicle = _args param [0, objNull, [objNull]];
        if (isNull _vehicle || { _unit distance _vehicle > 50 }) exitWith { "Get closer to the vehicle." call _fnc_reply; };
        private _name = getText (configFile >> "CfgVehicles" >> typeOf _vehicle >> "displayName");
        switch (_action) do {
            case "repair": { _vehicle setDamage 0; [format ["Repaired the %1.", _name], "success"] call _fnc_reply; };
            case "refuel": {
                if (local _vehicle) then { _vehicle setFuel 1; } else { ["fuel", _vehicle, 1] remoteExecCall ["ALife_fnc_itemUsed", _vehicle]; };
                [format ["Refuelled the %1.", _name], "success"] call _fnc_reply;
            };
            case "deleteVeh": {
                if ((crew _vehicle) findIf { isPlayer _x } >= 0) exitWith { "Someone's in it." call _fnc_reply; };
                ["game.vehicle_delete", format ["%1 %2", _name, _vehicle getVariable ["alife_plate", ""]],
                    format ['{"class": "%1", "owner": "%2"}', typeOf _vehicle, _vehicle getVariable ["alife_owner", ""]]] call _fnc_log;
                deleteVehicle _vehicle;
                [format ["Deleted the %1.", _name], "success"] call _fnc_reply;
            };
        };
    };
};
