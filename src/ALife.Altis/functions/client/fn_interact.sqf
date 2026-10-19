/*
    ALife_fnc_interact (client, the Windows key)

    Everything you do with things around you goes through here (there are
    no scroll-wheel actions):

      - an ATM (any on the map): the bank menu;
      - a vehicle within 8 m: its radial menu (ALife_fnc_vehicleMenu);
      - a dropped bag: opens it in the inventory screen;
      - a shop's NPC (CfgShops npcs[]) or a processor's worker within
        5 m: that shop / processing;
      - a player within 3.5 m (or a downed one): their radial menu
        (ALife_fnc_personMenu) -- restrain, fine, heal, revive, ...;
      - in a gathering area (CfgResources >> CfgGathering): gather there;
      - anything else: your own inventory with the ground beside it, to
        drop things -- or, at your faction's shops (CfgShops) and / or a
        processor (CfgProcessing), a menu with those and Inventory.

    The server checks everything again.

    Returns:
        BOOL - true if the key press was used
*/

if (!alive player || { !(missionNamespace getVariable ["ALife_spawned", false]) } || { dialog }) exitWith { false };

private _target = cursorObject;
private _faction = missionNamespace getVariable ["ALife_faction", "civ"];

if ([_target] call ALife_fnc_isAtm && { player distance _target <= getNumber (missionConfigFile >> "CfgALife" >> "atmRange") + 1 }) exitWith {
    ["open"] call ALife_fnc_atm;
    true
};

private _isVehicle = !isNull _target && { alive _target } && { _target isKindOf "LandVehicle" || { _target isKindOf "Air" } || { _target isKindOf "Ship" } };
if (_isVehicle && { player distance _target < 8 } && { vehicle player == player }) exitWith {
    ["open", _target] call ALife_fnc_vehicleMenu;
    true
};

if (!isNull _target && { _target in (missionNamespace getVariable ["ALife_groundBags", []]) } && { player distance _target < 4 }) exitWith {
    player playActionNow "PutDown"; // reach down to it
    [player, _target] remoteExecCall ["ALife_fnc_containerOpen", 2];
    true
};

// Opening a shop by its kind (CfgShops placement name in _this).
private _openShop = {
    switch (getText (missionConfigFile >> "CfgShops" >> _this >> "type")) do {
        case "clothing": { ["open", _this] call ALife_fnc_clothingShop; };
        case "vehicles": { ["open", _this] call ALife_fnc_vehicleShop; };
        default { ["open", _this] call ALife_fnc_shop; };
    };
};

// A shop / processor NPC you're looking at.
private _near = !isNull _target && { !isPlayer _target } && { player distance _target <= 5 };
private _process = if (_near) then { _target getVariable ["alife_process", ""] } else { "" };
if (_process != "") exitWith {
    if (isNil "ALife_using") then { [player, _process] remoteExecCall ["ALife_fnc_requestProcess", 2]; };
    true
};
private _npcShop = "";
if (_near && { vehicleVarName _target != "" }) then {
    private _shops = "true" configClasses (missionConfigFile >> "CfgShops");
    private _i = _shops findIf { (vehicleVarName _target) in getArray (_x >> "npcs") };
    if (_i >= 0) then { _npcShop = configName (_shops select _i); };
};
if (_npcShop != "") exitWith { _npcShop call _openShop; true };

if (!isNull _target && { _target != player } && { _target isKindOf "CAManBase" } && { player distance _target < 3.5 }
    && { (alive _target && { isPlayer _target }) || { _target getVariable ["alife_downed", false] } }) exitWith {
    ["open", _target] call ALife_fnc_personMenu;
    true
};

// A gathering area: gather (the server checks the tool, room and area).
private _resource = "";
if (vehicle player == player) then {
    {
        private _cfg = _x;
        if ([player, _cfg, "zones"] call ALife_fnc_atPlace) exitWith { _resource = configName _cfg; };
    } forEach ("true" configClasses (missionConfigFile >> "CfgGathering"));
};
if (_resource != "") exitWith {
    if (isNil "ALife_using") then { [player, _resource] remoteExecCall ["ALife_fnc_requestGather", 2]; };
    true
};

// Your faction's shops you're standing at (CfgShops markers / NPCs + radius).
private _shops = [];
{
    private _info = [configName _x] call ALife_fnc_shopInfo;
    if (_info isNotEqualTo []) then {
        _info params ["_type", "_place", "_shop"];
        if (_type in ["market", "clothing", "vehicles"] && { getText (_shop >> "side") == _faction } && { [player, _place] call ALife_fnc_atPlace }) then {
            _shops pushBack [getText (_shop >> "name"), _openShop, configName _place];
        };
    };
} forEach ("true" configClasses (missionConfigFile >> "CfgShops"));

if (_shops isNotEqualTo []) exitWith {
    ["open", "Shop", (_shops select [0, 7]) + [["Inventory", { ["openGround"] call ALife_fnc_transfer; }, []]]] call ALife_fnc_radial;
    true
};

["openGround"] call ALife_fnc_transfer;
true
