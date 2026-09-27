/*
    File: fn_loadingScreen.sqf
    Author: Tasman Dynamics

    Description:
        Everything about the connection loading screen lives in this one
        file, mode-dispatched -- same shape as fn_spawnMenu.sqf. Shown from
        the moment a client connects (initPlayerLocal.sqf) until the spawn
        menu is ready to open (dialog/loadingScreen.hpp).

        The status text and progress bar reflect the ACTUAL join sequence,
        not a simulated timer: initPlayerLocal.sqf pushes the client-side
        milestones (player object ready, join request sent) via a local
        "setProgress" call, and fn_playerJoin.sqf remoteExecs the same mode
        to this specific client for the server-side milestones (DB record
        loading, load finished) -- the exact chain the mockup's fake
        setInterval-based progress bar was replaced with.

        IDD/IDC numbers are hardcoded to match dialog/loadingScreen.hpp's
        #defines -- SQF doesn't see config .hpp macros unless the file is
        explicitly #included, which fn_spawnMenu.sqf also avoids for the
        same reason (see its own IDC comments).

    Parameter(s):
        0: STRING - mode:
           "open"        -- create the dialog (client-only, called locally
                             from initPlayerLocal.sqf, never remoteExec'd)
           "onLoad"      -- dialog onLoad: start the tip-rotation loop,
                            install the Escape-blocking key handler (see
                            fn_spawnMenu.sqf's own "keyDown"/"escape" modes
                            for why -- same reasoning applies here: this
                            screen only ever shows while the player has no
                            valid position yet)
           "setProgress" -- 1: SCALAR percent (0-100), 2: STRING status text
                            -- remoteExec'd to a specific client from
                            fn_playerJoin.sqf, or called locally from
                            initPlayerLocal.sqf
           "keyDown"     -- display KeyDown event -- blocks Escape (returns
                            true), routes it to "escape"
           "escape"      -- Escape was pressed: kick to the server browser
           "close"       -- close the dialog -- remoteExec'd to a specific
                            client from fn_spawnPlayer.sqf once the player
                            has actually been positioned, or called locally
           "onUnload"    -- dialog onUnload: nothing to clean up, the tip
                            loop stops itself once the display is gone

    Returns:
        Nothing (except "keyDown", which returns a Boolean the engine
        requires to decide whether it should still process that key)
*/

params ["_mode"];
private _args = _this select [1, (count _this) - 1];

switch (_mode) do {
    case "open": {
        if (!isNull (findDisplay 4600)) exitWith {};
        createDialog "ALife_LoadingScreen";
    };

    case "onLoad": {
        (findDisplay 4600) displaySetEventHandler ["KeyDown", "(['keyDown'] + _this) call ALife_fnc_loadingScreen"];

        private _tips = [
            "TasDyn-ALife uses a custom C++/PostgreSQL bridge instead of the usual extDB3/MySQL stack.",
            "Your character data saves automatically -- civilian, police, and medic each have their own record.",
            "This server is in active development. Thanks for helping test it!",
            "Found a bug? Let the Tasman Dynamics team know."
        ];

        [_tips] spawn {
            params ["_tips"];
            private _index = 0;
            private _rotateInterval = 4.5;
            while { !isNull (findDisplay 4600) } do {
                disableSerialization;
                private _display = findDisplay 4600;
                if (!isNull _display) then {
                    (_display displayCtrl 4610) ctrlSetStructuredText parseText (_tips select _index);
                };
                _index = (_index + 1) % (count _tips);
                sleep _rotateInterval;
            };
        };
    };

    case "setProgress": {
        _args params ["_pct", "_statusText"];

        disableSerialization;
        private _display = findDisplay 4600;
        if (isNull _display) exitWith {};

        (_display displayCtrl 4611) ctrlSetText _statusText;
        (_display displayCtrl 4612) ctrlSetText format ["%1%2", round _pct, "%"];
        (_display displayCtrl 4613) progressSetPosition (_pct / 100);
    };

    case "close": {
        if (isNull (findDisplay 4600)) exitWith {};
        closeDialog 0;
    };

    case "keyDown": {
        _args params ["", "_key"];
        if (_key != 1) exitWith { false };
        ["escape"] call ALife_fnc_loadingScreen;
        true
    };

    case "escape": {
        [worldName, 1, "You must finish connecting to join the server.", false] call BIS_fnc_endMission;
    };

    case "onUnload": {};
};
