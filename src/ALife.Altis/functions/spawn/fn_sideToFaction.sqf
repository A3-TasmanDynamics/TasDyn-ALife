/*
    File: fn_sideToFaction.sqf
    Author: Tasman Dynamics

    Description:
        Maps an Arma engine side to this project's faction string
        ("civ"/"cop"/"medic") -- the one source of truth for that mapping.
        Matches Tonic's AsYetUntitled/Framework convention (west = police,
        independent = medic, civilian = civ) rather than a custom in-dialog
        faction picker -- see docs/TONIC_REFERENCE.md §3 for why. Used both
        client-side (fn_spawnMenu.sqf, to pick which spawn points to show)
        and server-side (fn_spawnPlayer.sqf, to authoritatively determine a
        connecting player's faction from their actual assigned side rather
        than trusting a client-supplied faction string).

        Runs identically on client or server -- pure function of the given
        side, no privileged data.

    Parameter(s):
        0: SIDE - the unit's side (west / independent / civilian)

    Returns:
        STRING - "cop" / "medic" / "civ" (civilian is also the default for
        any other side, e.g. a fresh unit not yet assigned)
*/

params ["_side"];

switch (_side) do {
    case west: { "cop" };
    case independent: { "medic" };
    default { "civ" };
};
