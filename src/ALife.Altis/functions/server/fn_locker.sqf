/*
    ALife_fnc_locker (server; "open" / "items" / "save" / "close" are
    called by the client)

    Personal lockers for police and EMS (faction_lockers): open one at any
    of your faction's locker objects (CfgALife >> Lockers, placed in Eden).

      Weapons and clothing: a crate with your stored gear appears at the
      locker, hidden from everyone else, and Arma's inventory opens on it.
      What's in the crate is saved (read here, on the server) each time you
      take or put something, and when you close it -- then it's removed.
      How much fits: the crate's own space (Lockers >> crate).

      Items: our inventory screen (ALife_fnc_containerData) with your
      locker's virtual items -- container kind "locker", only yours, no
      cash, saved after each move (ALife_fnc_containerTransfer).

    Modes:
        ["open", unit, lockerObject]   - the gear crate
        ["items", unit, lockerObject]  - the items
        ["save", unit]                 - save the open crate (Take / Put)
        ["close", unit]                - save it and take it away
        ["close", uid]                 - (server) the same, they may be gone
        ["saveItems", containerId]     - (server) save a locker's items
*/

params [["_mode", "", [""]], ["_unit", objNull, [objNull, ""]], ["_locker", objNull, [objNull]]];

if (!isServer) exitWith {};
if (isNil "ALife_lockerBoxes") then { ALife_lockerBoxes = createHashMap; }; // uid -> [crate, faction]
private _cfg = missionConfigFile >> "CfgALife" >> "Lockers";

