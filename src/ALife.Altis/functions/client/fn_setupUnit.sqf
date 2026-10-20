/*
    ALife_fnc_setupUnit (client)

    Runs each time you're placed in the world (spawn menu or revive) --
    a respawn gives you a new unit, so this sets it up again:
      - the death screen when this unit goes down
      - group looks (uniform textures, hidden backpacks)
      - your view settings (phone -> Settings)
*/

if (!hasInterface) exitWith {};
// Once per unit. Kept in a global, not on the unit: Arma copies a unit's
// variables onto the new one when you respawn (so a flag on the unit would
// skip the setup for every life after the first).
if (!isNil "ALife_setUpUnit" && { ALife_setUpUnit isEqualTo player }) exitWith {};
ALife_setUpUnit = player;

ALife_spawned = true;

player addEventHandler ["Killed", {
    params ["_unit", "_killer", "_instigator"];
    ALife_spawned = false;
    if (dialog) then { closeDialog 0; };
    ["start", _unit, _killer, _instigator] call ALife_fnc_deathScreen;
}];

// No scroll-wheel actions: reviving, shops and vehicles are all on the
// Windows key (ALife_fnc_interact).

// Group looks (uniform textures, hidden cop / medic backpacks): now,
// whenever the inventory changes, and every few seconds in case the gear
// changed some other way.
{
    player addEventHandler [_x, { [true] spawn ALife_fnc_applySkins; }];
} forEach ["Take", "Put", "InventoryClosed"];
[true] call ALife_fnc_applySkins;
[] spawn {
    private _unit = player;
    while { alive _unit && { _unit == player } } do {
        sleep 3;
        [] call ALife_fnc_applySkins;
    };
};

["apply"] call ALife_fnc_phoneSettings;
[player] remoteExec ["ALife_fnc_requestPhoneData", 2];
