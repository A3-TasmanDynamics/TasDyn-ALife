/*
    ALife_fnc_placeMode (client, scheduled)

    Choosing where a police object goes: a copy of it follows in front of
    you (and turns with you) until you press Space to put it there, or
    Backspace to cancel. Q / E turn it, Page Up / Page Down move it nearer
    or further (CfgALife >> Placeables). Used when you Use a placeable item
    (ALife_fnc_useItem) and to move one that's already down (the Windows
    menu on it -- the real one is hidden on your screen meanwhile).

    Arguments:
        0: STRING - the object's class (CfgItems >> object)
        1: OBJECT - moving: the placed object (optional)

    Returns:
        ARRAY - [positionATL, direction], or [] if cancelled
*/

params [["_class", "", [""]], ["_existing", objNull, [objNull]]];

if (!hasInterface || { _class == "" } || { !isNil "ALife_placing" }) exitWith { [] };
private _cfg = missionConfigFile >> "CfgALife" >> "Placeables";
private _range = getNumber (_cfg >> "placeRange") max 2;

private _moving = !isNull _existing;
ALife_placing = true;
ALife_placeState = 0;   // 0 choosing, 1 place, -1 cancel
ALife_placeTurn = 0;    // degrees on top of where you face
ALife_placeDist = 2.5 min (_range - 0.5);
ALife_placeGhost = createVehicleLocal [_class, getPosATL player, [], 0, "CAN_COLLIDE"];
ALife_placeGhost enableSimulation false;
ALife_placeGhost allowDamage false;
ALife_placeGhost disableCollisionWith player;
if (!isNull _existing) then { _existing hideObject true; };

private _keys = (findDisplay 46) displayAddEventHandler ["KeyDown", {
    params ["", "_key"];
    private _cfg = missionConfigFile >> "CfgALife" >> "Placeables";
    private _range = getNumber (_cfg >> "placeRange") max 2;
    switch (true) do {
        case (_key == getNumber (_cfg >> "placeKey")): { ALife_placeState = 1; true };
        case (_key == getNumber (_cfg >> "cancelKey")): { ALife_placeState = -1; true };
        case (_key == getNumber (_cfg >> "turnLeftKey")): { ALife_placeTurn = ALife_placeTurn - 15; true };
        case (_key == getNumber (_cfg >> "turnRightKey")): { ALife_placeTurn = ALife_placeTurn + 15; true };
        case (_key == 201): { ALife_placeDist = (ALife_placeDist + 0.5) min (_range - 0.5); true }; // Page Up
        case (_key == 209): { ALife_placeDist = (ALife_placeDist - 0.5) max 1.5; true };            // Page Down
        default { false };
    };
}];
// Every frame, so it moves smoothly with you.
private _frame = addMissionEventHandler ["EachFrame", {
    if (isNull ALife_placeGhost) exitWith {};
    private _pos = player modelToWorldVisual [0, ALife_placeDist, 0];
    ALife_placeGhost setPosATL [_pos select 0, _pos select 1, 0];
    ALife_placeGhost setDir (getDirVisual player + ALife_placeTurn);
}];

"ALife_placeHint" cutRsc ["ALife_PlaceHint", "PLAIN", 0, false];
private _hint = {
    private _display = uiNamespace getVariable ["ALife_PlaceHint", displayNull];
    if (isNull _display) exitWith {};
    (_display displayCtrl 9211) ctrlSetStructuredText parseText format [
        "<t align='center' size='1.1' font='RobotoCondensedBold'>%1</t><br/><t align='center' size='0.9' color='#c7d0e0'>Space: put it here   Q / E: turn   Page Up / Down: nearer / further   Backspace: cancel</t>",
        _this];
};
private _name = getText (configFile >> "CfgVehicles" >> _class >> "displayName");
private _title = format [["Placing: %1", "Moving: %1"] select (!isNull _existing), _name];
_title call _hint;

private _result = [];
while { ALife_placeState >= 0 } do {
    if (!alive player || { vehicle player != player } || { player getVariable ["alife_downed", false] } || { player getVariable ["alife_restrained", false] }) exitWith {};
    if (_moving && { isNull _existing }) exitWith {}; // picked up / cleared meanwhile
    if (ALife_placeState == 1) then {
        private _pos = getPosATL ALife_placeGhost;
        if (surfaceIsWater _pos) then {
            ALife_placeState = 0;
            ["It can't go in the water.", "warning"] call ALife_fnc_toast;
        } else {
            _result = [[_pos select 0, _pos select 1, 0], getDir ALife_placeGhost];
            ALife_placeState = -1; // done
        };
    };
    uiSleep 0.05;
};

(findDisplay 46) displayRemoveEventHandler ["KeyDown", _keys];
removeMissionEventHandler ["EachFrame", _frame];
"ALife_placeHint" cutText ["", "PLAIN"];
deleteVehicle ALife_placeGhost;
if (!isNull _existing) then { _existing hideObject false; };
ALife_placing = nil;
if (_moving && { isNull _existing }) exitWith { [] };
_result
