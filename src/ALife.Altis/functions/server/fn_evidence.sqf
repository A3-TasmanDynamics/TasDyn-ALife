/*
    ALife_fnc_evidence (server; "seize", "begin", "finish", "guess" and
    "police" are called by the client)

    The police evidence vault (CfgALife >> Evidence). Police seize items off
    the ground -- a dropped weapon / gear (a weapon holder) or a bag of
    virtual items and cash -- and they go into the vault, kept in the
    database (evidence_vault) until it's robbed or police destroy them.

    Robbing it, at objects placed in Eden (entrance, vaultDoor, keypad,
    vault; Windows on one):
        1. the entrance: bolt cutters (cutTime s)  -- police alerted, map marker
        2. the vault room door: a blasting charge (placeTime s, then fuse s)
        3. the keypad: a Keypad Decoder, then crack the code (codeLength
           digits, `guesses` tries: each says how many are right and in the
           right place, and how many are right but elsewhere; out of tries:
           locked for `lockout` s, a new code)
        4. the vault opens: a crate with everything seized, and a bag with
           its virtual items, its cash and cashBonus. The vault is empty.
    Needs minCops police online; one robbery per `cooldown` s; not finished
    within giveUpAfter s, the vault reseals. Everyone who works a stage is
    wanted (crime). Police at the keypad or vault can secure it (reseal,
    alarm off) or, when it's quiet, destroy the evidence.

    Modes:
        ["load"]                              - (serverInit) from the database; the objects
        ["seize", cop, object]
        ["begin", unit, stage]                - stage "entrance" / "vaultDoor" / "keypad"
        ["finish", unit, stage]               - the client's progress bar finished
        ["guess", unit, code]                 - a keypad guess (a string of digits)
        ["police", unit, "secure" | "destroy" | "count"]
*/

params [["_mode", "", [""]], ["_unit", objNull, [objNull]], ["_arg", ""]];

