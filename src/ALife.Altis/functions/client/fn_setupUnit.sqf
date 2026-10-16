/*
    ALife_fnc_setupUnit (client)

    Runs each time you're placed in the world (spawn menu or revive) --
    a respawn gives you a new unit, so this sets it up again:
      - the death screen when this unit goes down
      - medics: the Revive action on downed players
      - shop actions for your faction (e.g. Medical Supplies at hospitals)
      - your view settings (phone -> Settings)
*/

if (!hasInterface) exitWith {};
if (player getVariable ["alife_setUp", false]) exitWith {};
player setVariable ["alife_setUp", true];

ALife_spawned = true;
private _faction = missionNamespace getVariable ["ALife_faction", "civ"];

player addEventHandler ["Killed", {
    params ["_unit", "_killer", "_instigator"];
    ALife_spawned = false;
    if (dialog) then { closeDialog 0; };
    ["start", _unit, _killer, _instigator] call ALife_fnc_deathScreen;
}];

if (_faction == "medic") then {
    player addAction [
        "<t color='#8ff0a4'>Revive</t>",
        { ["start", cursorObject] spawn ALife_fnc_revive; },
        nil, 10, true, true, "",
        "alive player && { vehicle player == player } && { isNil 'ALife_reviving' } && { !isNull cursorObject } && { cursorObject getVariable ['alife_downed', false] } && { player distance cursorObject < 3.5 }"
    ];
};

// Shops for this faction: [class, position, radius, name] for each marker.
ALife_shopSpots = [];
{
    private _shop = _x;
    if (getText (_shop >> "faction") == _faction) then {
        {
            ALife_shopSpots pushBack [configName _shop, getMarkerPos _x, getNumber (_shop >> "radius"), getText (_shop >> "displayName")];
        } forEach getArray (_shop >> "markers");
    };
} forEach ("true" configClasses (missionConfigFile >> "CfgShops"));

if (ALife_shopSpots isNotEqualTo []) then {
    player addAction [
        "<t color='#f5a524'>Open shop</t>",
        {
            private _spot = ALife_shopSpots select (ALife_shopSpots findIf { player distance2D (_x select 1) <= (_x select 2) });
            ["open", _spot select 0] call ALife_fnc_shop;
        },
        nil, 5, true, true, "",
        "alive player && { vehicle player == player } && { (ALife_shopSpots findIf { player distance2D (_x select 1) <= (_x select 2) }) >= 0 }"
    ];
};

["apply"] call ALife_fnc_phoneSettings;
[player] remoteExec ["ALife_fnc_requestPhoneData", 2];
