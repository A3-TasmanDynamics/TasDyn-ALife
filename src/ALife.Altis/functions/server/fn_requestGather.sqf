/*
    ALife_fnc_requestGather (server, called by the client: Windows in a
    gathering area)

    Gathers a resource (CfgResources.hpp >> CfgGathering). Checked here:
    that the player is inside one of its area markers, has the tool, isn't
    already busy, and has room to carry some. The server then times it
    (`time` seconds, the player sees a progress bar: ALife_fnc_resourceProgress),
    checks again that they're still there and alive, and gives them
    amount[] (as much as fits).

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - CfgGathering class
*/

params [["_unit", objNull, [objNull]], ["_resource", "", [""]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
private _cfg = missionConfigFile >> "CfgGathering" >> _resource;

if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" } || { !isClass _cfg }) exitWith {};
if (_unit getVariable ["alife_restrained", false]) exitWith {};
if (isNil "ALife_busy") then { ALife_busy = createHashMap; }; // uid -> true while gathering / processing
if (ALife_busy getOrDefault [_uid, false]) exitWith {};

private _fnc_inZone = { (getArray (_cfg >> "zones") findIf { _x in allMapMarkers && { _unit inArea _x } }) >= 0 };
if !(call _fnc_inZone) exitWith { "You're not in a gathering area for that." call _fnc_reply; };

private _items = createHashMapFromArray ((_record getOrDefault [_faction + "_items", []]) select { _x isEqualTypeParams ["", 0] });
private _tool = getText (_cfg >> "tool");
if (_tool != "" && { (_items getOrDefault [_tool, 0]) < 1 }) exitWith {
    format ["You need a %1.", getText (missionConfigFile >> "CfgItems" >> _tool >> "displayName")] call _fnc_reply;
};

private _item = getText (_cfg >> "item");
private _weight = getNumber (missionConfigFile >> "CfgItems" >> _item >> "weight") max 1;
private _room = {
    private _carried = [(_record getOrDefault [_faction + "_items", []]) select { _x isEqualTypeParams ["", 0] }] call ALife_fnc_itemsWeight;
    floor ((getNumber (missionConfigFile >> "CfgALife" >> "carryWeight") - _carried) / _weight)
};
if ((call _room) < 1) exitWith { "Your inventory is full." call _fnc_reply; };

private _time = getNumber (_cfg >> "time") max 1;
ALife_busy set [_uid, true];
["start", format ["Gathering %1...", getText (_cfg >> "name")], _time, "AinvPknlMstpSnonWnonDnon_medic0"] remoteExecCall ["ALife_fnc_resourceProgress", _owner];

[_unit, _uid, _faction, _record, _cfg, _item, _weight, _time, _room, _fnc_inZone, _fnc_reply, _owner] spawn {
    params ["_unit", "_uid", "_faction", "_record", "_cfg", "_item", "_weight", "_time", "_room", "_fnc_inZone", "_fnc_reply", "_owner"];
    sleep _time;
    ALife_busy deleteAt _uid;
    if (isNull _unit || { !alive _unit } || { _unit getVariable ["alife_restrained", false] } || { !(call _fnc_inZone) }) exitWith {
        "Gathering stopped: you moved away." call _fnc_reply;
    };
    getArray (_cfg >> "amount") params [["_min", 1], ["_max", 1]];
    private _amount = ((_min + floor random (_max - _min + 1)) min (call _room));
    if (_amount < 1) exitWith { "Your inventory is full." call _fnc_reply; };
    if (!([_uid, _faction, _item, _amount] call ALife_fnc_itemsChange)) exitWith { "That didn't save. Try again." call _fnc_reply; };
    [format ["+%1 %2", _amount, getText (missionConfigFile >> "CfgItems" >> _item >> "displayName")], "success"] call _fnc_reply;
    [_unit] call ALife_fnc_requestPhoneData;
};
