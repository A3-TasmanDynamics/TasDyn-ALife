/*
    ALife_fnc_itemsChange (server)

    Adds or removes virtual items for a player's faction, in their loaded
    record, and saves <faction>_items to the database.

    Arguments:
        0: STRING - Steam UID
        1: STRING - faction ("civ" / "cop" / "medic")
        2: STRING - item class (CfgItems)
        3: NUMBER - change (+ to add, - to remove)

    Returns:
        BOOL - false if the item is unknown, they don't have enough, or the
        save failed (nothing changes then)
*/

params ["_uid", "_faction", "_class", "_delta"];

if (!isServer) exitWith { false };
if (!isClass (missionConfigFile >> "CfgItems" >> _class)) exitWith { false };

private _record = ALife_sessions getOrDefault [_uid, createHashMap];
if ((_record getOrDefault ["status", ""]) != "OK") exitWith { false };

private _key = _faction + "_items";
private _items = createHashMapFromArray ((_record getOrDefault [_key, []]) select { _x isEqualTypeParams ["", 0] });
private _count = (_items getOrDefault [_class, 0]) + round _delta;
if (_count < 0) exitWith { false };

if (_count == 0) then { _items deleteAt _class } else { _items set [_class, _count] };
private _pairs = (keys _items) apply { [_x, _items get _x] };

private _result = ["save", [_uid, _key, str _pairs, "-"]] call ALife_fnc_extension;
if (_result != "OK") exitWith {
    diag_log format ["[ALife] items save failed for %1 (%2 %3 %4): %5", _uid, _faction, _class, _delta, _result];
    false
};

_record set [_key, _pairs];
true
