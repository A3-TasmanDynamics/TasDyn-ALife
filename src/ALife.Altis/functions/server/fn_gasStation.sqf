/*
    ALife_fnc_gasStation (server, called by the client)

    Gas stations (CfgALife >> GasStations).

    Fuel: every Altis fuel pump (the terrain's FUELSTATION objects) fills a
    vehicle up -- what it's missing, in litres (its fuelCapacity), at
    pricePerLitre, paid cash or bank. Pumps don't give fuel away
    (ALife_fnc_gasClient "init").

    Robbery: a cashier (CfgShops >> gas_station npcs[]) held up by a civilian
    with a weapon in their hands. They stay within robRange for robTime
    seconds (alive, up, not restrained) and the till's theirs: payoutMin to
    payoutMax in cash. Needs minCops police online; each station once per
    `cooldown`. Police are alerted with a map marker; the robber is wanted
    (crime).

    Arguments: [unit, mode, arg]:
        [unit, "refuel", [vehicle, "cash" | "bank"]]
        [unit, "rob", cashier]
*/

params [["_unit", objNull, [objNull]], ["_mode", "", [""]], ["_arg", []]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith { ["spoof", _unit, "gasStation", remoteExecutedOwner] call ALife_fnc_anticheat; };
if (isRemoteExecuted && { ["rate", _unit, "gas"] call ALife_fnc_anticheat }) exitWith {};

private _cfg = missionConfigFile >> "CfgALife" >> "GasStations";
private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" }) exitWith {};

switch (_mode) do {
    case "refuel": {
        _arg params [["_veh", objNull, [objNull]], ["_method", "", [""]]];
        if (isNull _veh || { !alive _veh } || { !(_method in ["cash", "bank"]) }) exitWith {};
        if !(_veh isKindOf "LandVehicle" || { _veh isKindOf "Air" } || { _veh isKindOf "Ship" }) exitWith {};
        if (_unit distance _veh > getNumber (_cfg >> "vehicleRange") + 2) exitWith { "Get closer to the vehicle." call _fnc_reply; };
        if ((nearestTerrainObjects [_unit, ["FUELSTATION"], getNumber (_cfg >> "pumpRange") + 2, false]) isEqualTo []
            && { (nearestTerrainObjects [_veh, ["FUELSTATION"], getNumber (_cfg >> "pumpRange") + 4, false]) isEqualTo [] }) exitWith { "There's no pump here." call _fnc_reply; };
        private _capacity = getNumber (configFile >> "CfgVehicles" >> typeOf _veh >> "fuelCapacity") max 10;
        private _litres = round (_capacity * (1 - fuel _veh));
        if (_litres < 1) exitWith { "It's already full." call _fnc_reply; };
        private _price = ceil (_litres * getNumber (_cfg >> "pricePerLitre")) max 1;
        private _name = getText (configFile >> "CfgVehicles" >> typeOf _veh >> "displayName");
        private _paid = [_uid, _faction, _record, _price, _method, format ["Fuel: %1 L for the %2", _litres, _name]] call ALife_fnc_shopCharge;
        if (_paid != "OK") exitWith {
            (if (_paid in ["INSUFFICIENT", "ERROR"]) then {
                format ["Filling it up costs $%1. Not enough %2.", [_price] call BIS_fnc_numberText, ["cash", "money in the bank"] select (_method == "bank")]
            } else { "That didn't go through. Try again." }) call _fnc_reply;
        };
        if (local _veh) then { _veh setFuel 1; } else { ["fuel", _veh, 1] remoteExecCall ["ALife_fnc_itemUsed", _veh]; };
        ["log", _unit, "gas.refuel", [["vehicle", typeOf _veh], ["litres", _litres], ["total", _price], ["paid_by", _method]]] call ALife_fnc_anticheat;
        [format ["Filled up the %1: %2 L, $%3.", _name, _litres, [_price] call BIS_fnc_numberText], "success"] call _fnc_reply;
        [_unit] call ALife_fnc_requestPhoneData;
    };

    case "rob": {
        private _cashier = _arg;
        if !(_cashier isEqualType objNull) exitWith {};
        private _name = vehicleVarName _cashier;
        if (isNull _cashier || { !(_name in getArray (missionConfigFile >> "CfgShops" >> "gas_station" >> "npcs")) }) exitWith {};
        if (_faction != "civ") exitWith { "Only civilians rob gas stations." call _fnc_reply; };
        if (_unit distance _cashier > 6) exitWith { "Get closer." call _fnc_reply; };
        if (currentWeapon _unit == "" || { vehicle _unit != _unit }) exitWith { "You'll need a weapon in your hands." call _fnc_reply; };
        if (isNil "ALife_gasRobbed") then { ALife_gasRobbed = createHashMap; };
        private _until = ALife_gasRobbed getOrDefault [_name, 0];
        if (_until < 0) exitWith { "Someone's already robbing it." call _fnc_reply; };
        if (time < _until) exitWith { format ["The till's empty. Come back in %1 min.", ceil ((_until - time) / 60)] call _fnc_reply; };
        private _cops = count (allPlayers select { (_x getVariable ["alife_faction", ""]) == "cop" });
        if (_cops < getNumber (_cfg >> "minCops")) exitWith { format ["There need to be at least %1 police on duty.", getNumber (_cfg >> "minCops")] call _fnc_reply; };

        ALife_gasRobbed set [_name, -1]; // in progress
        private _where = text ((nearestLocations [getPosATL _cashier, ["NameCity", "NameCityCapital", "NameVillage", "NameLocal"], 3000]) param [0, locationNull]);
        if (_where == "") then { _where = "a gas station"; } else { _where = format ["the %1 gas station", _where]; };
        // Wanted for it, police told.
        private _crime = missionConfigFile >> "CfgCrimes" >> getText (_cfg >> "crime");
        if (isClass _crime) then {
            private _bounty = getNumber (_crime >> "bounty");
            private _r = ["wanted", ["add", _uid, configName _crime, str _bounty, ""]] call ALife_fnc_extension;
            if (_r select [0, 3] == "OK:") then {
                private _total = (_record getOrDefault ["civ_bounty", 0]) + _bounty;
                _record set ["civ_bounty", _total];
                _unit setVariable ["alife_bounty", _total, true];
            };
        };
        {
            if ((_x getVariable ["alife_faction", ""]) == "cop") then { ["alarm", [_name, true, getPosATL _cashier, format ["Robbery in progress at %1!", _where]]] remoteExecCall ["ALife_fnc_gasClient", owner _x]; };
        } forEach allPlayers;
        _cashier playMoveNow "AmovPercMstpSsurWnonDnon"; // hands up
        ["rob", [getNumber (_cfg >> "robTime"), getNumber (_cfg >> "robRange"), _cashier]] remoteExecCall ["ALife_fnc_gasClient", _owner];
        diag_log format ["[ALife] %1 is robbing %2 (%3)", _uid, _name, _where];
        ["log", _unit, "robbery.gas_start", [["station", _name], ["where", _where]]] call ALife_fnc_anticheat;

        [_unit, _cashier, _name, _where] spawn {
            params ["_unit", "_cashier", "_name", "_where"];
            private _cfg = missionConfigFile >> "CfgALife" >> "GasStations";
            private _end = time + getNumber (_cfg >> "robTime");
            private _range = getNumber (_cfg >> "robRange");
            private _ok = true;
            while { time < _end } do {
                sleep 1;
                if (isNull _unit || { !alive _unit } || { _unit getVariable ["alife_downed", false] } || { _unit getVariable ["alife_restrained", false] }
                    || { _unit distance _cashier > _range }) exitWith { _ok = false; };
            };
            ALife_gasRobbed set [_name, time + getNumber (_cfg >> "cooldown")];
            _cashier playMoveNow "AmovPercMstpSsurWnonDnon_AmovPercMstpSnonWnonDnon";
            private _police = allPlayers select { (_x getVariable ["alife_faction", ""]) == "cop" };
            if (!_ok) exitWith {
                if (!isNull _unit) then { ["robFailed", []] remoteExecCall ["ALife_fnc_gasClient", owner _unit]; };
                { ["alarm", [_name, false, [0, 0, 0], format ["The robbery at %1 failed.", _where]]] remoteExecCall ["ALife_fnc_gasClient", owner _x]; } forEach _police;
                diag_log format ["[ALife] robbery of %1 failed", _name];
            };
            // The till: cash, a one-off token.
            private _uid = getPlayerUID _unit;
            private _faction = _unit getVariable ["alife_faction", "civ"];
            private _take = getNumber (_cfg >> "payoutMin") + floor random ((getNumber (_cfg >> "payoutMax") - getNumber (_cfg >> "payoutMin")) max 0);
            private _token = format ["gasrob-%1-%2-%3", _uid, _name, round (diag_tickTime * 1000)];
            private _r = ["save", [_uid, _faction + "_cash", _take toFixed 0, _token]] call ALife_fnc_extension;
            if (_r != "OK") exitWith { diag_log format ["[ALife] robbery payout for %1 failed: %2", _uid, _r]; };
            private _record = ALife_sessions getOrDefault [_uid, createHashMap];
            _record set [_faction + "_cash", (_record getOrDefault [_faction + "_cash", 0]) + _take];
            [format ["You emptied the till: $%1. Get out of here!", [_take] call BIS_fnc_numberText], "success"] remoteExecCall ["ALife_fnc_notify", owner _unit];
            ["robDone", []] remoteExecCall ["ALife_fnc_gasClient", owner _unit];
            [_unit] call ALife_fnc_requestPhoneData;
            { ["alarm", [_name, true, getPosATL _cashier, format ["The robbers got away with the till at %1!", _where]]] remoteExecCall ["ALife_fnc_gasClient", owner _x]; } forEach _police;
            diag_log format ["[ALife] %1 robbed %2 for $%3", _uid, _name, _take];
            ["log", _unit, "robbery.gas", [["station", _name], ["where", _where], ["cash", _take]]] call ALife_fnc_anticheat;
            // The marker stays a couple of minutes for the chase.
            sleep 120;
            { ["alarm", [_name, false, [0, 0, 0], ""]] remoteExecCall ["ALife_fnc_gasClient", owner _x]; } forEach (allPlayers select { (_x getVariable ["alife_faction", ""]) == "cop" });
        };
    };
};
