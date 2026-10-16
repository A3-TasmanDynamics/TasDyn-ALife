/*
    ALife_fnc_sessionReceived (client, called by the server)

    The server's answer to ALife_fnc_requestSession.

    Arguments:
        0: STRING - "OK", "BANNED" or "ERROR"
        1: STRING - message (BANNED / ERROR)
        2: STRING - faction ("civ" / "cop" / "medic")
        3: ARRAY  - spawn options [[key, name, description, position], ...]
        4: ARRAY  - [name, cash, bank]
*/

params ["_status", ["_message", ""], ["_faction", ""], ["_options", []], ["_info", []]];

if (!isNil "ALife_sessionReply") exitWith {}; // already answered (a retry crossed in flight)
ALife_sessionReply = _status;

switch (_status) do {
    case "OK": {
        ALife_faction = _faction;
        ALife_spawnOptions = _options;
        ALife_playerInfo = _info;
        if (_options isEqualTo []) exitWith {
            ["error", "There are no spawn points for your faction.<br/>Tell a developer: the spawn markers are missing from mission.sqm."] call ALife_fnc_loading;
        };
        ["open"] call ALife_fnc_spawnMenu;
    };
    case "BANNED": {
        ["error", _message] call ALife_fnc_loading;
        [] spawn {
            sleep 30;
            ["end1", false] call BIS_fnc_endMission;
        };
    };
    default {
        ["error", _message + "<br/>Please reconnect."] call ALife_fnc_loading;
    };
};
