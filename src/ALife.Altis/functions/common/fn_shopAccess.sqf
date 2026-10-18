/*
    ALife_fnc_shopAccess (common)

    Whether a player may use a shop, or buy one of its entries: checks the
    config's `licence` and `level`. Used by the shop window
    to grey things out and by the server to refuse them.

    Arguments:
        0: CONFIG or ARRAY - a shop or CfgMarket item (its licence / level
                    fields), or a Tonic row's [licence, level]
        1: ARRAY  - the player's licences (CfgLicences keys)
        2: NUMBER - the player's rank (cop_level / medic_level; 0 for civs)

    Returns:
        STRING - "" if allowed, otherwise what's missing (e.g. "Firearms Licence")
*/

params [["_cfg", configNull, [configNull, []]], ["_licences", [], [[]]], ["_level", 0, [0]]];

private _need = 0;
private _licence = "";
if (_cfg isEqualType []) then {
    _cfg params [["_l", "", [""]], ["_n", 0, [0]]];
    _licence = _l;
    _need = _n;
} else {
    _licence = getText (_cfg >> "licence");
    _need = getNumber (_cfg >> "level");
};
if (_licence != "" && { !(_licence in _licences) }) exitWith {
    private _name = getText (missionConfigFile >> "CfgLicences" >> _licence >> "displayName");
    [_name, _licence] select (_name == "")
};

if (_level < _need) exitWith { format ["Rank %1", _need] };

""
