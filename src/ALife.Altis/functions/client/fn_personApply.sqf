/*
    ALife_fnc_personApply (client, called by the server)

    Applies a police action to you, on your own machine (your unit is
    local here); ALife_fnc_personAction has already checked it.

        ["strip", officer name] - weapons, vest, backpack, hat, glasses and
                                  everything in your uniform are seized
*/

params [["_mode", "", [""]], ["_by", "", [""]]];

if (!hasInterface || { !alive player }) exitWith {};

switch (_mode) do {
    case "strip": {
        removeAllWeapons player;
        { player removeItemFromUniform _x; } forEach uniformItems player;
        removeVest player;
        removeBackpack player;
        removeHeadgear player;
        removeGoggles player;
        [true] call ALife_fnc_applySkins;
        [format ["%1 took your weapons and gear.", _by], "warning"] call ALife_fnc_toast;
    };
};