// ---- Gear: read a crate / fill one --------------------------------------
private _fnc_pairs = {
    (getItemCargo _this) params [["_classes", []], ["_counts", []]];
    private _out = [];
    { _out pushBack [_x, _counts select _forEachIndex]; } forEach _classes;
    _out
};
private _fnc_read = {
    private _box = _this;
    private _containers = everyContainer _box;
    // Uniforms and vests are items as well as containers: listed once, with their contents.
    private _items = _box call _fnc_pairs;
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
// A locker's [gear, items] from the database (answered in parts).
private _fnc_load = {
    params ["_uid", "_faction"];
    private _text = "";
    private _parts = 1;
    private _ok = true;
    for "_i" from 0 to 200 do {
        if (_i >= _parts) exitWith {};
        private _r = ["locker", ["get", _uid, _faction, str _i]] call ALife_fnc_extension;
        if ((_r select [0, 3]) != "OK:") exitWith { _ok = false; diag_log format ["[ALife] locker load for %1 (%2) failed: %3", _uid, _faction, _r]; };
        private _rest = _r select [3];
        private _colon = _rest find ":";
        _parts = parseNumber (_rest select [0, _colon]);
        _text = _text + (_rest select [_colon + 1]);
    };
    if (!_ok) exitWith { [] };
    private _data = parseSimpleArray _text;
    if !(_data isEqualTypeArray [[], []]) exitWith { [[], []] };
    _data
};
// Save a crate's contents; returns true if saved.
private _fnc_saveBox = {
    params ["_uid"];
    (ALife_lockerBoxes getOrDefault [_uid, []]) params [["_box", objNull], ["_faction", ""]];
    if (isNull _box) exitWith { false };
    private _r = ["locker", ["setGear", _uid, _faction, str (_box call _fnc_read)]] call ALife_fnc_extension;
    if (_r != "OK") then { diag_log format ["[ALife] locker save for %1 (%2) failed: %3", _uid, _faction, _r]; };
    _r == "OK"
};
private _fnc_closeBox = {
    params ["_uid"];
    (ALife_lockerBoxes getOrDefault [_uid, []]) params [["_box", objNull]];
    if (isNull _box) exitWith { ALife_lockerBoxes deleteAt _uid; true };
    if (!([_uid] call _fnc_saveBox)) exitWith { false }; // keep it: nothing is lost
    deleteVehicle _box;
    ALife_lockerBoxes deleteAt _uid;
    true
};

// ---- (server) close by uid: they walked off, left or died ---------------
if (_mode == "close" && { _unit isEqualType "" } && { !isRemoteExecuted }) exitWith {
    private _uid = _unit;
    if (!([_uid] call _fnc_closeBox)) then {
        // Couldn't save: try again shortly (the crate stays, hidden, until then).
        _uid spawn { sleep 10; ["close", _this] call ALife_fnc_locker; };
    };
};

// ---- (server) save a locker's virtual items ------------------------------
if (_mode == "saveItems") exitWith {
    if (isRemoteExecuted) exitWith {};
    private _id = _unit;
    private _c = (missionNamespace getVariable ["ALife_containers", createHashMap]) getOrDefault [_id, createHashMap];
    if (count _c == 0) exitWith {};
    private _stock = _c get "items";
    private _r = ["locker", ["setItems", _c get "owner", _c get "faction", str ((keys _stock) apply { [_x, _stock get _x] })]] call ALife_fnc_extension;
    if (_r != "OK") then { diag_log format ["[ALife] locker items save for %1 failed: %2", _c get "owner", _r]; };
};

if (!(_unit isEqualType objNull) || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};
private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;

if (_mode == "save") exitWith { [_uid] call _fnc_saveBox; };
if (_mode == "close") exitWith {
    if (!([_uid] call _fnc_closeBox)) then { "Your locker couldn't be saved. Close it again in a moment." call _fnc_reply; };
};

// ---- open / items: at one of your faction's lockers ------------------------
private _faction = _unit getVariable ["alife_faction", ""];
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
if (!(_mode in ["open", "items"]) || { isNull _locker } || { !alive _unit } || { (_record getOrDefault ["status", ""]) != "OK" }) exitWith {};
if (!(_faction in ["cop", "medic"]) || { !(vehicleVarName _locker in getArray (_cfg >> _faction)) }) exitWith { "That isn't your locker." call _fnc_reply; };
if (_unit distance _locker > getNumber (_cfg >> "range") + 1) exitWith { "Get closer." call _fnc_reply; };

// A crate still open from before (closed badly): saved and removed first.
if (!([_uid] call _fnc_closeBox)) exitWith { "Your locker is busy saving. Try again in a moment." call _fnc_reply; };
private _data = [_uid, _faction] call _fnc_load;
if (_data isEqualTo []) exitWith { "Your locker couldn't be opened. Try again." call _fnc_reply; };
_data params ["_gear", "_items"];

if (_mode == "items") exitWith {
    if (isNil "ALife_containers") then { ALife_containers = createHashMap; ALife_containerCount = 0; };
    private _id = format ["locker_%1_%2", _uid, _faction];
    private _stock = createHashMap;
    { if (_x isEqualTypeParams ["", 0] && { (_x select 1) > 0 }) then { _stock set [_x select 0, _x select 1]; }; } forEach _items;
    ALife_containers set [_id, createHashMapFromArray [
        ["object", _locker],
        ["kind", "locker"],
        ["owner", _uid],
        ["faction", _faction],
        ["title", "Your locker"],
        ["items", _stock],
        ["cash", 0],
        ["capacity", getNumber (_cfg >> "itemCapacity")],
        ["created", time]
    ]];
    [_unit, _id] call ALife_fnc_containerSend;
};

// open: the gear crate, hidden from everyone but its owner.
private _pos = _unit modelToWorld [0, 1, 0];
private _box = createVehicle [getText (_cfg >> "crate"), [_pos select 0, _pos select 1, 0], [], 0, "CAN_COLLIDE"];
_box allowDamage false;
clearWeaponCargoGlobal _box;
clearMagazineCargoGlobal _box;
clearItemCargoGlobal _box;
clearBackpackCargoGlobal _box;
[_box, _gear] call _fnc_fill;
_box hideObjectGlobal true;
_box setVariable ["alife_lockerOwner", _uid, true];
ALife_lockerBoxes set [_uid, [_box, _faction]];
["opened", _box] remoteExecCall ["ALife_fnc_lockerClient", _owner];
diag_log format ["[ALife] %1 opened their %2 locker", _uid, _faction];

// Walked off, left or died with it open: saved and removed.
[_uid, _box, _unit] spawn {
    params ["_uid", "_box", "_unit"];
    waitUntil { sleep 2; isNull _box || { isNull _unit } || { !alive _unit } || { _unit distance _box > 12 } };
    if (!isNull _box) then { ["close", _uid] call ALife_fnc_locker; };
};
