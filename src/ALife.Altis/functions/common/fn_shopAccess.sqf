/*
    ALife_fnc_shopAccess (common)

    Whether a player may use a shop, or buy one of its entries: checks the
    config's `licence` and `level` (CfgShops.hpp). Used by the shop window
    to grey things out and by the server to refuse them.

    Arguments:
        0: CONFIG - a CfgShops shop, or an entry in its Items / Gear / Vehicles
        1: ARRAY  - the player's licences (CfgLicences keys)
        2: NUMBER - the player's rank (cop_level / medic_level; 0 for civs)

    Returns:
        STRING - "" if allowed, otherwise what's missing (e.g. "Firearms Licence")
*/

params [["_cfg", configNull, [configNull]], ["_licences", [], [[]]], ["_level", 0, [0]]];

private _licence = getText (_cfg >> "licence");
if (_licence != "" && { !(_licence in _licences) }) exitWith {
    private _name = getText (missionConfigFile >> "CfgLicences" >> _licence >> "displayName");
    [_name, _licence] select (_name == "")
};

private _need = getNumber (_cfg >> "level");
if (_level < _need) exitWith { format ["Rank %1", _need] };

""
