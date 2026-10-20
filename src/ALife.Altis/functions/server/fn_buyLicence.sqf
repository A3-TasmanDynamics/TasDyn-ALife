/*
    ALife_fnc_buyLicence (server, called by the client: a processor
    worker's menu, or the licence office window)

    Buys a licence at its CfgLicences price, paid the licence's way
    ("bank", or "cash" -- drugs are cash only), into the player's
    <faction>_licence (saved). Either:

      - at a processor (CfgProcessing class): the licence it uses;
        processing there is then free;
      - at a licence office (CfgShops type "licences", for its side): one
        of the licences[] it sells.

    Arguments:
        0: OBJECT - the player's unit
        1: STRING - CfgProcessing class, or CfgShops licence office
        2: STRING - licence office: the CfgLicences key
*/

params [["_unit", objNull, [objNull]], ["_process", "", [""]], ["_wanted", "", [""]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;
private _record = ALife_sessions getOrDefault [_uid, createHashMap];
private _faction = _unit getVariable ["alife_faction", ""];
private _office = missionConfigFile >> "CfgShops" >> _process;
private _isOffice = isClass _office && { getText (_office >> "type") == "licences" };
private _cfg = [missionConfigFile >> "CfgProcessing" >> _process, _office] select _isOffice;

if ((_record getOrDefault ["status", ""]) != "OK" || { !alive _unit } || { _faction == "" } || { !isClass _cfg }) exitWith {};
if (!([_unit, _cfg] call ALife_fnc_atPlace)) exitWith { (["You're not at the processor.", "You're not at the licence office."] select _isOffice) call _fnc_reply; };
if (_isOffice && { getText (_office >> "side") != _faction }) exitWith { "This office doesn't sell licences to your faction." call _fnc_reply; };
if (_isOffice && { !(_wanted in getArray (_office >> "licences")) }) exitWith { "That licence isn't sold here." call _fnc_reply; };

private _licence = [getText (_cfg >> "licence"), _wanted] select _isOffice;
private _lcfg = missionConfigFile >> "CfgLicences" >> _licence;
private _price = getNumber (_lcfg >> "price");
private _method = getText (_lcfg >> "payment");
private _name = getText (_lcfg >> "displayName");
// "Driver Licence" as it is; "Oil Processing" -> "Oil Processing licence".
if (_name find "Licence" < 0) then { _name = _name + " licence"; };
if (_licence == "" || { _price <= 0 } || { !(_method in ["bank", "cash"]) }) exitWith { "That licence isn't for sale here." call _fnc_reply; };

private _key = _faction + "_licence";
private _licences = (_record getOrDefault [_key, []]) select { _x isEqualType "" };
if (_licence in _licences) exitWith { format ["You already have the %1.", _name] call _fnc_reply; };

private _paid = [_uid, _faction, _record, _price, _method, _name] call ALife_fnc_shopCharge;
if (_paid != "OK") exitWith {
    (if (_paid in ["INSUFFICIENT", "ERROR"]) then {
        format ["The %1 costs $%2 %3. You don't have enough.", _name, [_price] call BIS_fnc_numberText, ["in cash", "from your bank"] select (_method == "bank")]
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

diag_log format ["[ALife] %1 bought the %2 licence for $%3 (%4)%5", _uid, _licence, _price, _method, [" at a processor", " at the licence office"] select _isOffice];
[format ["Bought the %1 for $%2.%3", _name, [_price] call BIS_fnc_numberText, [" Processing here is free now.", ""] select _isOffice], "success"] call _fnc_reply;
[_unit] call ALife_fnc_requestPhoneData;
if (_isOffice) then { ["bought", _licences] remoteExecCall ["ALife_fnc_licenceShop", _owner]; };
