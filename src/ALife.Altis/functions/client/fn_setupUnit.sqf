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
        "<t color='#22c55e'>Revive</t>",
        { ["start", cursorObject] spawn ALife_fnc_revive; },
        nil, 10, true, true, "",
        "alive player && { vehicle player == player } && { isNil 'ALife_reviving' } && { !isNull cursorObject } && { cursorObject getVariable ['alife_downed', false] } && { player distance cursorObject < 3.5 }"
    ];
};

// Shops for this faction: one "Open <shop>" action each, shown near any of
// its markers. Only shops that sell virtual items (items[]) for now:
// vehicles, weapons and clothing can't be bought yet (see CfgShops.hpp).
{
    private _shop = _x;
    if (getText (_shop >> "faction") == _faction && { getArray (_shop >> "items") isNotEqualTo [] }) then {
        private _spots = getArray (_shop >> "markers") apply { getMarkerPos _x };
        player addAction [
            format ["<t color='#fbbf24'>Open %1</t>", getText (_shop >> "displayName")],
            { ["open", _this select 3] call ALife_fnc_shop; },
            configName _shop, 5, true, true, "",
            format ["alive player && { vehicle player == player } && { (%1 findIf { player distance2D _x <= %2 }) >= 0 }", _spots, getNumber (_shop >> "radius")]
        ];
    };
} forEach ("true" configClasses (missionConfigFile >> "CfgShops"));

["apply"] call ALife_fnc_phoneSettings;
[player] remoteExec ["ALife_fnc_requestPhoneData", 2];
