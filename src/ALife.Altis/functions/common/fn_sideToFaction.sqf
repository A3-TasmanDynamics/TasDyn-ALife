/*
    ALife_fnc_sideToFaction

    The slot's side decides the faction:
        BLUFOR (west)      -> "cop"
        Independent        -> "medic"
        anything else      -> "civ"

    Arguments:
        0: SIDE

    Returns:
        STRING
*/

params ["_side"];

switch (_side) do {
    case west: { "cop" };
    case independent: { "medic" };
    default { "civ" };
};
