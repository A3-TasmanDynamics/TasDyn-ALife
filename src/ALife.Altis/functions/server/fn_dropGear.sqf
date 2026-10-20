/*
    ALife_fnc_dropGear (server)

    Puts a player's Arma gear on the ground where they stand (police strip
    them, ALife_fnc_personAction "strip"): every weapon with its
    attachments and loaded magazine, the vest, backpack, headgear, goggles,
    binoculars and linked items (map, GPS, radio, ...), and everything in
    their uniform, vest and backpack -- in one pile anyone can take from
    (Arma's inventory). They keep wearing their uniform. The caller takes
    it off them (ALife_fnc_personApply "strip", on their machine). The pile
    clears after CfgALife >> bagLifetime, like a dropped bag.

    Arguments:
        0: OBJECT - the player

    Returns:
        OBJECT - the pile (objNull if they had nothing)
*/

params [["_unit", objNull, [objNull]]];

if (!isServer || { isNull _unit }) exitWith { objNull };

getUnitLoadout _unit params [
    ["_primary", []], ["_launcher", []], ["_handgun", []],
    ["_uniform", []], ["_vest", []], ["_backpack", []],
    ["_headgear", ""], ["_goggles", ""], ["_binoculars", []], ["_linked", []]
];

private _pos = getPosATL _unit;
private _pile = createVehicle ["GroundWeaponHolder", [_pos select 0, _pos select 1, 0], [], 0, "CAN_COLLIDE"];
private _any = false;

// A weapon as getUnitLoadout gives it: [class, muzzle, pointer, optic, [mag, ammo], [mag, ammo], bipod].
private _fnc_weapon = {
    if (_this isEqualType [] && { count _this > 0 } && { (_this select 0) isEqualType "" } && { (_this select 0) != "" }) then {
        _pile addWeaponWithAttachmentsCargoGlobal [_this, 1];
        _any = true;
    };
};
// A container's contents: [class, count], [magazine, count, ammo], [[weapon...], count].
private _fnc_contents = {
    {
        if ((_x param [0, ""]) isEqualType []) then {
            for "_i" from 1 to (_x param [1, 1]) do { (_x select 0) call _fnc_weapon; };
        } else {
            _x params [["_class", ""], ["_count", 1], ["_ammo", -1]];
            if (_class isEqualType "" && { _class != "" }) then {
                if (_ammo isEqualType 0 && { _ammo >= 0 } && { isClass (configFile >> "CfgMagazines" >> _class) }) then {
                    _pile addMagazineAmmoCargo [_class, _count, _ammo];
                } else {
                    if (_ammo isEqualType true) then { _count = 1; }; // a backpack inside: [class, true]
                    if (isClass (configFile >> "CfgVehicles" >> _class)) then { _pile addBackpackCargoGlobal [_class, _count]; } else { _pile addItemCargoGlobal [_class, _count]; };
                };
                _any = true;
            };
        };
    } forEach _this;
};

{ _x call _fnc_weapon; } forEach [_primary, _launcher, _handgun, _binoculars];
// Uniform: they keep it on; its contents drop.
if (_uniform isNotEqualTo []) then { (_uniform param [1, []]) call _fnc_contents; };
if (_vest isNotEqualTo []) then {
    _pile addItemCargoGlobal [_vest select 0, 1];
    (_vest param [1, []]) call _fnc_contents;
    _any = true;
};
if (_backpack isNotEqualTo []) then {
    _pile addBackpackCargoGlobal [_backpack select 0, 1];
    (_backpack param [1, []]) call _fnc_contents;
    _any = true;
};
{ if (_x isEqualType "" && { _x != "" }) then { _pile addItemCargoGlobal [_x, 1]; _any = true; }; } forEach ([_headgear, _goggles] + _linked);

if (!_any) exitWith { deleteVehicle _pile; objNull };
_pile spawn {
    sleep getNumber (missionConfigFile >> "CfgALife" >> "bagLifetime");
    if (!isNull _this) then { deleteVehicle _this; };
};
_pile
