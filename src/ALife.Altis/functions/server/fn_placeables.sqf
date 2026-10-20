/*
    ALife_fnc_placeables (server)

    Police placeables (CfgItems use = "place", ALife_fnc_requestUseItem):

        [spikeStrip]               - start watching a spike strip: any land
                                     vehicle within spikeRange that's moving
                                     loses its tyres (once per vehicle)
        ["pickup", unit, object]   - police (called by the client: Windows
                                     on a placed object): it's removed and
                                     its item goes back to them
        ["move", unit, object, positionATL, direction]
                                   - police: it goes where they chose
                                     (ALife_fnc_placeMode)
*/

params [["_a", objNull], ["_unit", objNull, [objNull]], ["_object", objNull, [objNull]], ["_at", [], [[]]], ["_dir", 0, [0]]];

if (!isServer) exitWith {};

if (_a isEqualType objNull) exitWith {
    _a spawn {
        private _strip = _this;
        private _range = getNumber (missionConfigFile >> "CfgALife" >> "Placeables" >> "spikeRange");
        private _popped = [];
        while { !isNull _strip } do {
            {
                if (alive _x && { speed _x > 3 } && { !(_x in _popped) } && { !(_x isKindOf "Tank") }) then {
                    _popped pushBack _x;
                    if (local _x) then { [_x] call ALife_fnc_vehicleTyres; } else { ["tyres", _x, []] remoteExecCall ["ALife_fnc_vehicleApply", _x]; };
                    diag_log format ["[ALife] spike strip popped the tyres of %1 (%2)", typeOf _x, _x getVariable ["alife_plate", ""]];
                };
            } forEach (_strip nearEntities [["Car"], _range]);
            sleep 0.25;
        };
    };
};

if (!(_a in ["pickup", "move"]) || { isNull _unit } || { isNull _object }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", owner _unit]; };
(_object getVariable ["alife_placed", []]) params [["_placedBy", ""], ["_class", ""]];
if (_class == "" || { !alive _unit } || { (_unit getVariable ["alife_faction", ""]) != "cop" }) exitWith {};
private _pcfg = missionConfigFile >> "CfgALife" >> "Placeables";

if (_a == "move") exitWith {
    if (!(_at isEqualTypeArray [0, 0, 0]) || { surfaceIsWater _at }) exitWith { "It can't go there." call _fnc_reply; };
    if (_unit distance2D _at > getNumber (_pcfg >> "placeRange") + 2) exitWith { "That's too far from you." call _fnc_reply; };
    if (_object distance2D _at > getNumber (_pcfg >> "moveRange")) exitWith { "That's too far to carry it. Pick it up instead." call _fnc_reply; };
    _object setDir _dir;
    _object setPosATL [_at select 0, _at select 1, 0];
    diag_log format ["[ALife] %1 moved a %2 to %3", getPlayerUID _unit, _class, _at];
};

if (_unit distance _object > getNumber (missionConfigFile >> "CfgALife" >> "Placeables" >> "pickupRange") + 1) exitWith { "Get closer." call _fnc_reply; };
_object setVariable ["alife_placed", [], true]; // nobody else picks it up too
deleteVehicle _object;
private _name = getText (missionConfigFile >> "CfgItems" >> _class >> "displayName");
if ([getPlayerUID _unit, "cop", _class, 1] call ALife_fnc_itemsChange) then {
    [format ["Picked up the %1.", _name], "success"] call _fnc_reply;
    [_unit] call ALife_fnc_requestPhoneData;
} else {
    format ["Picked up the %1, but you can't carry it.", _name] call _fnc_reply;
};
