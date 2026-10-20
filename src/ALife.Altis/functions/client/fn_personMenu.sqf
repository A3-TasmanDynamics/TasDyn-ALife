/*
    ALife_fnc_personMenu (client)

    The radial menu for a player you're looking at (Windows key,
    ALife_fnc_interact):

        Police:   Restrain / Unrestrain (also Left Shift + R), Fine, Wanted
                  (add a crime); when they're restrained: Arrest (at the
                  jail), Escort / Stop escorting, Put in vehicle, Strip down
        Everyone: Restrain with Zip Ties; Untie someone zip-tied
        Medics:   Revive (a downed player), Heal (a hurt one, uses a Medkit)

    The server checks everything again (ALife_fnc_personAction,
    ALife_fnc_requestRevive).

    Modes:
        ["open", unit]
        ["fine", unit]      - how much to fine them (CfgALife >> fines[])
        ["wanted", unit, page] - which crime to add (CfgCrimes), 6 a page
*/

params ["_mode", ["_target", objNull], ["_page", 0]];

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
                _options pushBack ["Wanted", { ["wanted", _this, 0] call ALife_fnc_personMenu; }, _target];
                if (_restrained) then {
                    if ([player, missionConfigFile >> "CfgJail"] call ALife_fnc_atPlace) then {
                        _options pushBack ["Arrest", { [player, "arrest", _this] remoteExecCall ["ALife_fnc_personAction", 2]; }, _target];
                    };
                    private _escorting = attachedTo _target == player;
                    _options pushBack [["Escort", "Stop escorting"] select _escorting, { [player, ["escort", "stopEscort"] select (attachedTo _this == player), _this] remoteExecCall ["ALife_fnc_personAction", 2]; }, _target];
                    _options pushBack ["Put in vehicle", { [player, "putIn", _this] remoteExecCall ["ALife_fnc_personAction", 2]; }, _target];
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
        // Police see a civilian's bounty under their name.
        private _bounty = _target getVariable ["alife_bounty", 0];
        private _sub = [];
        if (_target getVariable ["alife_surrendered", false]) then { _sub pushBack "Surrendered"; };
        if (_faction == "cop" && { _bounty > 0 }) then { _sub pushBack format ["Wanted %1", _bounty call _fnc_money]; };
        ["open", _name, _options, _sub joinString "  -  "] call ALife_fnc_radial;
    };

    case "wanted": {
        if (_faction != "cop") exitWith {};
        private _crimes = "true" configClasses (missionConfigFile >> "CfgCrimes");
        private _pages = ceil (count _crimes / 6);
        _page = (_page max 0) min (_pages - 1);
        private _options = (_crimes select [_page * 6, 6]) apply {
            [getText (_x >> "name"), { [player, "wanted", _this select 0, _this select 1] remoteExecCall ["ALife_fnc_personAction", 2]; }, [_target, configName _x]]
        };
        _options pushBack [["More", "First page"] select (_page >= _pages - 1), { ["wanted", _this select 0, _this select 1] call ALife_fnc_personMenu; }, [_target, [_page + 1, 0] select (_page >= _pages - 1)]];
        _options pushBack ["Back", { ["open", _this] call ALife_fnc_personMenu; }, _target];
        ["open", format ["Add a crime: %1", _name], _options, format ["Page %1 of %2", _page + 1, _pages]] call ALife_fnc_radial;
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
