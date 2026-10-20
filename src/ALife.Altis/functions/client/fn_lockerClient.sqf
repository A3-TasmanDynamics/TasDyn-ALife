/*
    ALife_fnc_lockerClient (client, called by the server)

    Your locker's gear crate is ready (ALife_fnc_locker): it's shown on your
    screen only, and Arma's inventory opens on it. Taking or putting
    anything saves it; closing the inventory saves it and the server takes
    the crate away.

        ["opened", crate]
*/

params [["_mode", "", [""]], ["_box", objNull, [objNull]]];

if (_mode != "opened" || { isNull _box } || { !hasInterface }) exitWith {};

ALife_lockerBox = _box;
_box hideObject false; // hidden for everyone else
if (dialog) then { closeDialog 0; };
player action ["Gear", _box];

// Saved on every change, and on closing (once).
if (!isNil "ALife_lockerHandlers") then { { player removeEventHandler _x; } forEach ALife_lockerHandlers; };
ALife_lockerHandlers = [
    ["Take", player addEventHandler ["Take", {
        params ["", "_container"];
        if (_container isEqualTo (missionNamespace getVariable ["ALife_lockerBox", objNull])) then { ["save", player] remoteExecCall ["ALife_fnc_locker", 2]; };
    }]],
    ["Put", player addEventHandler ["Put", {
        params ["", "_container"];
        if (_container isEqualTo (missionNamespace getVariable ["ALife_lockerBox", objNull])) then { ["save", player] remoteExecCall ["ALife_fnc_locker", 2]; };
    }]],
    ["InventoryClosed", player addEventHandler ["InventoryClosed", {
        params ["", "_container"];
        private _box = missionNamespace getVariable ["ALife_lockerBox", objNull];
        if (isNull _box || { !(_container isEqualTo _box) }) exitWith {};
        ALife_lockerBox = objNull;
        { player removeEventHandler _x; } forEach ALife_lockerHandlers;
        ALife_lockerHandlers = nil;
        ["close", player] remoteExecCall ["ALife_fnc_locker", 2];
    }]]
];
