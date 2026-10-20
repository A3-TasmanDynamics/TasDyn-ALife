/*
    ALife_fnc_personAction (server, called by the client: the person menu)

    Things players do to each other (ALife_fnc_personMenu), checked here.
    The other player must be alive, a player, and within 4 m.

        "restrain"    - police, or anyone using up a Zip Ties; the player is
                        held (alife_restrained, alife_restrainedBy "cop" /
                        "zipties") until released, and can't use their phone,
                        inventory or a vehicle's wheel (ALife_fnc_restrained)
        "unrestrain"  - police release anyone; anyone unties a zip-tied player
        "fine"        - police: offer a fine (one of CfgALife >> fines[]); the
                        player pays (cash / bank) or refuses
                        (ALife_fnc_fine), within fineTimeout seconds
        "finePay"     - the fined player pays: 3 = "cash" / "bank"
        "fineRefuse"  - the fined player refuses
        "escort"      - police: a restrained player on foot walks in front of
                        them (attached) until "stopEscort", release or either
                        of them going down
        "stopEscort"  - police: let go of them
        "putIn"       - police: a restrained player into the back of the
                        nearest vehicle with a free seat (within 8 m)
        "takeOut"     - police: a restrained player out of a vehicle
        "wanted"      - police: add a crime (3 = CfgCrimes class) to a
                        civilian's wanted list
        "arrest"      - police: a restrained, wanted civilian at the jail
                        (ALife_fnc_jail "arrest")
        "strip"       - police: a restrained player loses their weapons,
                        vest, backpack, hat, glasses and what's in their
                        uniform (seized; ALife_fnc_personApply)
        "heal"        - medics: heals a hurt player fully, using a Medkit

    Arguments:
        0: OBJECT - the player doing it
        1: STRING - action
        2: OBJECT - the other player (objNull for finePay / fineRefuse)
        3: ANY    - fine: amount; finePay: "cash" / "bank"
*/

