/*
    ALife_fnc_spawnOptions (server)

    The spawn points a player may pick, in menu order. "Last position" comes
    first when they logged off alive in this faction.

    Arguments:
        0: HASHMAP - loaded record
        1: STRING  - faction ("civ" / "cop" / "medic")

    Returns:
        ARRAY of [key, name, description, position]
*/

params ["_record", "_faction"];

private _options = [];

private _wasAlive = _record getOrDefault [_faction + "_alive", false];
private _lastPos = [_record getOrDefault [_faction + "_position", []]] call ALife_fnc_parsePosition;
if (_wasAlive isEqualTo true && { !isNil "_lastPos" }) then {
    _options pushBack ["last", "Last position", "Carry on where you logged off.", _lastPos];
};

{
    private _marker = getText (_x >> "marker");
    private _pos = getMarkerPos _marker;
    if (_marker != "" && { !(_pos isEqualTo [0, 0, 0]) }) then {
        _options pushBack [configName _x, getText (_x >> "displayName"), getText (_x >> "description"), _pos];
    } else {
        diag_log format ["[ALife] spawn point %1/%2: marker '%3' is missing from mission.sqm", _faction, configName _x, _marker];
    };
} forEach ("true" configClasses (missionConfigFile >> "CfgSpawnPoints" >> _faction));

_options
