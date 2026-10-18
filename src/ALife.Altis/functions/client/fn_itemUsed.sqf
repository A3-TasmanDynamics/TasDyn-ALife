/*
    ALife_fnc_itemUsed (client, called by the server)

    Applies a used item where the object is local (ALife_fnc_requestUseItem
    has already checked it and used the item up):

        ["heal", unit, amount, energy] - health back (0-1); energy: stamina reset
        ["fuel", vehicle, amount]      - add fuel (0-1 of a tank)
*/

params [["_mode", "", [""]], ["_object", objNull, [objNull]], ["_amount", 0, [0]], ["_energy", false, [false]]];

if (isNull _object || { !local _object } || { _amount < 0 }) exitWith {};

switch (_mode) do {
    case "heal": {
        if (_object != player || { !alive player }) exitWith {};
        player setDamage ((damage player - _amount) max 0);
        if (_energy) then { player setFatigue 0; };
    };
    case "fuel": {
        _object setFuel ((fuel _object + _amount) min 1);
    };
};
