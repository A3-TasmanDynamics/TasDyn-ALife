/*
    ALife_fnc_lockerClient (client, called by the server)

    Your locker is ready (ALife_fnc_locker): its gear crate is shown on
    your screen only, Arma's inventory opens on it, and the virtual items
    panel is added to the same screen (ALife_fnc_lockerPanel). Taking or
    putting anything saves it; closing the inventory saves it and the
    server takes the crate away.

        ["opened", crate]
*/

params [["_mode", "", [""]], ["_box", objNull, [objNull]]];

if (_mode != "opened" || { isNull _box } || { !hasInterface }) exitWith {};

ALife_lockerBox = _box;
_box hideObject false; // hidden for everyone else
if (dialog) then { closeDialog 0; };
player action ["Gear", _box];
[] spawn {
    private _until = diag_tickTime + 5;
    waitUntil { uiSleep 0.05; !isNull (findDisplay 602) || { diag_tickTime > _until } };
    ["build"] call ALife_fnc_lockerPanel;
};

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
