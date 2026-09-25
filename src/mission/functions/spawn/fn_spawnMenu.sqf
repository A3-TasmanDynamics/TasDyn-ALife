/*
    File: fn_spawnMenu.sqf
    Author: Tasman Dynamics

    Description:
        Opens the spawn selection dialog and populates the faction list.
        Client-side — this is the target of a remoteExec from the server
        (initPlayerServer.sqf) once this player's own record has loaded
        successfully. Never called for anyone else's client.

    Parameter(s):
        None

    Returns:
        Nothing
*/

closeDialog 0;
createDialog "ALife_SpawnMenu";

private _display = findDisplay 4700;
private _factionList = _display displayCtrl 4701;

lbClear _factionList;
{
    _x params ["_key", "_label"];
    private _index = _factionList lbAdd _label;
    _factionList lbSetData [_index, _key];
} forEach [
    ["civilian", "Civilian"],
    ["police", "Police"],
    ["medic", "Medic"]
];
