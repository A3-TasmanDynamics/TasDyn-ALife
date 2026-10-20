/*
    ALife_fnc_admin (client)

    The admin menu (ui\admin.hpp), opened from the phone's Admin app. What
    a staff member can do comes from their website permissions (loaded with
    their character: ALife_phone "perms") -- server owners (CfgALife >>
    owners[]) get everything: only those buttons are shown, and
    the server checks every action again (ALife_fnc_adminAction), which also
    logs the important ones to the staff log / #staff-log.

    Players (pick one in the list):
        Teleport to, Bring here (game.teleport) - Spectate (game.spectate)
        Heal / revive (game.heal) - Freeze / unfreeze (game.freeze)
        Message (game.admin) - Warn (game.warn) - Kick (game.kick)
        Ban (bans.issue; Permanent needs bans.permanent)
        Compensate (players.compensate; over $10,000 needs players.compensate_large)
        Give item, Give cash (game.spawn)
        Give / take licence (game.spawn): gives the picked licence, or takes
            it away if they have it. Their licences (for the faction they're
            playing) are shown with their details.
        Their vehicles (game.vehicles): every vehicle they own, in any of
            their garages -- pull one out (next to you) or send one back
    You:
        God mode, Invisible (game.self) - Player markers (game.markers)
        Map teleport (game.teleport) - Heal me (game.heal) - Stop spectating
        Arsenal (game.spawn): Arma's full arsenal
    Server:
        Announce (announce.post) - Spawn vehicle (game.spawn)
        Repair / Refuel / Delete the vehicle you're looking at (game.vehicles)

    Ban, Kick and Delete vehicle need a second click to confirm.

    Modes:
        ["open"] / ["onLoad", display] / ["refresh"] / ["select"]
        ["action", id]
*/

#define IDD 5800
#define IDC_TITLE 5801
#define IDC_COUNT 5802
#define IDC_DETAILS 5803
#define IDC_LIST 5810
#define IDC_REASON 5820
#define IDC_AMOUNT 5821
#define IDC_ACCOUNT 5822
#define IDC_BAN 5823
#define IDC_ITEM 5824
#define IDC_VEHCLASS 5825
#define IDC_LICENCE 5826
#define IDC_BUTTON 5850

params ["_mode", ["_arg", ""]];

private _display = findDisplay IDD;
private _data = missionNamespace getVariable ["ALife_phone", createHashMap];
private _perms = _data getOrDefault ["perms", []];
private _owner = (getPlayerUID player) in getArray (missionConfigFile >> "CfgALife" >> "owners");
private _can = { _owner || { _this in _perms } };
private _safe = { (_this splitString "<>&") joinString "" };
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };

// [id, label, permission, column]
private _buttons = [
    ["goto", "Teleport to", "game.teleport", "player"],
    ["bring", "Bring here", "game.teleport", "player"],
    ["spectate", "Spectate", "game.spectate", "player"],
    ["heal", "Heal / revive", "game.heal", "player"],
    ["freeze", "Freeze / unfreeze", "game.freeze", "player"],
    ["message", "Message", "game.admin", "player"],
    ["warn", "Warn", "game.warn", "player"],
    ["kick", "Kick", "game.kick", "player"],
    ["ban", "Ban", "bans.issue", "player"],
    ["compensate", "Compensate", "players.compensate", "player"],
    ["give", "Give item", "game.spawn", "player"],
    ["giveLicence", "Give / take licence", "game.spawn", "player"],
    ["vehicles", "Their vehicles", "game.vehicles", "player"],
    ["giveCash", "Give cash", "game.spawn", "player"],
    ["god", "God mode", "game.self", "self"],
    ["invisible", "Invisible", "game.self", "self"],
    ["markers", "Player markers", "game.markers", "self"],
    ["mapTeleport", "Map teleport", "game.teleport", "self"],
    ["healSelf", "Heal me", "game.heal", "self"],
    ["stopSpectate", "Stop spectating", "game.spectate", "self"],
    ["arsenal", "Arsenal", "game.spawn", "self"],
    ["announce", "Announce", "announce.post", "server"],
    ["spawnVehicle", "Spawn vehicle", "game.spawn", "server"],
    ["repair", "Repair vehicle", "game.vehicles", "server"],
    ["refuel", "Refuel vehicle", "game.vehicles", "server"],
    ["deleteVeh", "Delete vehicle", "game.vehicles", "server"]
];

