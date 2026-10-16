/*
    ALife_fnc_interact (client, the Windows key)

    Looking at an ATM (any on the map): the bank menu. Looking at a dropped
    bag or a vehicle close by: opens it in the inventory screen (the server
    checks it). Looking at nothing: opens your own inventory with the ground
    beside it, so you can drop things.

    Returns:
        BOOL - true if the key press was used
*/

if (!alive player || { !(missionNamespace getVariable ["ALife_spawned", false]) } || { dialog }) exitWith { false };

private _target = cursorObject;

if ([_target] call ALife_fnc_isAtm && { player distance _target <= getNumber (missionConfigFile >> "CfgALife" >> "atmRange") + 1 }) exitWith {
    ["open"] call ALife_fnc_atm;
    true
};

private _isBag = !isNull _target && { _target in (missionNamespace getVariable ["ALife_groundBags", []]) };
private _isVehicle = !isNull _target && { alive _target } && { _target isKindOf "LandVehicle" || { _target isKindOf "Air" } || { _target isKindOf "Ship" } };

if ((_isBag && { player distance _target < 4 }) || { _isVehicle && { player distance _target < 8 } }) then {
    [player, _target] remoteExecCall ["ALife_fnc_containerOpen", 2];
} else {
    ["openGround"] call ALife_fnc_transfer;
};
true
