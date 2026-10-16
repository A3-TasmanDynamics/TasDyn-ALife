/*
    ALife_fnc_deathScreen (client)

    The death screen (ui\deathScreen.hpp). The view switches to a camera 5 m
    above your body looking straight down. Shows who killed you, with what
    and from how far, the distance to the nearest on-duty medic, and a bar
    counting down from 100% to 0% over CfgALife >> respawnTime (2 minutes).
    Respawn unlocks at 0%; until you press it you keep waiting (for a
    medic). The game's own automatic respawn is held off while you wait.

    Modes:
        ["start", unit, killer, instigator] - from the player's Killed event
        ["open"] / ["onLoad", display]      - show the dialog
        ["tick"]                            - refresh timer, bar and medic distance
        ["camera"] / ["cameraOff"]          - bird's-eye view of the body / back to normal
        ["respawn"]                         - Respawn button
        ["keyDown", ...]                    - swallow Esc (no leaving while
                                              incapacitated: that's combat logging)
*/

#define IDD_DEATH    4900
#define IDC_KILLER   4910
#define IDC_MEDIC    4920
#define IDC_TIMER    4930
#define IDC_BAR      4931
#define IDC_RESPAWN  4940

params ["_mode"];
private _args = _this select [1];

private _fnc_total = {
    private _t = getNumber (missionConfigFile >> "CfgALife" >> "respawnTime");
    [120, _t] select (_t > 0)
};

