/*
    ALife_fnc_taser (client)

    Police tasers (CfgALife >> Taser): the taser rifle and pistol fire their
    taser cartridges (the yellow tracer magazines). A hit from one -- by a
    police officer, within range -- does no damage: you drop to the ground
    (unconscious) for `duration` seconds, then get up. Anyone else firing
    the same gun and magazine does normal damage.

    Modes:
        ["init"]           - setupUnit, for each unit you play: the damage hook
        [HandleDamage ...] - from the hook: the damage to apply, or nil (normal)
*/

if ((_this param [0, ""]) isEqualTo "init") exitWith {
    // The taser cartridges' bullets, from the game's config.
    ALife_taserAmmo = (getArray (missionConfigFile >> "CfgALife" >> "Taser" >> "magazines")) apply { toLower getText (configFile >> "CfgMagazines" >> _x >> "ammo") };
    player addEventHandler ["HandleDamage", { _this call ALife_fnc_taser }];
};

params ["_unit", "_selection", "_damage", "_source", "_projectile", "_hitIndex", "_instigator"];
private _cfg = missionConfigFile >> "CfgALife" >> "Taser";
private _shooter = [_instigator, _source] select (isNull _instigator);
if (isNull _shooter || { _shooter == _unit } || { !(toLower _projectile in (missionNamespace getVariable ["ALife_taserAmmo", []])) }) exitWith { nil };
if (side group _shooter != west || { !(currentWeapon _shooter in getArray (_cfg >> "weapons")) } || { _shooter distance _unit > getNumber (_cfg >> "range") }) exitWith { nil };

// A taser hit: no damage (this part's damage stays as it was), and down you go.
if (vehicle _unit == _unit && { !(_unit getVariable ["alife_tased", false]) } && { alive _unit }) then {
    _unit setVariable ["alife_tased", true, true];
    [_unit, name _shooter] spawn {
        params ["_unit", "_by"];
        _unit setUnconscious true;
        [format ["%1 tased you.", _by], "warning"] call ALife_fnc_toast;
        private _until = time + getNumber (missionConfigFile >> "CfgALife" >> "Taser" >> "duration");
        waitUntil { sleep 0.25; time >= _until || { !alive _unit } };
        if (alive _unit) then { _unit setUnconscious false; };
        _unit setVariable ["alife_tased", false, true];
    };
};
[_unit getHitIndex _hitIndex, damage _unit] select (_hitIndex < 0)
