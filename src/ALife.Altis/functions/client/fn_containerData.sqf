/*
    ALife_fnc_containerData (client, called by the server)

    What's in the bag / trunk you have open. Opens the inventory screen, or
    refreshes it. "closed" means it's gone (an emptied bag is removed): the
    screen goes back to dropping onto the ground, or closes for a vehicle.

    Arguments:
        0: STRING - container id
        1: STRING - title
        2: STRING - "ground", "vehicle" or "closed"
        3: ARRAY  - items [[class, count], ...]
        4: NUMBER - cash
        5: NUMBER - weight used
        6: NUMBER - capacity
*/

params ["_id", "_title", "_kind", ["_items", []], ["_cash", 0], ["_weight", 0], ["_capacity", 0]];

// Your locker's items: the panel inside Arma's inventory (ALife_fnc_lockerPanel).
if (_kind == "locker") exitWith {
    ALife_lockerItems = [_id, _title, _kind, _items, _cash, _weight, _capacity];
    ["refresh"] call ALife_fnc_lockerPanel;
};

private _open = !isNull (findDisplay 5300);

if (_kind == "closed") exitWith {
    private _was = (missionNamespace getVariable ["ALife_container", []]) param [2, ""];
    if (_open && { _was == "ground" }) then {
        ["openGround"] call ALife_fnc_transfer;
    } else {
        ALife_container = nil;
        if (_open) then { closeDialog 0; };
    };
};

ALife_container = [_id, _title, _kind, _items, _cash, _weight, _capacity];
if (_open) then { ["refresh"] call ALife_fnc_transfer; } else { ["open"] call ALife_fnc_transfer; };
