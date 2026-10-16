/*
    ALife_fnc_spawnMenu (client)

    The spawn point picker (ui\spawnMenu.hpp). Options come from the server
    (ALife_spawnOptions); the client only ever sends back the chosen key.

    Modes:
        ["open"]                - open the menu
        ["onLoad", display]     - fill it (dialog onLoad)
        ["select", ctrl, index] - a row was picked: show it on the map
        ["confirm"]             - Spawn button
        ["leave"]               - Leave button: back to the server browser
        ["keyDown", ...]        - swallow Esc (there's no "no spawn" state)
        ["onUnload"]            - tidy up map markers
*/

#define IDD_SPAWN   4800
#define IDC_HEADER  4801
#define IDC_LIST    4810
#define IDC_MAP     4820
#define IDC_INFO    4830
#define IDC_SPAWN   4840

params ["_mode"];
private _args = _this select [1];

private _fnc_clearMarkers = {
    { deleteMarkerLocal _x } forEach (allMapMarkers select { (_x find "alife_spawn_") == 0 });
};

switch (_mode) do {
    case "open": {
        if (!isNull (findDisplay IDD_SPAWN)) exitWith {};
        ["show", "Choose where to spawn."] call ALife_fnc_loading;
        createDialog "ALife_SpawnMenu";
    };

    case "onLoad": {
        _args params ["_display"];
        _display displayAddEventHandler ["KeyDown", { (["keyDown"] + _this) call ALife_fnc_spawnMenu }];

        private _faction = missionNamespace getVariable ["ALife_faction", "civ"];
        private _factionName = switch (_faction) do {
            case "cop": { "Police" };
            case "medic": { "Medic" };
            default { "Civilian" };
        };
        (missionNamespace getVariable ["ALife_playerInfo", []]) params [["_name", name player], ["_cash", 0], ["_bank", 0]];

        (_display displayCtrl IDC_HEADER) ctrlSetStructuredText parseText format [
            "<t size='1.35' font='RobotoCondensedBold'>CHOOSE A SPAWN POINT</t><t size='1.05' color='#9aa4b2'>   %1  |  %2  |  Cash $%3  |  Bank $%4</t>",
            _factionName, _name, [_cash] call BIS_fnc_numberText, [_bank] call BIS_fnc_numberText];

        call _fnc_clearMarkers;
        private _colour = switch (_faction) do {
            case "cop": { "ColorWEST" };
            case "medic": { "ColorGUER" };
            default { "ColorCIV" };
        };

        private _list = _display displayCtrl IDC_LIST;
        lnbClear _list;
        _list lnbSetColumnsPos [0, 0.14];
        {
            _x params ["_key", "_label", "", "_pos"];
            private _row = _list lnbAddRow ["", _label];
            _list lnbSetPicture [[_row, 0], ["\A3\ui_f\data\map\markers\military\dot_ca.paa", "\A3\ui_f\data\map\markers\military\flag_ca.paa"] select (_key == "last")];
            _list lnbSetData [[_row, 0], _key];

            private _marker = createMarkerLocal [format ["alife_spawn_%1", _forEachIndex], _pos];
            _marker setMarkerTypeLocal "mil_dot";
            _marker setMarkerColorLocal _colour;
            _marker setMarkerTextLocal _label;
        } forEach (missionNamespace getVariable ["ALife_spawnOptions", []]);

        if (lnbSize _list select 0 > 0) then {
            _list lnbSetCurSelRow 0;
            ["select", _list, 0] call ALife_fnc_spawnMenu;
        };
    };

    case "select": {
        _args params ["_ctrl", "_index"];
        private _display = findDisplay IDD_SPAWN;
        if (isNull _display || { _index < 0 }) exitWith {};

        private _key = _ctrl lnbData [_index, 0];
        private _option = (missionNamespace getVariable ["ALife_spawnOptions", []]) select { (_x select 0) == _key };
        if (_option isEqualTo []) exitWith {};
        (_option select 0) params ["", "_label", "_description", "_pos"];

        {
            private _selected = _x == format ["alife_spawn_%1", _index];
            _x setMarkerTypeLocal (["mil_dot", "mil_objective"] select _selected);
            _x setMarkerSizeLocal ([[0.8, 0.8], [1.2, 1.2]] select _selected);
        } forEach (allMapMarkers select { (_x find "alife_spawn_") == 0 });

        private _map = _display displayCtrl IDC_MAP;
        _map ctrlMapAnimAdd [0.4, 0.05, _pos];
        ctrlMapAnimCommit _map;

        (_display displayCtrl IDC_INFO) ctrlSetStructuredText parseText format [
            "<t size='1.6' font='RobotoCondensedBold'>%1</t><br/><t size='1' color='#9aa4b2'>%2</t>", _label, _description];
    };

    case "confirm": {
        private _display = findDisplay IDD_SPAWN;
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_LIST;
        private _row = lnbCurSelRow _list;
        if (_row < 0) exitWith { hint "Pick a spawn point first."; };
        private _key = _list lnbData [_row, 0];

        closeDialog 0;
        ["show", "Spawning..."] call ALife_fnc_loading;
        ALife_spawnPending = true;
        [player, _key] remoteExec ["ALife_fnc_requestSpawn", 2];

        // If the server never answers, put the menu back rather than
        // leaving the player on a black screen.
        [] spawn {
            private _deadline = diag_tickTime + 20;
            waitUntil { sleep 0.2; !ALife_spawnPending || { diag_tickTime > _deadline } };
            if (ALife_spawnPending) then {
                ALife_spawnPending = false;
                hint "The server didn't answer. Try again.";
                ["open"] call ALife_fnc_spawnMenu;
            };
        };
    };

    case "leave": {
        closeDialog 0;
        ["end1", false] call BIS_fnc_endMission;
    };

    case "keyDown": {
        _args params ["", "_key"];
        _key == 1 // DIK_ESCAPE: handled (ignored); everything else passes through
    };

    case "onUnload": {
        call _fnc_clearMarkers;
    };
};
