/*
    ALife_fnc_sendMessage (server, called by the client)

    Phone messages. "police" / "ems" go to everyone currently playing that
    faction; "player" goes to one player. Anonymous messages hide the
    sender from the recipients -- the server log always has the real one.

    Arguments:
        0: OBJECT - sender's unit
        1: STRING - "police", "ems" or "player"
        2: OBJECT - recipient (for "player")
        3: STRING - text
        4: BOOL   - anonymous
*/

params [["_unit", objNull, [objNull]], ["_channel", "", [""]], ["_target", objNull, [objNull]], ["_text", "", [""]], ["_anon", false, [false]]];

if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};

private _owner = owner _unit;
private _fnc_reply = { (if (_this isEqualType []) then { _this } else { [_this, "warning"] }) remoteExecCall ["ALife_fnc_notify", _owner]; };
private _uid = getPlayerUID _unit;
if ((_unit getVariable ["alife_faction", ""]) == "") exitWith {};

// One message every few seconds per player.
if (isNil "ALife_messageTimes") then { ALife_messageTimes = createHashMap; };
private _cooldown = getNumber (missionConfigFile >> "CfgALife" >> "messageCooldown");
if (diag_tickTime - (ALife_messageTimes getOrDefault [_uid, -1e6]) < _cooldown) exitWith {
    "Slow down -- wait a moment before sending another message." call _fnc_reply;
};

_text = (_text splitString (toString [10, 13])) joinString " ";
_text = _text select [0, getNumber (missionConfigFile >> "CfgALife" >> "messageMaxLength")];
if ((_text splitString " ") isEqualTo []) exitWith { "Type a message first." call _fnc_reply; };

private _recipients = switch (_channel) do {
    case "police": { allPlayers select { (_x getVariable ["alife_faction", ""]) == "cop" } };
    case "ems": { allPlayers select { (_x getVariable ["alife_faction", ""]) == "medic" } };
    case "player": { [_target] select { !isNull _x && { isPlayer _x } && { (_x getVariable ["alife_faction", ""]) != "" } } };
    default { [] };
};
if (_recipients isEqualTo []) exitWith {
    (switch (_channel) do {
        case "police": { "No police are on duty." };
        case "ems": { "No medics are on duty." };
        default { "They aren't available." };
    }) call _fnc_reply;
};

ALife_messageTimes set [_uid, diag_tickTime];
private _from = ["", name _unit] select !_anon;
private _hour = floor daytime;
private _minute = floor ((daytime - _hour) * 60);
private _time = format ["%1:%2%3", _hour, ["", "0"] select (_minute < 10), _minute];

diag_log format ["[ALife][SMS] %1 (%2)%3 -> %4 (%5 recipient(s)): %6",
    name _unit, _uid, ["", " [anonymous]"] select _anon, _channel, count _recipients, _text];

{
    [_from, _channel, _text, _time] remoteExecCall ["ALife_fnc_messageReceived", owner _x];
} forEach _recipients;

[(switch (_channel) do {
    case "police": { format ["Message sent to the police (%1 on duty).", count _recipients] };
    case "ems": { format ["Message sent to EMS (%1 on duty).", count _recipients] };
    default { format ["Message sent to %1.", name (_recipients select 0)] };
}), "success"] call _fnc_reply;
