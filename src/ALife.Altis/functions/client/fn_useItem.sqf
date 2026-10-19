/*
    ALife_fnc_useItem (client, scheduled)

    The Use button (phone Inventory, the Windows-key inventory) and the
    vehicle menu (Repair / Refuel / Lockpick): uses one
    of a virtual item, as its CfgItems `use` says -- eat / drink, heal
    yourself, repair / refuel / lockpick the vehicle you're next to. A
    progress bar runs for its `useTime` (ui\progress.hpp); for anything
    but eating and drinking, moving away cancels it. Then the server checks
    everything again, uses the item up and applies it
    (ALife_fnc_requestUseItem).

    Arguments:
        0: STRING - item class (CfgItems)
        1: OBJECT - the vehicle (optional; else the one you're looking at
                    or the nearest within 6 m)
*/

params [["_class", "", [""]], ["_vehicle", objNull, [objNull]]];

if (!hasInterface || { !isNil "ALife_using" } || { !isNil "ALife_reviving" }) exitWith {};
private _cfg = missionConfigFile >> "CfgItems" >> _class;
private _use = getText (_cfg >> "use");
private _name = getText (_cfg >> "displayName");

if (_class == "cash") exitWith { ["Cash can't be used: give it or drop it.", "warning"] call ALife_fnc_toast; };
if !(_use in ["consume", "heal", "repair", "refuel", "lockpick"]) exitWith { [format ["%1 can't be used from the inventory.", _name], "warning"] call ALife_fnc_toast; };
if (!alive player || { player getVariable ["alife_downed", false] }) exitWith {};

private _have = 0;
{ if ((_x select 0) == _class) then { _have = _x select 1; }; } forEach ((missionNamespace getVariable ["ALife_phone", createHashMap]) getOrDefault ["items", []]);
if (_have < 1) exitWith { [format ["You need a %1 for that.", _name], "warning"] call ALife_fnc_toast; };

// The vehicle you're looking at, or the nearest one within 5 m.
private _target = objNull;
if (_use in ["repair", "refuel", "lockpick"]) then {
    if (vehicle player != player) exitWith {};
    private _types = ["LandVehicle", "Air", "Ship"];
    if (!isNull _vehicle && { player distance _vehicle <= 8 }) exitWith { _target = _vehicle; };
    if (!isNull cursorObject && { (_types findIf { cursorObject isKindOf _x }) >= 0 } && { player distance cursorObject <= 6 }) then {
        _target = cursorObject;
    } else {
        _target = (nearestObjects [player, _types, 6]) param [0, objNull];
    };
};
if (_use in ["repair", "refuel", "lockpick"] && { isNull _target }) exitWith {
    [if (vehicle player != player) then { "Get out of the vehicle first." } else { "Stand next to a vehicle to use that." }, "warning"] call ALife_fnc_toast;
};
private _vehName = if (isNull _target) then { "" } else { getText (configFile >> "CfgVehicles" >> typeOf _target >> "displayName") };
private _refuse = switch (_use) do {
    case "heal": { ["", "You're not hurt."] select (damage player < 0.01) };
    case "repair": { ["", format ["The %1 isn't damaged.", _vehName]] select (damage _target < 0.01 && { ((getAllHitPointsDamage _target) param [2, []]) findIf { _x > 0.01 } < 0 }) };
    case "refuel": { ["", format ["The %1 is full.", _vehName]] select (fuel _target > 0.99) };
    case "lockpick": { ["", format ["The %1 isn't locked.", _vehName]] select (locked _target < 2) };
    default { "" };
};
if (_refuse != "") exitWith { [_refuse, "warning"] call ALife_fnc_toast; };

if (dialog) then { closeDialog 0; };
ALife_using = true;
private _label = switch (_use) do {
    case "consume": { format ["Using %1...", _name] };
    case "heal": { "Patching yourself up..." };
    case "repair": { format ["Repairing the %1...", _vehName] };
    case "refuel": { format ["Refuelling the %1...", _vehName] };
    case "lockpick": { format ["Picking the lock of the %1...", _vehName] };
};
private _duration = getNumber (_cfg >> "useTime") max 0.5;
private _still = _use != "consume"; // eating and drinking you can do on the move
private _anim = getText (_cfg >> "anim"); // CfgItems: a looped kneeling move, "" = none
private _start = getPosATL player;
private _began = diag_tickTime;
private _done = false;
private _outSince = -1; // when they left the medic moves (-1: in them)

"ALife_progress" cutRsc ["ALife_Progress", "PLAIN", 0, false];
if (_anim != "") then { player playMoveNow _anim; };

while { true } do {
    private _elapsed = diag_tickTime - _began;
    private _display = uiNamespace getVariable ["ALife_Progress", displayNull];
    if (!isNull _display) then {
        (_display displayCtrl 9201) ctrlSetText _label;
        (_display displayCtrl 9202) progressSetPosition (_elapsed / _duration);
    };
    if (_elapsed >= _duration) exitWith { _done = true; };
    if (!alive player || { player getVariable ["alife_downed", false] }) exitWith {};
    if (_still && { player distance _start > 1.5 || { !isNull _target && { player distance _target > 8 } } }) exitWith {};
    // Started once (it loops); again only if something knocked them out of
    // the kneeling medic moves for 2 s -- re-starting it while it's still
    // blending in from standing makes it jitter.
    if (_anim != "") then {
        if ((animationState player) find "ainvpknlmstpsnonwnondnon_medic" == 0) then {
            _outSince = -1;
        } else {
            if (_outSince < 0) then { _outSince = diag_tickTime; };
            if (diag_tickTime - _outSince > 2) then { player playMoveNow _anim; _outSince = diag_tickTime; };
        };
    };
    sleep 0.1;
};

"ALife_progress" cutText ["", "PLAIN"];
// Stand back up with the game's own end move (switchMove would only change it
// on this machine and snap back).
if (_anim != "" && { alive player }) then { player playMoveNow "AinvPknlMstpSnonWnonDnon_medicEnd"; };
ALife_using = nil;

if (_done) then {
    [player, _class, _target] remoteExec ["ALife_fnc_requestUseItem", 2];
} else {
    ["Cancelled.", "warning"] call ALife_fnc_toast;
};
