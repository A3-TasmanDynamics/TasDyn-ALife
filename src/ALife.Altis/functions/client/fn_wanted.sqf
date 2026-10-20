/*
    ALife_fnc_wanted (client, called by the server: ALife_fnc_wantedBoard)

    Data for the phone's Wanted app (police):

        ["board", [[uid, name, bounty, crimes], ...]]  - everyone wanted
        ["crimes", [uid, [[id, crime, bounty], ...]]]   - one player's crimes
*/

params [["_mode", "", [""]], ["_data", []]];

switch (_mode) do {
    case "board": { ALife_wantedBoard = _data; };
    case "crimes": { ALife_wantedCrimes = _data; };
};
if (!isNull (findDisplay 5000) && { (missionNamespace getVariable ["ALife_phoneApp", ""]) == "wanted" }) then {
    ["refresh"] call ALife_fnc_phone;
};
