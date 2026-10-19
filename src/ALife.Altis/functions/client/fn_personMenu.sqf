/*
    ALife_fnc_personMenu (client)

    The radial menu for a player you're looking at (Windows key,
    ALife_fnc_interact):

        Police:   Restrain / Unrestrain (also Left Shift + R), Fine,
                  Strip down (when they're restrained)
        Everyone: Restrain with Zip Ties; Untie someone zip-tied
        Medics:   Revive (a downed player), Heal (a hurt one, uses a Medkit)

    The server checks everything again (ALife_fnc_personAction,
    ALife_fnc_requestRevive).

    Modes:
        ["open", unit]
        ["fine", unit]      - how much to fine them (CfgALife >> fines[])
*/

params ["_mode", ["_target", objNull]];

if (isNull _target || { _target == player }) exitWith {};
private _faction = missionNamespace getVariable ["ALife_faction", "civ"];
private _name = _target getVariable ["alife_name", name _target];
private _have = { ((missionNamespace getVariable ["ALife_phone", createHashMap]) getOrDefault ["items", []]) findIf { (_x select 0) == _this && { (_x select 1) > 0 } } >= 0 };
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };

switch (_mode) do {
    case "open": {
        private _options = [];
        if (!alive _target) then {
            if (_faction == "medic" && { _target getVariable ["alife_downed", false] }) then {
                _options pushBack ["Revive", { ["start", _this] spawn ALife_fnc_revive; }, _target];
            };
        } else {
            private _restrained = _target getVariable ["alife_restrained", false];
            private _zipTied = (_target getVariable ["alife_restrainedBy", ""]) == "zipties";
            if (_faction == "cop") then {
                _options pushBack [["Restrain", "Unrestrain"] select _restrained, { [player, ["restrain", "unrestrain"] select (_this getVariable ["alife_restrained", false]), _this] remoteExecCall ["ALife_fnc_personAction", 2]; }, _target];
                _options pushBack ["Fine", { ["fine", _this] call ALife_fnc_personMenu; }, _target];
                if (_restrained) then {
                    _options pushBack ["Strip down", { [player, "strip", _this] remoteExecCall ["ALife_fnc_personAction", 2]; }, _target];
                };
            } else {
                if (!_restrained && { "zipties" call _have }) then {
                    _options pushBack ["Zip Tie", { [player, "restrain", _this] remoteExecCall ["ALife_fnc_personAction", 2]; }, _target];
                };
                if (_restrained && { _zipTied }) then {
                    _options pushBack ["Untie", { [player, "unrestrain", _this] remoteExecCall ["ALife_fnc_personAction", 2]; }, _target];
                };
            };
            if (_faction == "medic") then {
                _options pushBack ["Heal", { ["heal", _this] spawn ALife_fnc_personAct; }, _target];
            };
        };
        if (_options isEqualTo []) exitWith {
            [format ["Nothing to do with %1 right now.", _name], "info"] call ALife_fnc_toast;
        };
        ["open", _name, _options] call ALife_fnc_radial;
    };

    case "fine": {
        if (_faction != "cop") exitWith {};
        private _options = (getArray (missionConfigFile >> "CfgALife" >> "fines") select [0, 7]) apply {
            [_x call _fnc_money, { [player, "fine", _this select 0, _this select 1] remoteExecCall ["ALife_fnc_personAction", 2]; }, [_target, _x]]
        };
        _options pushBack ["Back", { ["open", _this] call ALife_fnc_personMenu; }, _target];
        ["open", format ["Fine %1", _name], _options] call ALife_fnc_radial;
    };
};