params [["_unit", objNull, [objNull]], ["_action", "", [""]], ["_target", objNull, [objNull]], ["_arg", 0, [0, ""]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _fnc_tell = { params ["_who", "_text", ["_type", "info"]]; [_text, _type] remoteExecCall ["ALife_fnc_notify", owner _who]; };
private _uid = getPlayerUID _unit;
private _faction = _unit getVariable ["alife_faction", ""];
private _me = _unit getVariable ["alife_name", name _unit];
if (!alive _unit || { _faction == "" }) exitWith {};
if (isNil "ALife_tickets") then { ALife_tickets = createHashMap; }; // fined uid -> [cop uid, cop name, amount, expires]

// The fined player answering: no other player involved.
if (_action in ["finePay", "fineRefuse"]) exitWith {
    private _ticket = ALife_tickets getOrDefault [_uid, []];
    if (_ticket isEqualTo [] || { diag_tickTime > (_ticket select 3) }) exitWith {
        ALife_tickets deleteAt _uid;
        "That fine has expired." call _fnc_reply;
    };
    _ticket params ["_copUid", "_copName", "_amount"];
    private _copIndex = allPlayers findIf { getPlayerUID _x == _copUid };
    private _cop = if (_copIndex < 0) then { objNull } else { allPlayers select _copIndex };
    if (_action == "fineRefuse") exitWith {
        ALife_tickets deleteAt _uid;
        diag_log format ["[ALife] %1 refused a $%2 fine from %3", _uid, _amount, _copUid];
        "You refused the fine." call _fnc_reply;
        if (!isNull _cop) then { [_cop, format ["%1 refused to pay the $%2 fine.", _me, [_amount] call BIS_fnc_numberText], "warning"] call _fnc_tell; };
    };
    if !(_arg in ["cash", "bank"]) exitWith {};
    private _record = ALife_sessions getOrDefault [_uid, createHashMap];
    private _paid = [_uid, _faction, _record, _amount, _arg, "Fine from " + _copName] call ALife_fnc_shopCharge;
    if (_paid != "OK") exitWith {
        format ["Not enough %1 to pay the $%2 fine.", ["cash", "money in the bank"] select (_arg == "bank"), [_amount] call BIS_fnc_numberText] call _fnc_reply;
    };
    ALife_tickets deleteAt _uid;
    diag_log format ["[ALife] %1 paid a $%2 fine (%3) from %4", _uid, _amount, _arg, _copUid];
    [format ["You paid the $%1 fine.", [_amount] call BIS_fnc_numberText], "success"] call _fnc_reply;
    if (!isNull _cop) then { [_cop, format ["%1 paid the $%2 fine.", _me, [_amount] call BIS_fnc_numberText], "success"] call _fnc_tell; };
    [_unit] call ALife_fnc_requestPhoneData;
};

if (isNull _target || { !isPlayer _target } || { _target == _unit }) exitWith {};
private _them = _target getVariable ["alife_name", name _target];
if (!alive _target) exitWith { format ["%1 is down.", _them] call _fnc_reply; };
if (_unit distance (vehicle _target) > ([4, 8] select (vehicle _target != _target))) exitWith { format ["Get closer to %1.", _them] call _fnc_reply; };
private _restrained = _target getVariable ["alife_restrained", false];

switch (_action) do {
    case "restrain": {
        if (_restrained) exitWith { format ["%1 is already restrained.", _them] call _fnc_reply; };
        if (vehicle _target != _target) exitWith { format ["Get %1 out of the vehicle first.", _them] call _fnc_reply; };
        private _by = "cop";
        if (_faction != "cop") then {
            _by = "zipties";
            if (!([_uid, _faction, "zipties", -1] call ALife_fnc_itemsChange)) then { _by = ""; };
        };
        if (_by == "") exitWith { "You need Zip Ties." call _fnc_reply; };
        _target setVariable ["alife_restrained", true, true];
        _target setVariable ["alife_restrainedBy", _by, true];
        ["start", _me] remoteExecCall ["ALife_fnc_restrained", owner _target];
        diag_log format ["[ALife] %1 (%2) restrained %3 (%4)", _uid, _faction, getPlayerUID _target, _by];
        [format ["You restrained %1.", _them], "success"] call _fnc_reply;
        if (_by == "zipties") then { [_unit] call ALife_fnc_requestPhoneData; };
    };
    case "unrestrain": {
        if (!_restrained) exitWith {};
        if (_faction != "cop" && { (_target getVariable ["alife_restrainedBy", ""]) != "zipties" }) exitWith { "Only the police can take handcuffs off." call _fnc_reply; };
        _target setVariable ["alife_restrained", false, true];
        _target setVariable ["alife_restrainedBy", "", true];
        if (!isNull attachedTo _target) then { detach _target; };
        diag_log format ["[ALife] %1 released %2", _uid, getPlayerUID _target];
        [format ["You released %1.", _them], "success"] call _fnc_reply;
        [_target, format ["%1 released you.", _me], "success"] call _fnc_tell;
    };
    case "wanted": {
        if (_faction != "cop") exitWith {};
        private _crime = missionConfigFile >> "CfgCrimes" >> (["", _arg] select (_arg isEqualType ""));
        if (!isClass _crime) exitWith {};
        if ((_target getVariable ["alife_faction", ""]) != "civ") exitWith { "Only civilians go on the wanted list." call _fnc_reply; };
        private _bounty = getNumber (_crime >> "bounty");
        private _theirUid = getPlayerUID _target;
        private _r = ["wanted", ["add", _theirUid, configName _crime, str _bounty, _uid]] call ALife_fnc_extension;
        if (_r select [0, 3] != "OK:") exitWith { diag_log format ["[ALife] wanted add failed: %1", _r]; "That didn't save. Try again." call _fnc_reply; };
        private _record = ALife_sessions getOrDefault [_theirUid, createHashMap];
        private _total = (_record getOrDefault ["civ_bounty", 0]) + _bounty;
        _record set ["civ_bounty", _total];
        _target setVariable ["alife_bounty", _total, true];
        diag_log format ["[ALife] %1 added %2 ($%3) to %4's wanted list", _uid, configName _crime, _bounty, _theirUid];
        [format ["%1: %2 added. Wanted for $%3.", _them, getText (_crime >> "name"), [_total] call BIS_fnc_numberText], "success"] call _fnc_reply;
        [_target, format ["The police added %1 to your record ($%2 bounty).", getText (_crime >> "name"), [_bounty] call BIS_fnc_numberText], "warning"] call _fnc_tell;
    };
    case "arrest": {
        if (_faction != "cop") exitWith {};
        ["arrest", _unit, _target] call ALife_fnc_jail;
    };
    case "escort": {
        if (_faction != "cop") exitWith {};
        if (!_restrained) exitWith { format ["Restrain %1 first.", _them] call _fnc_reply; };
        if (vehicle _target != _target) exitWith { format ["Take %1 out of the vehicle first.", _them] call _fnc_reply; };
        if (!isNull attachedTo _target) exitWith { format ["Someone's already escorting %1.", _them] call _fnc_reply; };
        _target attachTo [_unit, [0, 1.1, 0]];
        [format ["You're escorting %1.", _them], "success"] call _fnc_reply;
        [_target, format ["%1 is escorting you.", _me], "warning"] call _fnc_tell;
    };
    case "stopEscort": {
        if (attachedTo _target != _unit) exitWith {};
        detach _target;
        [format ["You let go of %1.", _them], "info"] call _fnc_reply;
    };
    case "putIn": {
        if (_faction != "cop") exitWith {};
        if (!_restrained) exitWith { format ["Restrain %1 first.", _them] call _fnc_reply; };
        private _vehicle = objNull;
        {
            if (alive _x && { (_x emptyPositions "cargo") > 0 } && { locked _x < 2 || { (_x getVariable ["alife_owner", ""]) == _uid } }) exitWith { _vehicle = _x; };
        } forEach (nearestObjects [_unit, ["LandVehicle", "Air", "Ship"], 8]);
        if (isNull _vehicle) exitWith { "No vehicle with a free back seat within 8 m (and unlocked)." call _fnc_reply; };
        if (!isNull attachedTo _target) then { detach _target; };
        ["putIn", _me, _vehicle] remoteExecCall ["ALife_fnc_personApply", owner _target];
        diag_log format ["[ALife] %1 put %2 in %3", _uid, getPlayerUID _target, typeOf _vehicle];
        [format ["You put %1 in the %2.", _them, getText (configFile >> "CfgVehicles" >> typeOf _vehicle >> "displayName")], "success"] call _fnc_reply;
    };
    case "takeOut": {
        if (_faction != "cop") exitWith {};
        if (!_restrained || { vehicle _target == _target }) exitWith {};
        ["takeOut", _me] remoteExecCall ["ALife_fnc_personApply", owner _target];
        [format ["You took %1 out.", _them], "success"] call _fnc_reply;
    };
    case "fine": {
        if (_faction != "cop") exitWith {};
        if (!(_arg isEqualType 0) || { !(_arg in getArray (missionConfigFile >> "CfgALife" >> "fines")) }) exitWith {};
        private _theirUid = getPlayerUID _target;
        private _pending = ALife_tickets getOrDefault [_theirUid, []];
        if (_pending isNotEqualTo [] && { diag_tickTime < (_pending select 3) }) exitWith { format ["%1 already has a fine waiting.", _them] call _fnc_reply; };
        private _timeout = getNumber (missionConfigFile >> "CfgALife" >> "fineTimeout") max 10;
        ALife_tickets set [_theirUid, [_uid, _me, _arg, diag_tickTime + _timeout]];
        ["offer", _me, _arg, _timeout] remoteExecCall ["ALife_fnc_fine", owner _target];
        diag_log format ["[ALife] %1 fined %2 $%3", _uid, _theirUid, _arg];
        [format ["You fined %1 $%2. Waiting for them to pay.", _them, [_arg] call BIS_fnc_numberText], "info"] call _fnc_reply;
    };
    case "strip": {
        if (_faction != "cop") exitWith {};
        if (!_restrained) exitWith { format ["Restrain %1 first.", _them] call _fnc_reply; };
        ["strip", _me] remoteExecCall ["ALife_fnc_personApply", owner _target];
        diag_log format ["[ALife] %1 stripped %2", _uid, getPlayerUID _target];
        [format ["You took %1's gear.", _them], "success"] call _fnc_reply;
    };
    case "heal": {
        if (_faction != "medic") exitWith {};
        if (damage _target < 0.01) exitWith { format ["%1 isn't hurt.", _them] call _fnc_reply; };
        if (!([_uid, "medic", "medkit", -1] call ALife_fnc_itemsChange)) exitWith { "You need a Medkit." call _fnc_reply; };
        ["heal", _target, 1, false] remoteExecCall ["ALife_fnc_itemUsed", owner _target];
        diag_log format ["[ALife] %1 healed %2 with a medkit", _uid, getPlayerUID _target];
        [format ["You treated %1.", _them], "success"] call _fnc_reply;
        [_target, format ["%1 treated your wounds.", _me], "success"] call _fnc_tell;
        [_unit] call ALife_fnc_requestPhoneData;
    };
};
