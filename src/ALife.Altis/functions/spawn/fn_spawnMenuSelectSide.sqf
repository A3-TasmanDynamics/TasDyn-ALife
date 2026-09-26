/*
    File: fn_spawnMenuSelectSide.sqf
    Author: Tasman Dynamics

    Description:
        Handles a side (faction) button click in the spawn menu --
        repopulates the spawn point list for that faction and highlights
        the chosen button. Wired as each side button's action in
        dialog/spawnMenu.hpp.

        Selection state lives on the display itself (setVariable), not a
        mission-namespace global -- it only matters while this dialog is
        open, and disappears with it.

    Parameter(s):
        0: STRING - "civ" / "cop" / "medic"

    Returns:
        Nothing
*/

params [["_side", "civ", [""]]];

disableSerialization;

private _display = findDisplay 4700;
if (isNull _display) exitWith {};

_display setVariable ["alife_spawn_side", _side];
_display setVariable ["alife_spawn_point", []];

private _btnCiv = _display displayCtrl 4710;
private _btnCop = _display displayCtrl 4711;
private _btnMedic = _display displayCtrl 4712;

_btnCiv ctrlSetBackgroundColor [0.2, 0.5, 0.2, 1];
_btnCop ctrlSetBackgroundColor [0.2, 0.2, 0.6, 1];
_btnMedic ctrlSetBackgroundColor [0.6, 0.2, 0.2, 1];

switch (_side) do {
    case "civ": { _btnCiv ctrlSetBackgroundColor [0.3, 0.75, 0.3, 1]; };
    case "cop": { _btnCop ctrlSetBackgroundColor [0.3, 0.3, 0.85, 1]; };
    case "medic": { _btnMedic ctrlSetBackgroundColor [0.85, 0.3, 0.3, 1]; };
};

private _listCtrl = _display displayCtrl 4720;
lbClear _listCtrl;

{
    _x params ["_key", "_displayName"];
    private _index = _listCtrl lbAdd _displayName;
    _listCtrl lbSetData [_index, _key];
} forEach ([_side] call ALife_fnc_getSpawnPoints);

{
    deleteMarkerLocal _x;
} forEach (allMapMarkers select { _x find "alife_spawn_marker" == 0 });

private _infoCtrl = _display displayCtrl 4740;
_infoCtrl ctrlSetStructuredText parseText "Select a spawn point from the list.";
