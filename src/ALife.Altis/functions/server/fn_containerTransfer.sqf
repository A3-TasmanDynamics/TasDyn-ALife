/*
    ALife_fnc_containerTransfer (server, called by the client; remoteExecCall
    so it runs in one go, with no other request in between)

    Moves items and cash between a player and a container (dropped bag or
    vehicle trunk). The client only says what it wants; everything is
    checked here: distance, what each side actually has, the player's carry
    weight and the container's space. Amounts are clamped, never trusted.

    Storing into id "" drops into a new bag at the player's feet.

    Arguments:
        0: OBJECT - the player
        1: STRING - container id ("" = new bag, store only)
        2: STRING - "take" (container -> player) or "store" (player -> container)
        3: ARRAY  - items [[class, quantity], ...]
        4: NUMBER - cash
*/

params [["_unit", objNull, [objNull]], ["_id", "", [""]], ["_direction", "", [""]], ["_entries", [], [[]]], ["_cash", 0, [0]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};
if (!(_direction in ["take", "store"])) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;
private _faction = _unit getVariable ["alife_faction", ""];
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
if (!alive _unit || { _faction == "" } || { (_record getOrDefault ["status", ""]) != "OK" }) exitWith {};
if (isNil "ALife_containers") then { ALife_containers = createHashMap; ALife_containerCount = 0; };

// Nothing to take from a bag that doesn't exist yet.
if (_id == "" && { _direction == "take" }) exitWith {};

private _itemsKey = _faction + "_items";
private _cashKey = _faction + "_cash";
private _have = createHashMapFromArray ((_record getOrDefault [_itemsKey, []]) select { _x isEqualTypeParams ["", 0] });
private _haveSomething = ((values _have) findIf { _x > 0 }) >= 0 || { (_record getOrDefault [_cashKey, 0]) > 0 };
if (_id == "" && { !_haveSomething }) exitWith {};
// Cash on its own lands as a money stack; anything with items, a bag.
if (_id == "") then { _id = [_unit, _entries isEqualTo []] call ALife_fnc_containerCreateBag; };

private _container = ALife_containers getOrDefault [_id, createHashMap];
if (count _container == 0 || { isNull (_container get "object") }) exitWith {
    "It's gone." call _fnc_reply;
    [_unit, ""] call ALife_fnc_containerSend;
};
private _object = _container get "object";
private _isVehicle = (_container get "kind") == "vehicle";
private _isLocker = (_container get "kind") == "locker";
if (_isLocker && { (_container get "owner") != _uid || { (_container get "faction") != _faction } }) exitWith {};
if (_isLocker) then { _cash = 0; }; // lockers hold items, not money
if (_unit distance _object > ([4, 8] select _isVehicle)) exitWith { "Get closer." call _fnc_reply; };

private _stock = _container get "items";
private _carry = getNumber (missionConfigFile >> "CfgALife" >> "carryWeight");
private _fnc_playerWeight = { [_record getOrDefault [_itemsKey, []]] call ALife_fnc_itemsWeight };
private _fnc_containerWeight = { [(keys _stock) apply { [_x, _stock get _x] }] call ALife_fnc_itemsWeight };

private _moved = [];
private _full = false;

{
    _x params [["_class", "", [""]], ["_qty", 0, [0]]];
    _qty = floor _qty;
    private _cfg = missionConfigFile >> "CfgItems" >> _class;
    if (_qty > 0 && { isClass _cfg }) then {
        private _weight = getNumber (_cfg >> "weight");
        if (_direction == "take") then {
            private _n = _qty min (_stock getOrDefault [_class, 0]);
            if (_weight > 0) then {
                private _fit = floor ((_carry - call _fnc_playerWeight) / _weight) max 0;
                if (_fit < _n) then { _n = _fit; _full = true; };
            };
            if (_n > 0 && { [_uid, _faction, _class, _n] call ALife_fnc_itemsChange }) then {
                private _left = (_stock get _class) - _n;
                if (_left > 0) then { _stock set [_class, _left] } else { _stock deleteAt _class };
                _moved pushBack format ["%1 x%2", getText (_cfg >> "displayName"), _n];
            };
        } else {
            private _owned = 0;
            { if ((_x select 0) == _class) then { _owned = _x select 1; }; } forEach (_record getOrDefault [_itemsKey, []]);
            private _n = _qty min _owned;
            if (_weight > 0) then {
                private _fit = floor (((_container get "capacity") - call _fnc_containerWeight) / _weight) max 0;
                if (_fit < _n) then { _n = _fit; _full = true; };
            };
            if (_n > 0 && { [_uid, _faction, _class, -_n] call ALife_fnc_itemsChange }) then {
                _stock set [_class, (_stock getOrDefault [_class, 0]) + _n];
                _moved pushBack format ["%1 x%2", getText (_cfg >> "displayName"), _n];
            };
        };
    };
} forEach _entries;

// Cash: weightless, moved with the same idempotent delta save as purchases.
_cash = floor _cash;
if (_cash > 0) then {
    private _n = if (_direction == "take") then { _cash min (_container get "cash") } else { _cash min (_record getOrDefault [_cashKey, 0]) };
    if (_n > 0) then {
        private _delta = [_n, -_n] select (_direction == "store");
        private _token = format ["inv-%1-%2-%3", _uid, round (diag_tickTime * 1000), floor random 1e6];
        private _result = ["save", [_uid, _cashKey, _delta toFixed 0, _token]] call ALife_fnc_extension;
        if (_result == "OK") then {
            _record set [_cashKey, (_record getOrDefault [_cashKey, 0]) + _delta];
            _container set ["cash", (_container get "cash") - _delta];
            _moved pushBack format ["$%1 cash", [_n] call BIS_fnc_numberText];
        } else {
            diag_log format ["[ALife] cash %1 of %2 for %3 failed: %4", _direction, _n, _uid, _result];
        };
    };
};

if (_moved isNotEqualTo []) then {
    diag_log format ["[ALife] %1 %2 %3 %4: %5", _uid, ["took from", "put in"] select (_direction == "store"), _container get "kind", _id, _moved joinString ", "];
};
if (_full) then {
    (["There isn't room for all of that.", "You can't carry all of that."] select (_direction == "take")) call _fnc_reply;
};

// An empty dropped bag goes away.
if ((_container get "kind") == "ground" && { count _stock == 0 } && { (_container get "cash") <= 0 }) then {
    ALife_groundBags = (missionNamespace getVariable ["ALife_groundBags", []]) - [_object];
    publicVariable "ALife_groundBags";
    deleteVehicle _object;
    ALife_containers deleteAt _id;
};

// A locker's items are saved with it (faction_lockers.items).
if (_isLocker && { _moved isNotEqualTo [] }) then { ["saveItems", _id] call ALife_fnc_locker; };

// An owned vehicle's trunk is saved with the vehicle (vehicles.inventory).
if (_isVehicle && { _moved isNotEqualTo [] }) then {
    private _vid = _object getVariable ["alife_vehicleId", ""];
    if (_vid != "") then {
        private _trunk = ([_object] call ALife_fnc_vehicleState) select 2;
        private _r = ["vehicle", ["inventory", _vid, str _trunk]] call ALife_fnc_extension;
        if (_r != "OK") then { diag_log format ["[ALife] trunk save for vehicle %1 failed: %2", _vid, _r]; };
    };
};
[_unit, _id] call ALife_fnc_containerSend;
