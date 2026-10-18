/*
    ALife_fnc_vehicleSkin (common)

    The textures (one per hidden selection) for a vehicle skin choice.

    Arguments:
        0: STRING - vehicle classname
        1: STRING - "tex:<row>": a row of the mission's CfgVehicles textures[]
                    ({"name", "side", {textures}});
                    "src:<class>": one of the vehicle's own TextureSources;
                    "": none (as it comes)

    Returns:
        ARRAY - texture paths, [] for none
*/

params [["_class", "", [""]], ["_choice", "", [""]]];

private _textures = [];
if (_choice select [0, 4] == "tex:") then {
    private _rows = getArray (missionConfigFile >> "CfgVehicles" >> _class >> "textures");
    private _i = parseNumber (_choice select [4]);
    if (_i >= 0 && { _i < count _rows }) then { _textures = (_rows select _i) param [2, []]; };
};
if (_choice select [0, 4] == "src:") then {
    _textures = getArray (configFile >> "CfgVehicles" >> _class >> "TextureSources" >> (_choice select [4]) >> "textures");
};
_textures
