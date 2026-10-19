/*
    ALife_fnc_playerTags (client)

    Name tags over other players you can see, as set in CfgALife >>
    PlayerTags: their name, and for police their rank under it
    (alife_rank, the website's police rank name, set when they spawn).
    Only players on foot, within `distance`, in your line of sight; the
    tag fades out towards the edge of that range. Drawn every frame
    (Draw3D), for the nearest few.

    Started once from initPlayerLocal.sqf.
*/

if (!hasInterface) exitWith {};
private _cfg = missionConfigFile >> "CfgALife" >> "PlayerTags";
if (getNumber (_cfg >> "on") != 1) exitWith {};

ALife_tagDistance = getNumber (_cfg >> "distance") max 2;
ALife_tagMax = getNumber (_cfg >> "max") max 1;
ALife_tagColours = createHashMapFromArray [
    ["cop", getArray (_cfg >> "copColour")],
    ["medic", getArray (_cfg >> "medicColour")],
    ["civ", getArray (_cfg >> "civColour")]
];

addMissionEventHandler ["Draw3D", {
    if (!(missionNamespace getVariable ["ALife_spawned", false]) || { !alive player } || { visibleMap }) exitWith {};
    private _eye = eyePos player;
    private _near = (player nearEntities ["CAManBase", ALife_tagDistance]) select {
        _x != player && { isPlayer _x } && { alive _x } && { !isObjectHidden _x } && { vehicle _x == _x }
    };
    _near = [_near, [], { player distance _x }] call BIS_fnc_sortBy;
    {
        // In sight: not behind walls or terrain.
        if (([player, "VIEW", _x] checkVisibility [_eye, eyePos _x]) > 0.2) then {
            private _distance = player distance _x;
            private _alpha = 1 - ((_distance / ALife_tagDistance) ^ 3);
            private _head = _x modelToWorldVisual ((_x selectionPosition "head") vectorAdd [0, 0, 0.42]);
            private _faction = [side group _x] call ALife_fnc_sideToFaction;
            private _rank = _x getVariable ["alife_rank", ""];
            private _colour = +(ALife_tagColours getOrDefault [_faction, [1, 1, 1, 1]]);
            _colour set [3, (_colour param [3, 1]) * _alpha];
            drawIcon3D ["", [1, 1, 1, _alpha], _head, 0, 0, 0, _x getVariable ["alife_name", name _x], 2, 0.034, "RobotoCondensedBold", "center"];
            if (_faction == "cop" && { _rank != "" }) then {
                drawIcon3D ["", _colour, _head vectorAdd [0, 0, -0.13 - _distance * 0.008], 0, 0, 0, _rank, 2, 0.028, "RobotoCondensed", "center"];
            };
        };
    } forEach (_near select [0, ALife_tagMax]);
}];
