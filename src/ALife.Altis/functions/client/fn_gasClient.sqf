/*
    ALife_fnc_gasClient (client; "rob", "robDone", "robFailed" and "alarm"
    come from the server -- ALife_fnc_gasStation)

    Gas stations on this side: Altis's fuel pumps stop filling vehicles for
    free (they're paid for at the pump), the robber's countdown, and the
    police alarm (a marker per station).

    Modes:
        ["init"]                                - from initPlayerLocal.sqf
        ["menu", [vehicle, withVehicleMenu]]    - the pump wheel: fill up, cash or bank (and the vehicle's own menu)
        ["rob", [seconds, range, cashier]]      - the robber's countdown
        ["robDone"] / ["robFailed"]
        ["alarm", [station, on, position, text]] - police only
*/

params [["_mode", "", [""]], ["_arg", []]];

private _cfg = missionConfigFile >> "CfgALife" >> "GasStations";

switch (_mode) do {
    case "init": {
        // Pumps (the terrain's fuel stations) hold no free fuel.
        { _x setFuelCargo 0; } forEach (nearestTerrainObjects [[worldSize / 2, worldSize / 2], ["FUELSTATION"], worldSize, false]);
    };

    case "menu": {
        _arg params ["_veh", ["_withVehicle", false]];
        private _capacity = getNumber (configFile >> "CfgVehicles" >> typeOf _veh >> "fuelCapacity") max 10;
        private _litres = round (_capacity * (1 - fuel _veh));
        private _name = getText (configFile >> "CfgVehicles" >> typeOf _veh >> "displayName");
        private _options = [];
        private _sub = format ["%1 is full", _name];
        if (_litres >= 1) then {
            private _price = ceil (_litres * getNumber (_cfg >> "pricePerLitre")) max 1;
            _options pushBack [format ["Fill up: $%1 cash", [_price] call BIS_fnc_numberText], { [player, "refuel", [_this, "cash"]] remoteExecCall ["ALife_fnc_gasStation", 2]; }, _veh];
            _options pushBack [format ["Fill up: $%1 bank", [_price] call BIS_fnc_numberText], { [player, "refuel", [_this, "bank"]] remoteExecCall ["ALife_fnc_gasStation", 2]; }, _veh];
            _sub = format ["%1: %2 L at $%3 a litre", _name, _litres, getNumber (_cfg >> "pricePerLitre")];
        };
        if (_withVehicle) then { _options pushBack ["Vehicle", { ["open", _this] call ALife_fnc_vehicleMenu; }, _veh]; };
        if (_options isEqualTo []) exitWith { [format ["The %1 is full.", _name], "info"] call ALife_fnc_toast; };
        ["open", "Fuel pump", _options, _sub] call ALife_fnc_radial;
    };

    case "rob": {
        _arg params ["_seconds", "_range", "_cashier"];
        ALife_robbing = true;
        [_seconds, _range, _cashier] spawn {
            params ["_seconds", "_range", "_cashier"];
            private _began = diag_tickTime;
            "ALife_progress" cutRsc ["ALife_Progress", "PLAIN", 0, false];
            while { missionNamespace getVariable ["ALife_robbing", false] } do {
                private _left = _seconds - (diag_tickTime - _began);
                private _d = uiNamespace getVariable ["ALife_Progress", displayNull];
                if (!isNull _d) then {
                    (_d displayCtrl 9201) ctrlSetText format ["Emptying the till... %1 s  (stay within %2 m)", ceil (_left max 0), _range];
                    (_d displayCtrl 9202) progressSetPosition (((diag_tickTime - _began) / _seconds) min 1);
                };
                if (_left < -10) exitWith {}; // the server's answer is late: stop showing it
                sleep 0.2;
            };
            "ALife_progress" cutText ["", "PLAIN"];
        };
        ["Hands up! Keep them covered and stay close.", "warning", "Robbery"] call ALife_fnc_toast;
    };

    case "robDone": { ALife_robbing = false; };

    case "robFailed": {
        ALife_robbing = false;
        ["The robbery failed: you went too far, or you're down.", "error", "Robbery"] call ALife_fnc_toast;
    };

    case "alarm": {
        _arg params [["_station", ""], ["_on", false], ["_pos", [0, 0, 0]], ["_text", ""]];
        if ((missionNamespace getVariable ["ALife_faction", "civ"]) != "cop") exitWith {};
        private _m = "alife_gas_alarm_" + _station;
        deleteMarkerLocal _m;
        if (_on) then {
            createMarkerLocal [_m, _pos];
            _m setMarkerTypeLocal "mil_warning";
            _m setMarkerColorLocal "ColorRed";
            _m setMarkerTextLocal "Gas station robbery";
        };
        if (_text != "") then { [_text, ["info", "error"] select _on, "Gas station", 12] call ALife_fnc_toast; };
    };
};