// The selected player (by UID, so the list can refresh underneath).
private _fnc_target = {
    private _uid = missionNamespace getVariable ["ALife_adminTarget", ""];
    private _i = allPlayers findIf { getPlayerUID _x == _uid };
    if (_i < 0) then { objNull } else { allPlayers select _i }
};
private _fnc_toggleText = {
    params ["_id", "_label"];
    switch (_id) do {
        case "god": { format ["%1: %2", _label, ["Off", "On"] select (missionNamespace getVariable ["ALife_adminGod", false])] };
        case "invisible": { format ["%1: %2", _label, ["Off", "On"] select isObjectHidden player] };
        case "markers": { format ["%1: %2", _label, ["Off", "On"] select (missionNamespace getVariable ["ALife_adminMarkers", false])] };
        default { _label };
    };
};

switch (_mode) do {
    case "open": {
        if !("game.admin" call _can) exitWith { ["You're not on the staff team.", "warning"] call ALife_fnc_toast; };
        if (dialog) then { closeDialog 0; };
        createDialog "ALife_Admin";
    };

    case "onLoad": {
        _display = _arg;
        ALife_adminOpen = true;
        ALife_adminLicences = [];
        ALife_adminLicencesAsked = [];
        private _rank = _data getOrDefault ["staffRank", ""];
        (_display displayCtrl IDC_TITLE) ctrlSetStructuredText parseText format [
            "<t font='RobotoCondensedBold' size='1.25'>ADMIN</t><t color='#8b98b3' size='1.05'>   %1</t>",
            ((_rank splitString "_") apply { (toUpper (_x select [0, 1])) + (_x select [1]) }) joinString " "];

        private _combo = _display displayCtrl IDC_ACCOUNT;
        { _combo lbSetData [_combo lbAdd (_x select 1), _x select 0]; } forEach [["civ", "Civilian bank"], ["cop", "Police bank"], ["medic", "EMS bank"]];
        _combo lbSetCurSel 0;
        _combo = _display displayCtrl IDC_BAN;
        {
            if ((_x select 0) > 0 || { "bans.permanent" call _can }) then { _combo lbSetValue [_combo lbAdd (_x select 1), _x select 0]; };
        } forEach [[60, "1 hour"], [1440, "1 day"], [4320, "3 days"], [10080, "7 days"], [43200, "30 days"], [0, "Permanent"]];
        _combo lbSetCurSel 1;
        _combo = _display displayCtrl IDC_LICENCE;
        { _combo lbSetData [_combo lbAdd getText (_x >> "displayName"), configName _x]; } forEach ("true" configClasses (missionConfigFile >> "CfgLicences"));
        _combo lbSetCurSel 0;
        _combo = _display displayCtrl IDC_ITEM;
        { _combo lbSetData [_combo lbAdd getText (_x >> "displayName"), configName _x]; } forEach ("true" configClasses (missionConfigFile >> "CfgItems"));
        _combo lbSetCurSel 0;
        (_display displayCtrl IDC_AMOUNT) ctrlSetText "1";

        // The buttons this staff member can use, in their column.
        private _cols = createHashMapFromArray [["player", [0.385, 0.54, 0.107]], ["self", [0.62, 0.235, 0.102]], ["server", [0.62, 0.53, 0.102]]];
        private _counts = createHashMap;
        {
            _x params ["_id", "_label", "_perm", "_col"];
            if (_perm call _can) then {
                (_cols get _col) params ["_x0", "_y0", "_w"];
                private _n = _counts getOrDefault [_col, 0];
                _counts set [_col, _n + 1];
                private _btn = _display ctrlCreate ["ALife_RscButton", IDC_BUTTON + _forEachIndex];
                _btn ctrlSetPosition [
                    safezoneX + safezoneW * (_x0 + (_n mod 2) * (_w + 0.006)),
                    safezoneY + safezoneH * (_y0 + floor (_n / 2) * 0.042),
                    safezoneW * _w, safezoneH * 0.036];
                _btn ctrlCommit 0;
                _btn ctrlSetText ([_id, _label] call _fnc_toggleText);
                private _danger = _id in ["kick", "ban", "deleteVeh"];
                _btn ctrlSetBackgroundColor ([[1, 1, 1, 0.08], [0.6, 0.15, 0.15, 0.6]] select _danger);
                _btn ctrlAddEventHandler ["ButtonClick", { ["action", ctrlIDC (_this select 0) - IDC_BUTTON] call ALife_fnc_admin; }];
            };
        } forEach _buttons;

        ["refresh"] call ALife_fnc_admin;
        // Keep the list and the selected player's details fresh while open.
        [] spawn {
            while { missionNamespace getVariable ["ALife_adminOpen", false] } do {
                uiSleep 2;
                if (missionNamespace getVariable ["ALife_adminOpen", false]) then { ["refresh"] call ALife_fnc_admin; };
            };
        };
    };

    case "refresh": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _players = [allPlayers select { !isObjectHidden _x || { _x == player } }, [], { name _x }] call BIS_fnc_sortBy;
        private _selected = missionNamespace getVariable ["ALife_adminTarget", ""];
        lnbClear _list;
        {
            private _faction = [side group _x] call ALife_fnc_sideToFaction;
            private _row = _list lnbAddRow [(_x getVariable ["alife_name", name _x]) call _safe, switch (_faction) do { case "cop": { "Police" }; case "medic": { "EMS" }; default { "Civilian" }; }];
            _list lnbSetData [[_row, 0], getPlayerUID _x];
            _list lnbSetColor [[_row, 1], switch (_faction) do { case "cop": { [0.34, 0.62, 1, 1] }; case "medic": { [0.29, 0.81, 0.59, 1] }; default { [0.7, 0.55, 0.9, 1] }; }];
            if (getPlayerUID _x == _selected) then { _list lnbSetCurSelRow _row; };
        } forEach _players;
        (_display displayCtrl IDC_COUNT) ctrlSetText format ["PLAYERS (%1)", count _players];
        ["select", true] call ALife_fnc_admin;
    };

    // From the server: [uid, faction, licences] for the selected player.
    case "licences": {
        ALife_adminLicences = _arg;
        ["select", true] call ALife_fnc_admin;
    };

    case "select": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        if (_row >= 0 && { _arg isNotEqualTo true }) then { ALife_adminTarget = _list lnbData [_row, 0]; };
        private _t = call _fnc_target;
        // Their licences come from the server: asked for when the selected
        // player (or the faction they're playing) changes.
        private _licences = "";
        if (!isNull _t && { "game.admin" call _can }) then {
            private _faction = if (isObjectHidden _t || { !alive _t }) then { "" } else { [side group _t] call ALife_fnc_sideToFaction };
            private _key = [getPlayerUID _t, _faction];
            private _known = missionNamespace getVariable ["ALife_adminLicences", []];
            private _asked = missionNamespace getVariable ["ALife_adminLicencesAsked", []];
            if ((_known param [0, ""]) == getPlayerUID _t && { (_known select [0, 2]) isEqualTo _key || { _asked isEqualTo _key } }) then {
                private _names = (_known param [2, []]) apply { getText (missionConfigFile >> "CfgLicences" >> _x >> "displayName") };
                _licences = if (_names isEqualTo []) then { "none" } else { _names joinString ", " };
            } else {
                _licences = "...";
                if (_asked isNotEqualTo _key) then {
                    ALife_adminLicencesAsked = _key;
                    [player, "licences", _t, []] remoteExecCall ["ALife_fnc_adminAction", 2];
                };
            };
        };
        (_display displayCtrl IDC_DETAILS) ctrlSetStructuredText parseText (if (isNull _t) then {
            "<t color='#8b98b3'>Pick a player.</t>"
        } else {
            private _state = [];
            if (_t getVariable ["alife_downed", false]) then { _state pushBack "<t color='#fca5a5'>downed</t>"; };
            if (!alive _t && { !(_t getVariable ["alife_downed", false]) }) then { _state pushBack "dead"; };
            if (_t getVariable ["alife_restrained", false]) then { _state pushBack "<t color='#fbbf24'>restrained</t>"; };
            if (isObjectHidden _t) then { _state pushBack "in the spawn menu"; };
            if (vehicle _t != _t) then { _state pushBack format ["in a %1", getText (configFile >> "CfgVehicles" >> typeOf vehicle _t >> "displayName")]; };
            format ["<t font='RobotoCondensedBold' size='1.2'>%1</t><br/><t color='#8b98b3'>UID </t>%2<br/><t color='#8b98b3'>Health </t>%3%%   <t color='#8b98b3'>Distance </t>%4 m%5%6",
                (_t getVariable ["alife_name", name _t]) call _safe, getPlayerUID _t, round ((1 - damage _t) * 100), round (player distance _t),
                if (_state isEqualTo []) then { "" } else { "<br/>" + (_state joinString ", ") },
                if (_licences == "") then { "" } else { format ["<br/><t size='0.9'><t color='#8b98b3'>Licences </t>%1</t>", _licences] }]
        });
    };

    case "action": {
        if (isNull _display) exitWith {};
        (_buttons param [_arg, []]) params [["_id", ""], ["_label", ""], ["_perm", ""]];
        if (_id == "" || { !(_perm call _can) }) exitWith {};
        private _t = call _fnc_target;
        private _reason = ctrlText (_display displayCtrl IDC_REASON);
        private _amount = floor parseNumber ctrlText (_display displayCtrl IDC_AMOUNT);
        private _needsTarget = (_buttons select _arg) select 3 == "player";
        if (_needsTarget && { isNull _t }) exitWith { ["Pick a player first.", "warning"] call ALife_fnc_toast; };
        if (_id in ["message", "warn", "kick", "ban", "compensate", "announce"] && { _reason == "" }) exitWith {
            [["Type a reason first.", "Type the message first."] select (_id in ["message", "announce"]), "warning"] call ALife_fnc_toast;
        };
        // Ban, Kick and Delete vehicle: a second click within 4 s confirms.
        if (_id in ["kick", "ban", "deleteVeh"]) then {
            private _key = [_id, missionNamespace getVariable ["ALife_adminTarget", ""]];
            private _pending = missionNamespace getVariable ["ALife_adminConfirm", []];
            if ((_pending param [0, []]) isEqualTo _key && { diag_tickTime - (_pending param [1, 0]) < 4 }) then {
                ALife_adminConfirm = [];
            } else {
                ALife_adminConfirm = [_key, diag_tickTime];
                _id = "";
                [format ["Press %1 again to confirm.", _label], "warning"] call ALife_fnc_toast;
            };
        };
        if (_id == "") exitWith {};
        private _vehicle = if (vehicle player != player) then { vehicle player } else { cursorObject };
        if (_id in ["repair", "refuel", "deleteVeh"] && { isNull _vehicle || { !(_vehicle isKindOf "LandVehicle" || { _vehicle isKindOf "Air" } || { _vehicle isKindOf "Ship" }) } }) exitWith {
            ["Look at a vehicle (or sit in one) first.", "warning"] call ALife_fnc_toast;
        };

        switch (_id) do {
            // Things only this machine needs to do.
            case "goto": {
                if (vehicle _t != _t && { (vehicle _t) emptyPositions "cargo" > 0 }) then { player moveInCargo vehicle _t; } else { player setPosATL (_t modelToWorld [0, -2, 0]); };
                [format ["Teleported to %1.", name _t], "success"] call ALife_fnc_toast;
            };
            case "spectate": {
                closeDialog 0;
                _t switchCamera "EXTERNAL";
                ALife_adminSpectating = true;
                [format ["Spectating %1. Phone -> Admin -> Stop spectating to come back.", name _t], "info"] call ALife_fnc_toast;
            };
            case "stopSpectate": {
                player switchCamera "INTERNAL";
                ALife_adminSpectating = false;
            };
            case "vehicles": {
                ["openAdmin", [getPlayerUID _t, _t getVariable ["alife_name", name _t]]] call ALife_fnc_garageMenu;
            };
            case "healSelf": { player setDamage 0; ["Healed.", "success"] call ALife_fnc_toast; };
            case "arsenal": {
                closeDialog 0;
                [player, "arsenal", objNull, []] remoteExecCall ["ALife_fnc_adminAction", 2]; // logged
                ["Open", true] spawn BIS_fnc_arsenal; // the full arsenal; what you leave with is saved with your gear
            };
            case "god": {
                ALife_adminGod = !(missionNamespace getVariable ["ALife_adminGod", false]);
                player allowDamage !ALife_adminGod;
                if (ALife_adminGod) then { [player, "god", objNull, []] remoteExecCall ["ALife_fnc_adminAction", 2]; };
                [format ["God mode %1.", ["off", "on"] select ALife_adminGod], "info"] call ALife_fnc_toast;
            };
            case "markers": {
                ALife_adminMarkers = !(missionNamespace getVariable ["ALife_adminMarkers", false]);
                if (ALife_adminMarkers) then {
                    [] spawn {
                        private _shown = [];
                        while { missionNamespace getVariable ["ALife_adminMarkers", false] } do {
                            private _now = [];
                            {
                                private _name = "alife_admin_" + getPlayerUID _x;
                                if (getMarkerType _name == "") then {
                                    createMarkerLocal [_name, getPosWorld _x];
                                    _name setMarkerTypeLocal "mil_dot";
                                };
                                _name setMarkerPosLocal getPosWorld (vehicle _x);
                                _name setMarkerColorLocal (switch ([side group _x] call ALife_fnc_sideToFaction) do { case "cop": { "ColorBLUFOR" }; case "medic": { "ColorGUER" }; default { "ColorCIV" }; });
                                _name setMarkerTextLocal (name _x);
                                _now pushBack _name;
                            } forEach (allPlayers select { _x != player && { alive _x } });
                            { deleteMarkerLocal _x; } forEach (_shown - _now);
                            _shown = _now;
                            uiSleep 1;
                        };
                        { deleteMarkerLocal _x; } forEach _shown;
                    };
                };
                [format ["Player markers %1.", ["off", "on"] select ALife_adminMarkers], "info"] call ALife_fnc_toast;
            };
            case "mapTeleport": {
                closeDialog 0;
                openMap true;
                ["Click on the map to teleport there.", "info"] call ALife_fnc_toast;
                addMissionEventHandler ["MapSingleClick", {
                    params ["", "_pos"];
                    removeMissionEventHandler ["MapSingleClick", _thisEventHandler];
                    (vehicle player) setPosATL [_pos select 0, _pos select 1, 0];
                    openMap false;
                }];
            };
            // Everything else: the server checks and does it.
            default {
                private _args = switch (_id) do {
                    case "ban": { [_reason, (_display displayCtrl IDC_BAN) lbValue lbCurSel (_display displayCtrl IDC_BAN)] };
                    case "compensate": { [_reason, _amount, (_display displayCtrl IDC_ACCOUNT) lbData lbCurSel (_display displayCtrl IDC_ACCOUNT)] };
                    case "give": { [(_display displayCtrl IDC_ITEM) lbData lbCurSel (_display displayCtrl IDC_ITEM), _amount] };
                    case "giveLicence": { [(_display displayCtrl IDC_LICENCE) lbData lbCurSel (_display displayCtrl IDC_LICENCE)] };
                    case "giveCash": { [_reason, _amount] };
                    case "spawnVehicle": { [ctrlText (_display displayCtrl IDC_VEHCLASS)] };
                    case "repair";
                    case "refuel";
                    case "deleteVeh": { [_vehicle] };
                    default { [_reason] };
                };
                [player, _id, [objNull, _t] select _needsTarget, _args] remoteExecCall ["ALife_fnc_adminAction", 2];
            };
        };
        // Toggle labels.
        if (!isNull _display) then {
            {
                private _btn = _display displayCtrl (IDC_BUTTON + _forEachIndex);
                if (!isNull _btn) then { _btn ctrlSetText ([_x select 0, _x select 1] call _fnc_toggleText); };
            } forEach _buttons;
        };
    };
};
