/*
    ALife_fnc_jailClient (client, called by the server)

    Being in jail (ALife_fnc_jail keeps the clock):

        ["start", seconds, bail, jail position]  - moved into the jail (weapons
                                                   taken); kept inside the jail
                                                   area; a countdown at the top
        ["release", 0, 0, release position]      - out, at the release point

    Press Windows in jail to pay bail (ALife_fnc_interact).
*/

params [["_mode", "", [""]], ["_seconds", 0, [0]], ["_bail", 0, [0]], ["_pos", [], [[]]]];

if (!hasInterface) exitWith {};

switch (_mode) do {
    case "start": {
        if (dialog) then { closeDialog 0; };
        if (vehicle player != player) then { moveOut player; };
        if (!isNull attachedTo player) then { detach player; };
        removeAllWeapons player;
        if (_pos isNotEqualTo []) then { player setPosATL [_pos select 0, _pos select 1, 0]; };
        ALife_jailed = true;
        ALife_jailUntil = diag_tickTime + _seconds;
        ALife_jailBail = _bail;
        [format ["You're in jail for %1 minutes.%2", ceil (_seconds / 60),
            if (_bail > 0) then { format [" Bail is $%1 (Windows to pay it from your bank).", [_bail] call BIS_fnc_numberText] } else { "" }], "warning", "Jail", 15] call ALife_fnc_toast;
        if (!isNil "ALife_jailLoop" && { !scriptDone ALife_jailLoop }) exitWith {};
        ALife_jailLoop = [] spawn {
            private _area = getArray (missionConfigFile >> "CfgJail" >> "markers") param [0, ""];
            while { missionNamespace getVariable ["ALife_jailed", false] } do {
                // Kept inside the jail.
                if (alive player && { _area in allMapMarkers } && { !(player inArea _area) }) then {
                    private _c = getMarkerPos _area;
                    player setPosATL [_c select 0, _c select 1, 0];
                    ["You can't leave the jail.", "warning"] call ALife_fnc_toast;
                };
                private _left = ((ALife_jailUntil - diag_tickTime) max 0);
                titleText [format ["<t size='1.1' color='#fca5a5'>JAIL  %1:%2</t>", floor (_left / 60), ([str (floor (_left % 60)), "0" + str (floor (_left % 60))] select (floor (_left % 60) < 10))], "PLAIN DOWN", 0.001, false, true];
                sleep 1;
            };
            titleText ["", "PLAIN DOWN"];
        };
    };
    case "release": {
        ALife_jailed = false;
        ALife_jailUntil = 0;
        ALife_jailBail = 0;
        titleText ["", "PLAIN DOWN"];
        if (_pos isNotEqualTo [] && { alive player }) then { player setPosATL [_pos select 0, _pos select 1, 0]; };
        ["You've been released from jail.", "success", "Jail"] call ALife_fnc_toast;
    };
};
