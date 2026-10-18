/*
    ALife_fnc_groupMarkers (client)

    You and your own people on the map and the GPS panel, as set in
    CfgALife >> GroupMarkers: by default police see police, EMS see EMS,
    and a civilian in a gang sees the rest of their gang, when both you and
    they have a GPS (ItemGPS linked). Local markers, updated every
    `interval` seconds; anyone who drops out (dies, leaves, loses their
    GPS) is taken off.

    Started once from initPlayerLocal.sqf; runs for the whole session.
*/

if (!hasInterface) exitWith {};

private _cfg = missionConfigFile >> "CfgALife" >> "GroupMarkers";
private _interval = getNumber (_cfg >> "interval") max 0.25;
private _needGps = getNumber (_cfg >> "needGps") == 1;
private _type = getText (_cfg >> "type");
private _size = getNumber (_cfg >> "size");
private _hasGps = { !_needGps || { "ItemGPS" in assignedItems _this } };
private _shown = createHashMap; // marker name -> true

while { true } do {
    private _faction = missionNamespace getVariable ["ALife_faction", "civ"];
    private _gang = player getVariable ["alife_gang", ""];
    // Civilians: their gang; police / EMS: their faction's class.
    private _mine = _cfg >> (["gang", _faction] select (_faction != "civ"));
    private _sees = if (_faction == "civ") then { [] } else { getArray (_mine >> "sees") };
    private _wanted = createHashMap;

    if (
        (missionNamespace getVariable ["ALife_spawned", false]) && { alive player } && { getNumber (_mine >> "on") == 1 }
        && { _faction != "civ" || { _gang != "" } } && { player call _hasGps }
    ) then {
        {
            private _theirs = [side group _x] call ALife_fnc_sideToFaction;
            if (
                _x != player && { alive _x } && { !isObjectHidden _x }
                && { if (_faction == "civ") then { _theirs == "civ" && { (_x getVariable ["alife_gang", ""]) == _gang } } else { _theirs in _sees } }
                && { _x call _hasGps }
            ) then {
                private _name = "alife_gps_" + getPlayerUID _x;
                if (getMarkerType _name == "") then {
                    createMarkerLocal [_name, getPosWorld _x];
                    _name setMarkerTypeLocal _type;
                    _name setMarkerSizeLocal [_size, _size];
                };
                // Their own group's colour, so police and EMS stand apart.
                _name setMarkerColorLocal getText (_cfg >> (["gang", _theirs] select (_theirs != "civ")) >> "colour");
                _name setMarkerPosLocal getPosWorld (vehicle _x);
                _name setMarkerDirLocal getDir (vehicle _x);
                _name setMarkerTextLocal ((_x getVariable ["alife_name", name _x]) + (["", " (down)"] select (_x getVariable ["alife_downed", false])));
                _wanted set [_name, true];
            };
        } forEach allPlayers;
    };

    // You: always (when the map has no own-position marker of its own).
    private _self = _cfg >> "self";
    if (getNumber (_self >> "on") == 1 && { missionNamespace getVariable ["ALife_spawned", false] } && { alive player } && { player call _hasGps }) then {
        private _name = "alife_gps_self";
        if (getMarkerType _name == "") then {
            createMarkerLocal [_name, getPosWorld player];
            _name setMarkerTypeLocal getText (_self >> "type");
            _name setMarkerSizeLocal [getNumber (_self >> "size"), getNumber (_self >> "size")];
            _name setMarkerColorLocal getText (_self >> "colour");
            _name setMarkerTextLocal getText (_self >> "text");
        };
        _name setMarkerPosLocal getPosWorld (vehicle player);
        _name setMarkerDirLocal getDir (vehicle player);
        _wanted set [_name, true];
    };

    // Take off anyone who isn't shown any more.
    { if !(_x in _wanted) then { deleteMarkerLocal _x; }; } forEach keys _shown;
    _shown = _wanted;
    sleep _interval;
};
