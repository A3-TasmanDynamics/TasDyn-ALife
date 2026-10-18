/*
    ALife_fnc_containerSend (server)

    Sends a player what's in a container (ALife_fnc_containerData) and their
    own items and cash (ALife_fnc_phoneData). An id that no longer exists
    sends "closed".

    Arguments:
        0: OBJECT - the player
        1: STRING - container id
*/

params ["_unit", "_id"];

private _container = (missionNamespace getVariable ["ALife_containers", createHashMap]) getOrDefault [_id, createHashMap];

if (count _container == 0) then {
    ["", "", "closed"] remoteExecCall ["ALife_fnc_containerData", owner _unit];
} else {
    private _stock = _container get "items";
    private _items = (keys _stock) apply { [_x, _stock get _x] };
    [_id, _container get "title", _container get "kind", _items, _container get "cash",
        [_items] call ALife_fnc_itemsWeight, _container get "capacity"] remoteExecCall ["ALife_fnc_containerData", owner _unit];
};
[_unit] call ALife_fnc_requestPhoneData;
