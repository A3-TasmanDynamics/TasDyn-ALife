/*
    ALife_fnc_buyLicence (server, called by the client: the processor
    worker's menu)

    Buys the licence a processor uses (CfgProcessing >> licence), at that
    processor: its CfgLicences price, paid the licence's way -- "bank" for
    resources, "cash" only for drugs. The licence is added to the player's
    <faction>_licence and saved; processing there is then free.

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - CfgProcessing class (the processor they're at)
*/

params [["_unit", objNull, [objNull]], ["_process", "", [""]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
private _cfg = missionConfigFile >> "CfgProcessing" >> _process;

if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" } || { !isClass _cfg }) exitWith {};
if (!([_unit, _cfg] call ALife_fnc_atPlace)) exitWith { "You're not at the processor." call _fnc_reply; };

private _licence = getText (_cfg >> "licence");
private _lcfg = missionConfigFile >> "CfgLicences" >> _licence;
private _price = getNumber (_lcfg >> "price");
private _method = getText (_lcfg >> "payment");
private _name = getText (_lcfg >> "displayName");
if (_licence == "" || { _price <= 0 } || { !(_method in ["bank", "cash"]) }) exitWith { "That licence isn't for sale here." call _fnc_reply; };

private _key = _faction + "_licence";
private _licences = (_record getOrDefault [_key, []]) select { _x isEqualType "" };
if (_licence in _licences) exitWith { format ["You already have the %1 licence.", _name] call _fnc_reply; };

private _paid = [_uid, _faction, _record, _price, _method, _name + " licence"] call ALife_fnc_shopCharge;
if (_paid != "OK") exitWith {
    (if (_paid in ["INSUFFICIENT", "ERROR"]) then {
        format ["The %1 licence costs $%2 %3. You don't have enough.", _name, [_price] call BIS_fnc_numberText, ["in cash", "from your bank"] select (_method == "bank")]
    } else {
        "The purchase didn't go through. Try again."
    }) call _fnc_reply;
};

_licences pushBack _licence;
private _saved = ["save", [_uid, _key, str _licences, "-"]] call ALife_fnc_extension;
if (_saved != "OK") exitWith {
    diag_log format ["[ALife] %1 paid $%2 for the %3 licence but saving it failed: %4 -- needs a refund or a manual grant", _uid, _price, _licence, _saved];
    "You were charged but the licence didn't save. Tell staff (it's in the server log)." call _fnc_reply;
};
_record set [_key, _licences];

diag_log format ["[ALife] %1 bought the %2 licence for $%3 (%4)", _uid, _licence, _price, _method];
[format ["Bought the %1 licence for $%2. Processing here is free now.", _name, [_price] call BIS_fnc_numberText], "success"] call _fnc_reply;
[_unit] call ALife_fnc_requestPhoneData;