switch (_mode) do {
    case "start": {
        _args params ["_unit", "_killer", "_instigator"];

        // Only the Respawn button respawns you (it sets this back to 0).
        setPlayerRespawnTime 1e6;
        ALife_respawnRequested = false;

        ALife_deathBody = _unit;
        ALife_deathAt = diag_tickTime;
        ALife_deathInfo = [_unit, _killer, _instigator] call ALife_fnc_killInfo;

        closeDialog 0;
        ["camera"] call ALife_fnc_deathScreen;
        ["open"] call ALife_fnc_deathScreen;

        [] spawn {
            // `player` stays the dead body until the respawn happens.
            while { !alive player } do {
                // Keep the game's own respawn timer held off until Respawn is pressed.
                if (!ALife_respawnRequested && { playerRespawnTime < 1e5 }) then { setPlayerRespawnTime 1e6; };
                if (!ALife_respawnRequested && { isNull (findDisplay IDD_DEATH) }) then { ["open"] call ALife_fnc_deathScreen; };
                ["tick"] call ALife_fnc_deathScreen;
                ["camera"] call ALife_fnc_deathScreen;
                sleep 0.25;
            };
            ["cameraOff"] call ALife_fnc_deathScreen;
            if (!isNull (findDisplay IDD_DEATH)) then { (findDisplay IDD_DEATH) closeDisplay 2; };
        };
    };

    case "open": {
        if (!isNull (findDisplay IDD_DEATH)) exitWith {};
        createDialog "ALife_DeathScreen";
    };

    case "onLoad": {
        _args params ["_display"];
        _display displayAddEventHandler ["KeyDown", { (["keyDown"] + _this) call ALife_fnc_deathScreen }];

        (missionNamespace getVariable ["ALife_deathInfo", ["", "", -1]]) params ["_name", "_weapon", "_distance"];
        private _text = if (_name == "") then {
            "<t color='#ffffff'>You died.</t>"
        } else {
            format ["<t color='#c0c7d1'>Killed by </t><t color='#ffffff' font='RobotoCondensedBold'>%1</t><t color='#c0c7d1'>   |   </t><t color='#ffffff'>%2</t><t color='#c0c7d1'>   |   </t><t color='#ffffff'>%3 m</t>",
                (_name splitString "<>") joinString "", (_weapon splitString "<>") joinString "", _distance]
        };
        (_display displayCtrl IDC_KILLER) ctrlSetStructuredText parseText _text;
        ["tick"] call ALife_fnc_deathScreen;
    };

    case "tick": {
        private _display = findDisplay IDD_DEATH;
        if (isNull _display) exitWith {};

        private _total = call _fnc_total;
        private _left = (_total - (diag_tickTime - (missionNamespace getVariable ["ALife_deathAt", diag_tickTime]))) max 0;
        private _fraction = _left / _total;

        (_display displayCtrl IDC_BAR) progressSetPosition _fraction;

        private _timer = if (_left > 0) then {
            private _secs = ceil _left;
            private _ss = _secs % 60;
            private _clock = format ["%1:%2%3", floor (_secs / 60), ["", "0"] select (_ss < 10), _ss];
            format ["<t color='#c0c7d1'>Respawn in </t><t color='#ffffff' font='RobotoCondensedBold'>%1</t><t color='#c0c7d1'>  (%2)</t>",
                _clock, (str round (_fraction * 100)) + "%"]
        } else {
            "<t color='#8ff0a4' font='RobotoCondensedBold'>Respawn ready</t><t color='#c0c7d1'>, or wait for a medic</t>"
        };
        (_display displayCtrl IDC_TIMER) ctrlSetStructuredText parseText _timer;
        (_display displayCtrl IDC_RESPAWN) ctrlEnable (_left <= 0);

        // Nearest medic who's actually spawned in (not still in the spawn menu).
        private _body = missionNamespace getVariable ["ALife_deathBody", player];
        private _medics = allPlayers select {
            side group _x == independent && { alive _x } && { _x != player } && { !isObjectHidden _x }
        };
        private _medicText = if (_medics isEqualTo []) then {
            "<t align='right' color='#ff8f88'>No medics on duty</t>"
        } else {
            private _nearest = selectMin (_medics apply { _x distance _body });
            format ["<t align='right'><t color='#c0c7d1'>Nearest medic </t><t color='#ffffff' font='RobotoCondensedBold'>%1</t></t>",
                if (_nearest >= 1000) then { ((_nearest / 1000) toFixed 1) + " km" } else { (str round _nearest) + " m" }]
        };
        (_display displayCtrl IDC_MEDIC) ctrlSetStructuredText parseText _medicText;
    };

    case "camera": {
        // Follows the body (it may still be sliding or ragdolling).
        private _body = missionNamespace getVariable ["ALife_deathBody", objNull];
        if (isNull _body) exitWith {};
        // A hair off vertical so "straight down" has a defined up direction.
        private _above = (ASLToAGL (getPosASL _body)) vectorAdd [0, -0.05, 5];
        private _cam = missionNamespace getVariable ["ALife_deathCam", objNull];
        if (isNull _cam) then {
            _cam = "camera" camCreate _above;
            _cam cameraEffect ["Internal", "Back"];
            showCinemaBorder false;
            _cam camSetFov 0.75;
            ALife_deathCam = _cam;
            _cam camSetPos _above;
            _cam camSetTarget _body;
            _cam camCommit 0;
        } else {
            _cam camSetPos _above;
            _cam camSetTarget _body;
            _cam camCommit 0.25;
        };
    };

    case "cameraOff": {
        private _cam = missionNamespace getVariable ["ALife_deathCam", objNull];
        if (isNull _cam) exitWith {};
        _cam cameraEffect ["Terminate", "Back"];
        camDestroy _cam;
        ALife_deathCam = objNull;
    };

    case "respawn": {
        private _left = (call _fnc_total) - (diag_tickTime - (missionNamespace getVariable ["ALife_deathAt", diag_tickTime]));
        if (_left > 0 || { alive player }) exitWith {};
        ALife_respawnRequested = true;
        closeDialog 0;
        ["show", "Respawning..."] call ALife_fnc_loading;
        setPlayerRespawnTime 0;
    };

    case "keyDown": {
        _args params ["", "_key"];
        _key == 1 // Esc does nothing here
    };
};
