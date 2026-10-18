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

// Shops for this faction: one "Open <shop>" action per placement (CfgShops),
// shown near any of its markers. Markets, clothing and vehicle shops for
// now; weapon shops come later.
{
    private _info = [configName _x] call ALife_fnc_shopInfo;
    if (_info isNotEqualTo []) then {
        _info params ["_type", "_place", "_shop"];
        if (_type in ["market", "clothing", "vehicles"] && { getText (_shop >> "side") == _faction }) then {
            private _spots = getArray (_place >> "markers") apply { getMarkerPos _x };
            player addAction [
                format ["<t color='#fbbf24'>Open %1</t>", getText (_shop >> "name")],
                {
                    private _place = _this select 3;
                    switch (getText (missionConfigFile >> "CfgShops" >> _place >> "type")) do {
                        case "clothing": { ["open", _place] call ALife_fnc_clothingShop; };
                        case "vehicles": { ["open", _place] call ALife_fnc_vehicleShop; };
                        default { ["open", _place] call ALife_fnc_shop; };
                    };
                },
                configName _place, 5, true, true, "",
                format ["alive player && { vehicle player == player } && { (%1 findIf { player distance2D _x <= %2 }) >= 0 }", _spots, getNumber (_place >> "radius")]
            ];
        };
    };
} forEach ("true" configClasses (missionConfigFile >> "CfgShops"));

["apply"] call ALife_fnc_phoneSettings;
[player] remoteExec ["ALife_fnc_requestPhoneData", 2];