if (!isServer) exitWith {};
private _cfg = missionConfigFile >> "CfgALife" >> "Evidence";
// (The server itself: "load", and "police" "timeout" with no unit.)
if (_mode != "load" && { isNull _unit } && { !(_mode == "police" && { _arg isEqualTo "timeout" }) }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith { ["spoof", _unit, "evidence", remoteExecutedOwner] call ALife_fnc_anticheat; };
if (isRemoteExecuted && { ["rate", _unit, "evidence"] call ALife_fnc_anticheat }) exitWith {};

private _owner = if (isNull _unit) then { 0 } else { owner _unit };
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _obj = { missionNamespace getVariable [getText (_cfg >> _this), objNull] };
private _cops = { count (allPlayers select { (_x getVariable ["alife_faction", ""]) == "cop" }) };
private _has = {
    params ["_who", "_class"];
    private _items = ((missionNamespace getVariable ["ALife_sessions", createHashMap]) getOrDefault [getPlayerUID _who, createHashMap]) getOrDefault ["civ_items", []];
    (_items findIf { _x isEqualTypeParams ["", 0] && { (_x select 0) == _class } && { (_x select 1) > 0 } }) >= 0
};

// ---- What's in a weapon holder / crate (the locker's gear format) ----------
private _fnc_pairs = {
    (getItemCargo _this) params [["_classes", []], ["_counts", []]];
    private _out = [];
    { _out pushBack [_x, _counts select _forEachIndex]; } forEach _classes;
    _out
};
private _fnc_read = {
    private _box = _this;
    private _containers = everyContainer _box;
    private _items = _box call _fnc_pairs;
    // Uniforms / vests are items as well as containers: listed once, with their contents.
    {
        private _class = _x select 0;
        private _i = _items findIf { (_x select 0) == _class };
        if (_i >= 0) then {
            private _n = ((_items select _i) select 1) - 1;
            if (_n > 0) then { (_items select _i) set [1, _n]; } else { _items deleteAt _i; };
        };
    } forEach _containers;
    [
        ["items", _items],
        ["mags", magazinesAmmoCargo _box],
        ["weapons", weaponsItemsCargo _box],
        ["containers", _containers apply { [_x select 0, (_x select 1) call _fnc_pairs, magazinesAmmoCargo (_x select 1), weaponsItemsCargo (_x select 1)] }]
    ]
};
private _fnc_fill = {
    params ["_box", "_gear"];
    private _fnc_into = {
        params ["_c", ["_items", []], ["_mags", []], ["_weapons", []]];
        { _c addItemCargoGlobal [_x select 0, _x select 1]; } forEach (_items select { _x isEqualTypeParams ["", 0] });
        { _c addMagazineAmmoCargo [_x select 0, 1, _x select 1]; } forEach (_mags select { _x isEqualTypeParams ["", 0] });
        { _c addWeaponWithAttachmentsCargoGlobal [_x, 1]; } forEach (_weapons select { _x isEqualType [] && { (_x param [0, ""]) isEqualType "" } });
    };
    private _data = createHashMapFromArray (_gear select { _x isEqualTypeParams ["", []] });
    [_box, _data getOrDefault ["items", []], _data getOrDefault ["mags", []], _data getOrDefault ["weapons", []]] call _fnc_into;
    {
        _x params [["_class", ""], ["_items", []], ["_mags", []], ["_weapons", []]];
        if (_class isEqualType "" && { _class != "" }) then {
            private _before = (everyContainer _box) apply { _x select 1 };
            if (isClass (configFile >> "CfgVehicles" >> _class)) then { _box addBackpackCargoGlobal [_class, 1]; } else { _box addItemCargoGlobal [_class, 1]; };
            private _new = ((everyContainer _box) apply { _x select 1 }) - _before;
            if (_new isNotEqualTo []) then { [_new select 0, _items, _mags, _weapons] call _fnc_into; };
        };
    } forEach (_data getOrDefault ["containers", []]);
};
// Save the vault to the database.
private _fnc_save = {
    private _e = ALife_evidence;
    private _items = (keys (_e get "items")) apply { [_x, (_e get "items") get _x] };
    private _r = ["evidence", ["set", str (_e get "gear"), str _items, str (_e get "cash")]] call ALife_fnc_extension;
    if (_r != "OK") then { diag_log format ["[ALife] evidence save failed: %1", _r]; };
    _r == "OK"
};
private _fnc_empty = { ALife_evidence = createHashMapFromArray [["gear", [["items", []], ["mags", []], ["weapons", []], ["containers", []]]], ["items", createHashMap], ["cash", 0]]; };
// How much is in it, for people.
private _fnc_count = {
    private _g = createHashMapFromArray ((ALife_evidence get "gear") select { _x isEqualTypeParams ["", []] });
    [count (_g getOrDefault ["weapons", []]), count (_g getOrDefault ["mags", []]) + count (_g getOrDefault ["items", []]) + count (_g getOrDefault ["containers", []]),
     (values (ALife_evidence get "items")) call { private _n = 0; { _n = _n + _x; } forEach _this; _n }, ALife_evidence get "cash"]
};
// Police: the alarm (marker + notice) on or off.
private _fnc_alarm = {
    params ["_on", ["_text", ""]];
    private _pos = getPosATL ("entrance" call _obj);
    {
        if ((_x getVariable ["alife_faction", ""]) == "cop") then { ["alarm", [_on, _pos, _text]] remoteExecCall ["ALife_fnc_evidenceClient", owner _x]; };
    } forEach allPlayers;
};
// Robbers: wanted (once each per robbery).
private _fnc_wanted = {
    private _who = _this;
    private _uid = getPlayerUID _who;
    private _done = ALife_vault getOrDefault ["robbers", []];
    if (_uid in _done) exitWith {};
    _done pushBack _uid;
    ALife_vault set ["robbers", _done];
    private _crime = missionConfigFile >> "CfgCrimes" >> getText (_cfg >> "crime");
    if (!isClass _crime || { (_who getVariable ["alife_faction", ""]) != "civ" }) exitWith {};
    private _bounty = getNumber (_crime >> "bounty");
    private _r = ["wanted", ["add", _uid, configName _crime, str _bounty, ""]] call ALife_fnc_extension;
    if (_r select [0, 3] != "OK:") exitWith { diag_log format ["[ALife] evidence wanted add failed: %1", _r]; };
    private _record = ALife_sessions getOrDefault [_uid, createHashMap];
    private _total = (_record getOrDefault ["civ_bounty", 0]) + _bounty;
    _record set ["civ_bounty", _total];
    _who setVariable ["alife_bounty", _total, true];
};
// Back to sealed.
private _fnc_reset = {
    private _keep = ALife_vault getOrDefault ["cooldownUntil", 0];
    ALife_vault = createHashMapFromArray [["stage", 0], ["cooldownUntil", _keep], ["robbers", []]];
    [false] call _fnc_alarm;
};

switch (_mode) do {
    case "load": {
        call _fnc_empty;
        ALife_vault = createHashMapFromArray [["stage", 0], ["cooldownUntil", 0], ["robbers", []]];
        private _text = "";
        private _parts = 1;
        private _ok = true;
        for "_i" from 0 to 300 do {
            if (_i >= _parts) exitWith {};
            private _r = ["evidence", ["get", str _i]] call ALife_fnc_extension;
            if ((_r select [0, 3]) != "OK:") exitWith { _ok = false; diag_log format ["[ALife] evidence load failed: %1", _r]; };
            private _rest = _r select [3];
            private _colon = _rest find ":";
            _parts = parseNumber (_rest select [0, _colon]);
            _text = _text + (_rest select [_colon + 1]);
        };
        if (_ok) then {
            (parseSimpleArray _text) params [["_gear", []], ["_items", []], ["_cash", 0]];
            if (_gear isNotEqualTo []) then { ALife_evidence set ["gear", _gear]; };
            { if (_x isEqualTypeParams ["", 0]) then { (ALife_evidence get "items") set [_x select 0, _x select 1]; }; } forEach _items;
            ALife_evidence set ["cash", _cash];
        };
        // The vault's objects: still and unhurt.
        {
            private _o = _x call _obj;
            if (!isNull _o) then { _o allowDamage false; _o enableSimulationGlobal false; };
        } forEach ["entrance", "vaultDoor", "keypad", "vault"];
        diag_log format ["[ALife] evidence vault loaded: %1", call _fnc_count];
        // Not finished in time: resealed.
        [] spawn {
            while { true } do {
                sleep 15;
                private _stage = ALife_vault getOrDefault ["stage", 0];
                if (_stage in [1, 2, 2.5] && { time - (ALife_vault getOrDefault ["startedAt", time]) > getNumber (missionConfigFile >> "CfgALife" >> "Evidence" >> "giveUpAfter") }) then {
                    diag_log "[ALife] evidence vault robbery timed out: resealed";
                    ["police", objNull, "timeout"] call ALife_fnc_evidence;
                };
            };
        };
    };

    case "seize": {
        private _target = _arg;
        if !(_target isEqualType objNull) exitWith {};
        if ((_unit getVariable ["alife_faction", ""]) != "cop" || { !alive _unit } || { isNull _target }) exitWith {};
        if (_unit distance _target > getNumber (_cfg >> "seizeRange") + 1) exitWith { "Get closer." call _fnc_reply; };
        private _what = [];
        // A bag of virtual items (and cash) on the ground.
        private _bagId = "";
        if (_target in (missionNamespace getVariable ["ALife_groundBags", []])) then {
            { if ((_y get "object") isEqualTo _target && { (_y get "kind") == "ground" }) exitWith { _bagId = _x; }; } forEach (missionNamespace getVariable ["ALife_containers", createHashMap]);
        };
        if (_bagId != "") then {
            private _c = ALife_containers get _bagId;
            private _stock = _c get "items";
            private _store = ALife_evidence get "items";
            { _store set [_x, (_store getOrDefault [_x, 0]) + _y]; } forEach _stock;
            ALife_evidence set ["cash", (ALife_evidence get "cash") + (_c getOrDefault ["cash", 0])];
            _what pushBack format ["%1 item(s)%2", count _stock, ["", format [", $%1", [_c getOrDefault ["cash", 0]] call BIS_fnc_numberText]] select ((_c getOrDefault ["cash", 0]) > 0)];
            ALife_containers deleteAt _bagId;
            ALife_groundBags = (missionNamespace getVariable ["ALife_groundBags", []]) - [_target];
            publicVariable "ALife_groundBags";
        };
        // Weapons / gear on the ground (a weapon holder; a bag is a simple object, no cargo).
        if (_bagId == "" && { _target isKindOf "WeaponHolder" || { _target isKindOf "WeaponHolderSimulated" } }) then {
            private _gear = _target call _fnc_read;
            private _store = ALife_evidence get "gear";
            {
                _x params ["_key", "_list"];
                private _i = _store findIf { (_x select 0) == _key };
                if (_i < 0) then { _store pushBack [_key, +_list]; } else { ((_store select _i) select 1) append _list; };
            } forEach _gear;
            private _n = count ((createHashMapFromArray _gear) get "weapons");
            _what pushBack format ["%1 weapon(s) and gear", _n];
        };
        if (_what isEqualTo []) exitWith { "There's nothing there to seize." call _fnc_reply; };
        deleteVehicle _target;
        call _fnc_save;
        diag_log format ["[ALife] %1 seized %2 into evidence", getPlayerUID _unit, _what];
        ["log", _unit, "police.seize", [["what", _what joinString "; "], ["position", (getPosATL _unit) apply { round _x }]]] call ALife_fnc_anticheat;
        [format ["Seized %1. It's in the evidence vault.", _what joinString " and "], "success"] call _fnc_reply;
    };

    case "begin": {
        // A robber starts on a stage: checks, then their progress bar (or the keypad).
        private _stage = ALife_vault getOrDefault ["stage", 0];
        if ((_unit getVariable ["alife_faction", ""]) != "civ" || { !alive _unit }) exitWith {};
        private _target = _arg call _obj;
        if (isNull _target || { _unit distance _target > getNumber (_cfg >> "range") + 1.5 }) exitWith { "Get closer." call _fnc_reply; };
        switch (_arg) do {
            case "entrance": {
                if (_stage > 0) exitWith { "The door's already open." call _fnc_reply; };
                if (time < (ALife_vault getOrDefault ["cooldownUntil", 0])) exitWith { format ["The vault was hit not long ago. Try again in %1 min.", ceil (((ALife_vault get "cooldownUntil") - time) / 60)] call _fnc_reply; };
                if (call _cops < getNumber (_cfg >> "minCops")) exitWith { format ["There need to be at least %1 police on duty.", getNumber (_cfg >> "minCops")] call _fnc_reply; };
                if !([_unit, "boltcutter"] call _has) exitWith { "You need Bolt Cutters." call _fnc_reply; };
                ALife_vault set ["pending", [getPlayerUID _unit, _arg, diag_tickTime]];
                ["progress", [_arg, getNumber (_cfg >> "cutTime"), "Cutting through the door..."]] remoteExecCall ["ALife_fnc_evidenceClient", _owner];
            };
            case "vaultDoor": {
                if (_stage < 1) exitWith { "Get through the building's door first." call _fnc_reply; };
                if (_stage > 1) exitWith { (["The charge is already set.", "The vault room door is open."] select (_stage >= 2)) call _fnc_reply; };
                if !([_unit, "blastingcharge"] call _has) exitWith { "You need a Blasting Charge." call _fnc_reply; };
                ALife_vault set ["pending", [getPlayerUID _unit, _arg, diag_tickTime]];
                ["progress", [_arg, getNumber (_cfg >> "placeTime"), "Setting the charge..."]] remoteExecCall ["ALife_fnc_evidenceClient", _owner];
            };
            case "keypad": {
                if (_stage < 2 || { _stage == 2.5 }) exitWith { "Get into the vault room first." call _fnc_reply; };
                if (_stage >= 3) exitWith { "The vault's open." call _fnc_reply; };
                if (time < (ALife_vault getOrDefault ["lockedUntil", 0])) exitWith { format ["The keypad's locked out. %1 s.", ceil ((ALife_vault get "lockedUntil") - time)] call _fnc_reply; };
                if !([_unit, "keypaddecoder"] call _has) exitWith { "You need a Keypad Decoder." call _fnc_reply; };
                if ((ALife_vault getOrDefault ["code", ""]) == "") then {
                    private _code = "";
                    for "_i" from 1 to (getNumber (_cfg >> "codeLength") max 3) do { _code = _code + str (floor random 10); };
                    ALife_vault set ["code", _code];
                    ALife_vault set ["left", getNumber (_cfg >> "guesses")];
                    ALife_vault set ["history", []];
                };
                _unit call _fnc_wanted;
                ["crack", [count (ALife_vault get "code"), ALife_vault get "left", ALife_vault get "history"]] remoteExecCall ["ALife_fnc_evidenceClient", _owner];
            };
        };
    };

    case "finish": {
        // Their progress bar ran out: was it theirs, long enough, still there?
        (ALife_vault getOrDefault ["pending", []]) params [["_who", ""], ["_stage", ""], ["_at", 0]];
        if (_who != getPlayerUID _unit || { _stage != _arg }) exitWith {};
        private _need = getNumber (_cfg >> (["placeTime", "cutTime"] select (_stage == "entrance")));
        if (diag_tickTime - _at < _need - 1.5) exitWith { diag_log format ["[ALife] evidence %1 by %2 finished too soon", _stage, _who]; };
        private _target = _stage call _obj;
        if (!alive _unit || { _unit distance _target > getNumber (_cfg >> "range") + 2 }) exitWith {};
        ALife_vault deleteAt "pending";
        private _uid = getPlayerUID _unit;
        switch (_stage) do {
            case "entrance": {
                if ((ALife_vault get "stage") != 0) exitWith {};
                if (!([_uid, "civ", "boltcutter", -1] call ALife_fnc_itemsChange)) exitWith { "You need Bolt Cutters." call _fnc_reply; };
                ALife_vault set ["stage", 1];
                ALife_vault set ["startedAt", time];
                _unit call _fnc_wanted;
                [true, "Break-in at the evidence vault!"] call _fnc_alarm;
                diag_log format ["[ALife] evidence vault: %1 cut the door", _uid];
                ["log", _unit, "evidence.breakin", [["stage", "entrance"]]] call ALife_fnc_anticheat;
                ["You're in. Police have been alerted. The vault room door is next: a Blasting Charge.", "warning"] call _fnc_reply;
                [_unit] call ALife_fnc_requestPhoneData;
            };
            case "vaultDoor": {
                if ((ALife_vault get "stage") != 1) exitWith {};
                if (!([_uid, "civ", "blastingcharge", -1] call ALife_fnc_itemsChange)) exitWith { "You need a Blasting Charge." call _fnc_reply; };
                ALife_vault set ["stage", 2.5]; // the fuse is burning
                _unit call _fnc_wanted;
                private _fuse = getNumber (_cfg >> "fuse");
                diag_log format ["[ALife] evidence vault: %1 set the charge", _uid];
                ["log", _unit, "evidence.breakin", [["stage", "vaultDoor"]]] call ALife_fnc_anticheat;
                [format ["Charge set: %1 seconds. Stand well back!", _fuse], "warning"] call _fnc_reply;
                [true, "The evidence vault room is being blown open!"] call _fnc_alarm;
                [_target, _fuse] spawn {
                    params ["_door", "_fuse"];
                    sleep _fuse;
                    if ((ALife_vault getOrDefault ["stage", 0]) != 2.5) exitWith {}; // secured meanwhile
                    // A bang and smoke, no damage.
                    playSound3D ["A3\Sounds_F\arsenal\explosives\grenades\Explosion_HE_grenade_01.wss", _door, false, getPosASL _door, 5, 1, 1500];
                    private _smoke = "SmokeShell" createVehicle (getPosATL _door);
                    ALife_vault set ["stage", 2];
                    ALife_vault set ["code", ""];
                    diag_log "[ALife] evidence vault: the vault room door is open";
                };
            };
        };
    };

    case "guess": {
        if ((ALife_vault getOrDefault ["stage", 0]) != 2 || { (_unit getVariable ["alife_faction", ""]) != "civ" }) exitWith {};
        private _uid = getPlayerUID _unit;
        private _keypad = "keypad" call _obj;
        if (_unit distance _keypad > getNumber (_cfg >> "range") + 2) exitWith {};
        if (time < (ALife_vault getOrDefault ["lockedUntil", 0])) exitWith { ["locked", ceil ((ALife_vault get "lockedUntil") - time)] remoteExecCall ["ALife_fnc_evidenceClient", _owner]; };
        if !([_unit, "keypaddecoder"] call _has) exitWith { "You need a Keypad Decoder." call _fnc_reply; };
        private _code = ALife_vault getOrDefault ["code", ""];
        private _guess = if (_arg isEqualType "") then { _arg } else { "" };
        if (count _guess != count _code || { (_guess splitString "0123456789") isNotEqualTo [] }) exitWith {};
        // Right digit right place; right digit elsewhere (each digit counted once).
        private _exact = 0;
        private _restCode = [];
        private _restGuess = [];
        for "_i" from 0 to count _code - 1 do {
            if ((_code select [_i, 1]) == (_guess select [_i, 1])) then { _exact = _exact + 1; } else {
                _restCode pushBack (_code select [_i, 1]);
                _restGuess pushBack (_guess select [_i, 1]);
            };
        };
        private _near = 0;
        { private _j = _restCode find _x; if (_j >= 0) then { _near = _near + 1; _restCode deleteAt _j; }; } forEach _restGuess;
        private _history = ALife_vault getOrDefault ["history", []];
        _history pushBack [_guess, _exact, _near];
        private _left = (ALife_vault getOrDefault ["left", 0]) - 1;
        ALife_vault set ["left", _left];
        if (_exact == count _code) exitWith {
            // Cracked: the vault opens -- a crate with the gear, a bag with the items and cash.
            ALife_vault set ["stage", 3];
            [_uid, "civ", "keypaddecoder", -1] call ALife_fnc_itemsChange;
            ["cracked", []] remoteExecCall ["ALife_fnc_evidenceClient", _owner];
            private _vault = "vault" call _obj;
            private _pos = _vault getPos [1.5, getDir _vault];
            private _crate = createVehicle [getText (_cfg >> "crate"), [_pos select 0, _pos select 1, 0], [], 0, "CAN_COLLIDE"];
            clearWeaponCargoGlobal _crate; clearMagazineCargoGlobal _crate; clearItemCargoGlobal _crate; clearBackpackCargoGlobal _crate;
            [_crate, ALife_evidence get "gear"] call _fnc_fill;
            // Its virtual items and cash: a ground container on the crate (Windows on it).
            private _cash = (ALife_evidence get "cash") + getNumber (_cfg >> "cashBonus");
            if (isNil "ALife_containers") then { ALife_containers = createHashMap; ALife_containerCount = 0; };
            ALife_containerCount = ALife_containerCount + 1;
            private _id = format ["bag_vault_%1", ALife_containerCount];
            ALife_containers set [_id, createHashMapFromArray [
                ["object", _crate], ["kind", "ground"], ["owner", ""], ["title", "Evidence vault"],
                ["items", +(ALife_evidence get "items")], ["cash", _cash], ["capacity", 100000], ["created", time]
            ]];
            ALife_groundBags = (missionNamespace getVariable ["ALife_groundBags", []]) + [_crate];
            publicVariable "ALife_groundBags";
            private _count = call _fnc_count;
            call _fnc_empty;
            call _fnc_save;
            ALife_vault set ["cooldownUntil", time + getNumber (_cfg >> "cooldown")];
            diag_log format ["[ALife] evidence vault cracked by %1: %2 weapons, %3 other, %4 items, $%5", getPlayerUID _unit, _count select 0, _count select 1, _count select 2, _cash];
            ["log", _unit, "evidence.robbed", [["weapons", _count select 0], ["gear", _count select 1], ["items", _count select 2], ["cash", _cash]]] call ALife_fnc_anticheat;
            [true, "The evidence vault has been cracked open!"] call _fnc_alarm;
            // The crate goes after a while; the vault reseals with it.
            [_crate, _id] spawn {
                params ["_crate", "_id"];
                sleep getNumber (missionConfigFile >> "CfgALife" >> "Evidence" >> "lootLifetime");
                ALife_containers deleteAt _id;
                ALife_groundBags = (missionNamespace getVariable ["ALife_groundBags", []]) - [_crate];
                publicVariable "ALife_groundBags";
                deleteVehicle _crate;
                if ((ALife_vault getOrDefault ["stage", 0]) == 3) then { ["police", objNull, "timeout"] call ALife_fnc_evidence; };
            };
        };
        if (_left <= 0) exitWith {
            ALife_vault set ["lockedUntil", time + getNumber (_cfg >> "lockout")];
            ALife_vault set ["code", ""];
            ["locked", getNumber (_cfg >> "lockout")] remoteExecCall ["ALife_fnc_evidenceClient", _owner];
            [true, "Someone is trying to crack the evidence vault keypad!"] call _fnc_alarm;
        };
        ["crack", [count _code, _left, _history]] remoteExecCall ["ALife_fnc_evidenceClient", _owner];
    };

    case "police": {
        // _arg: "secure" (reseal, alarm off), "destroy" (the evidence, when it's quiet),
        // "count" (what's in it); "timeout" from the server itself.
        if (_arg == "timeout") exitWith { if (!isRemoteExecuted) then { call _fnc_reset; }; };
        if ((_unit getVariable ["alife_faction", ""]) != "cop" || { !alive _unit }) exitWith {};
        private _near = ["keypad", "vault", "entrance", "vaultDoor"] findIf { private _o = _x call _obj; !isNull _o && { _unit distance _o < getNumber (_cfg >> "range") + 2 } } >= 0;
        if (!_near) exitWith { "Get closer." call _fnc_reply; };
        (call _fnc_count) params ["_weapons", "_gear", "_items", "_cash"];
        switch (_arg) do {
            case "count": {
                [format ["Evidence: %1 weapon(s), %2 other gear, %3 item(s), $%4. %5", _weapons, _gear, _items, [_cash] call BIS_fnc_numberText,
                    ["The vault is sealed.", "There's a break-in in progress!"] select ((ALife_vault getOrDefault ["stage", 0]) > 0)], "info"] call _fnc_reply;
            };
            case "secure": {
                if ((ALife_vault getOrDefault ["stage", 0]) == 0) exitWith { "The vault's already sealed." call _fnc_reply; };
                call _fnc_reset;
                diag_log format ["[ALife] %1 secured the evidence vault", getPlayerUID _unit];
                ["log", _unit, "police.vault_secured", []] call ALife_fnc_anticheat;
                ["Vault secured and resealed.", "success"] call _fnc_reply;
            };
            case "destroy": {
                if ((ALife_vault getOrDefault ["stage", 0]) > 0) exitWith { "Not during a break-in." call _fnc_reply; };
                call _fnc_empty;
                call _fnc_save;
                diag_log format ["[ALife] %1 destroyed the evidence (%2 weapons, %3 gear, %4 items, $%5)", getPlayerUID _unit, _weapons, _gear, _items, _cash];
                ["log", _unit, "police.evidence_destroyed", [["weapons", _weapons], ["gear", _gear], ["items", _items], ["cash", _cash]]] call ALife_fnc_anticheat;
                ["Evidence destroyed.", "success"] call _fnc_reply;
            };
        };
    };
};
