/*
    ALife_fnc_personApply (client, called by the server)

    Applies a police action to you, on your own machine (your unit is
    local here); ALife_fnc_personAction has already checked it.

        ["strip", officer name] - weapons, vest, backpack, hat, glasses and
                                  everything in your uniform are seized
        ["putIn", officer, vehicle] - into a back seat (restrained)
        ["takeOut", officer]        - out of the vehicle (restrained)
*/

params [["_mode", "", [""]], ["_by", "", [""]], ["_vehicle", objNull, [objNull]]];

if (!hasInterface || { !alive player }) exitWith {};

switch (_mode) do {
    case "putIn": {
        if (isNull _vehicle) exitWith {};
        if (!isNull attachedTo player) then { detach player; };
        player moveInCargo _vehicle;
        [format ["%1 put you in the vehicle.", _by], "warning"] call ALife_fnc_toast;
    };
    case "takeOut": {
        if (vehicle player == player) exitWith {};
        moveOut player;
        [format ["%1 took you out of the vehicle.", _by], "warning"] call ALife_fnc_toast;
    };
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
