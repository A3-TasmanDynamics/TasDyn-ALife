/*
    ALife_fnc_requestProcess (server, called by the client: a processor in
    the Windows-key menu)

    Processes everything the player carries for one processor
    (CfgResources.hpp >> CfgProcessing): as many batches of from[] as they
    have, into to[]. Free with the processor's licence; otherwise `fee` per
    batch, from cash. The server times it (`time` per batch, at most
    `maxTime`; the player sees a progress bar: ALife_fnc_resourceProgress),
    checks again that they're still there and still have the items, then
    swaps them and takes the fee.

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - CfgProcessing class
*/

params [["_unit", objNull, [objNull]], ["_process", "", [""]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
private _cfg = missionConfigFile >> "CfgProcessing" >> _process;

if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" } || { !isClass _cfg }) exitWith {};
if (_unit getVariable ["alife_restrained", false]) exitWith {};
if (isNil "ALife_busy") then { ALife_busy = createHashMap; };
if (ALife_busy getOrDefault [_uid, false]) exitWith {};

private _fnc_here = { (getArray (_cfg >> "markers") findIf { _x in allMapMarkers && { _unit inArea _x } }) >= 0 };
if !(call _fnc_here) exitWith { "You're not at the processor." call _fnc_reply; };

private _from = getArray (_cfg >> "from") select { _x isEqualTypeParams ["", 0] && { (_x select 1) > 0 } };
private _to = getArray (_cfg >> "to") select { _x isEqualTypeParams ["", 0] && { (_x select 1) > 0 } };
private _itemName = { getText (missionConfigFile >> "CfgItems" >> _this >> "displayName") };
// How many batches they can make right now.
private _fnc_batches = {
    private _items = createHashMapFromArray ((_record getOrDefault [_faction + "_items", []]) select { _x isEqualTypeParams ["", 0] });
    private _n = 1e6;
    { _n = _n min floor ((_items getOrDefault [_x select 0, 0]) / (_x select 1)); } forEach _from;
    _n
};
private _batches = call _fnc_batches;
if (_from isEqualTo [] || { _batches < 1 }) exitWith {
    format ["You need %1 to process here.", (_from apply { format ["%1 %2", _x select 1, (_x select 0) call _itemName] }) joinString " and "] call _fnc_reply;
};

private _licence = getText (_cfg >> "licence");
private _licensed = _licence == "" || { _licence in ((_record getOrDefault [_faction + "_licence", []]) select { _x isEqualType "" }) };
private _fee = if (_licensed) then { 0 } else { getNumber (_cfg >> "fee") };
if (_fee * _batches > (_record getOrDefault [_faction + "_cash", 0])) exitWith {
    format ["Without a %1 it costs $%2 a batch: you need $%3 cash.",
        getText (missionConfigFile >> "CfgLicences" >> _licence >> "displayName"), _fee, [_fee * _batches] call BIS_fnc_numberText] call _fnc_reply;
};

private _time = ((getNumber (_cfg >> "time") * _batches) min (getNumber (_cfg >> "maxTime") max 1)) max 1;
ALife_busy set [_uid, true];
["start", format ["%1: processing %2...", getText (_cfg >> "name"), _batches], _time, "AinvPknlMstpSnonWnonDnon_medic1"] remoteExecCall ["ALife_fnc_resourceProgress", _owner];

[_unit, _uid, _faction, _record, _cfg, _from, _to, _batches, _fee, _time, _fnc_here, _fnc_batches, _fnc_reply, _itemName, _owner] spawn {
    params ["_unit", "_uid", "_faction", "_record", "_cfg", "_from", "_to", "_batches", "_fee", "_time", "_fnc_here", "_fnc_batches", "_fnc_reply", "_itemName", "_owner"];
    sleep _time;
    ALife_busy deleteAt _uid;
    if (isNull _unit || { !alive _unit } || { _unit getVariable ["alife_restrained", false] } || { !(call _fnc_here) }) exitWith {
        "Processing stopped: you moved away." call _fnc_reply;
    };
    _batches = _batches min (call _fnc_batches); // they may have dropped some meanwhile
    if (_batches < 1) exitWith { "You don't have the items any more." call _fnc_reply; };

    if (_fee > 0) then {
        private _paid = [_uid, _faction, _record, _fee * _batches, "cash", getText (_cfg >> "name")] call ALife_fnc_shopCharge;
        if (_paid != "OK") then { _batches = 0; };
    };
    if (_batches < 1) exitWith { "Not enough cash for the processing fee." call _fnc_reply; };

    // Take the inputs, then give the outputs (each change is saved).
    private _ok = true;
    { if (_ok) then { _ok = [_uid, _faction, _x select 0, -((_x select 1) * _batches)] call ALife_fnc_itemsChange; }; } forEach _from;
    if (!_ok) exitWith {
        diag_log format ["[ALife] %1 processing %2: taking the inputs failed (fee $%3 paid)", _uid, configName _cfg, _fee * _batches];
        "Processing didn't go through. Tell staff (it's in the server log)." call _fnc_reply;
    };
    { [_uid, _faction, _x select 0, (_x select 1) * _batches] call ALife_fnc_itemsChange; } forEach _to;

    diag_log format ["[ALife] %1 processed %2 x%3 at %4 (fee $%5)", _uid, configName _cfg, _batches, getText (_cfg >> "name"), _fee * _batches];
    [format ["Made %1%2.", (_to apply { format ["%1 %2", (_x select 1) * _batches, (_x select 0) call _itemName] }) joinString ", ",
        ["", format [" Fee: $%1 (no licence).", [_fee * _batches] call BIS_fnc_numberText]] select (_fee > 0)], "success"] call _fnc_reply;
    [_unit] call ALife_fnc_requestPhoneData;
};
