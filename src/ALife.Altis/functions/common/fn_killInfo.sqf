/*
    ALife_fnc_killInfo

    Who killed someone, with what, and from how far -- for the death
    screen and the server log.

    Arguments (as from a Killed / EntityKilled event):
        0: OBJECT - victim
        1: OBJECT - killer (may be a vehicle)
        2: OBJECT - instigator (the person who pulled the trigger / drove)

    Returns:
        ARRAY [name, weapon, distance in metres]
        name is "" when nobody else was involved (fall, drowning, suicide).
*/

params ["_victim", ["_killer", objNull], ["_instigator", objNull]];

private _source = [_killer, _instigator] select (!isNull _instigator);
if (isNull _source || { _source == _victim }) exitWith { ["", "", -1] };

// A vehicle as the killer: credit whoever was in charge of it.
private _person = [effectiveCommander _source, _source] select (_source isKindOf "CAManBase");
private _name = if (isNull _person) then { "Unknown" } else { name _person };

private _weapon = "";
private _vehicle = if (_source isKindOf "CAManBase") then { vehicle _person } else { _source };
if (!isNull _vehicle && { _vehicle != _person }) then {
    _weapon = getText (configFile >> "CfgVehicles" >> typeOf _vehicle >> "displayName");
} else {
    private _class = currentWeapon _person;
    if (_class != "") then {
        _weapon = getText (configFile >> "CfgWeapons" >> _class >> "displayName");
    };
};
if (_weapon == "") then { _weapon = "Unknown" };

[_name, _weapon, round (_victim distance _source)]
