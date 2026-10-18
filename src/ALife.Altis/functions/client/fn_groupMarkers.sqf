/*
    ALife_fnc_groupMarkers (client)

    Your own people on the map and the GPS panel: police see police, EMS
    see EMS, and a civilian in a gang sees the rest of their gang. Both
    you and they need a GPS (ItemGPS linked); nobody else is shown. Local
    markers, updated every second; anyone who drops out of the group (dies,
    leaves, loses their GPS) is taken off.

    Started once from initPlayerLocal.sqf; runs for the whole session.
*/

if (!hasInterface) exitWith {};

private _hasGps = { "ItemGPS" in assignedItems _this };
// Colour per group: police blue, EMS red, gangs amber.
private _colours = createHashMapFromArray [["cop", "ColorBLUFOR"], ["medic", "ColorRed"], ["civ", "ColorOrange"]];
private _shown = createHashMap; // marker name -> true

while { true } do {
    private _faction = missionNamespace getVariable ["ALife_faction", "civ"];
    private _gang = player getVariable ["alife_gang", ""];
    private _side = side group player;
    private _wanted = createHashMap;

    if ((missionNamespace getVariable ["ALife_spawned", false]) && { alive player } && { player call _hasGps } && { _faction != "civ" || { _gang != "" } }) then {
        {
            if (
                _x != player && { alive _x } && { !isObjectHidden _x } && { side group _x == _side }
                && { _faction != "civ" || { (_x getVariable ["alife_gang", ""]) == _gang } }
                && { _x call _hasGps }
            ) then {
                private _name = "alife_gps_" + getPlayerUID _x;
                if (getMarkerType _name == "") then {
                    createMarkerLocal [_name, getPosWorld _x];
                    _name setMarkerTypeLocal "mil_triangle";
                    _name setMarkerSizeLocal [0.6, 0.6];
                    _name setMarkerColorLocal (_colours getOrDefault [_faction, "ColorOrange"]);
                };
                _name setMarkerPosLocal getPosWorld (vehicle _x);
                _name setMarkerDirLocal getDir (vehicle _x);
                _name setMarkerTextLocal ((_x getVariable ["alife_name", name _x]) + (["", " (down)"] select (_x getVariable ["alife_downed", false])));
                _wanted set [_name, true];
            };
        } forEach allPlayers;
    };

    // Take off anyone who isn't shown any more.
    { if !(_x in _wanted) then { deleteMarkerLocal _x; }; } forEach keys _shown;
    _shown = _wanted;
    sleep 1;
};
