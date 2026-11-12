/*
    ALife_fnc_gameCommands (server)

    Runs what staff send from the website's admin panel: a private message,
    a warning, a kick, or an announcement to everyone. The website has
    already checked the staff member's permission and written the Staff
    Log; it queues the command in the database (game_commands) and this
    loop claims pending ones every few seconds through the extension
    (["gamecmd", ["claim"]]), runs them the same way the in-game admin menu
    does (ALife_fnc_adminApply on the player's machine), and reports back
    (["gamecmd", ["done", id, "done" | "failed", result]]). A command that
    waits more than 2 minutes expires instead (the extension does that).

    Started once from ALife_fnc_serverInit when the database is ready.
*/

if (!isServer) exitWith {};

private _fnc_done = {
    params ["_id", "_status", ["_result", ""]];
    private _r = ["gamecmd", ["done", _id, _status, _result]] call ALife_fnc_extension;
    if (_r != "OK") then { diag_log format ["[ALife] gamecmd %1: couldn't record the outcome: %2", _id, _r]; };
};

while { true } do {
    uiSleep 4;
    private _raw = ["gamecmd", ["claim"]] call ALife_fnc_extension;
    if (_raw select [0, 6] == "ERROR:") then {
        diag_log format ["[ALife] gamecmd: claim failed: %1", _raw];
    } else {
        private _cmds = parseSimpleArray _raw;
        {
            _x params [["_id", "", [""]], ["_action", "", [""]], ["_uid", "", [""]], ["_text", "", [""]], ["_by", "Staff", [""]]];
            if (_action == "announce") then {
                ["announce", _by, _text] remoteExecCall ["ALife_fnc_adminApply", 0];
                [_id, "done", "sent to everyone"] call _fnc_done;
                diag_log format ["[ALife] website: %1 announced: %2", _by, _text];
            } else {
                private _i = allPlayers findIf { getPlayerUID _x == _uid };
                if (_i < 0) then {
                    [_id, "failed", "not in game"] call _fnc_done;
                } else {
                    private _who = allPlayers select _i;
                    switch (_action) do {
                        case "message": {
                            ["message", _by, _text] remoteExecCall ["ALife_fnc_adminApply", owner _who];
                            [_id, "done", "delivered"] call _fnc_done;
                        };
                        case "warn": {
                            ["warn", _by, _text] remoteExecCall ["ALife_fnc_adminApply", owner _who];
                            [_id, "done", "delivered"] call _fnc_done;
                        };
                        case "kick": {
                            // Same as the in-game menu: they see why for a few
                            // seconds, then a real kick with the server's
                            // command password (without it, their own
                            // machine ends the mission).
                            ["kick", _by, format ["Kicked: %1", _text]] remoteExecCall ["ALife_fnc_adminApply", owner _who];
                            [_id, "done", "kicked"] call _fnc_done;
                            diag_log format ["[ALife] website: %1 kicked %2: %3", _by, _uid, _text];
                            [_who] spawn {
                                params ["_who"];
                                sleep 5;
                                if (isNull _who) exitWith {};
                                private _pw = ["server", ["password"]] call ALife_fnc_extension;
                                if (_pw select [0, 6] == "ERROR:") exitWith { diag_log "[ALife] kick: no server command password -- the player's own machine ends the mission"; };
                                if !(_pw serverCommand format ["#kick %1", getPlayerID _who]) then { diag_log "[ALife] kick: serverCommand #kick was refused"; };
                            };
                        };
                        default { [_id, "failed", "unknown action"] call _fnc_done; };
                    };
                };
            };
        } forEach _cmds;
    };
};
