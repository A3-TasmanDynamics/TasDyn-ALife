/*
    ALife_fnc_jailBail (server, called by the client: Windows in jail -> Pay bail)

    Arguments:
        0: OBJECT - the prisoner
*/

params [["_unit", objNull, [objNull]]];
if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith { ["spoof", _unit, "jailBail", remoteExecutedOwner] call ALife_fnc_anticheat; };
if (isRemoteExecuted && { ["rate", _unit, "bank"] call ALife_fnc_anticheat }) exitWith {};
["bail", _unit] call ALife_fnc_jail;
