/*
    ALife_fnc_jailBail (server, called by the client: Windows in jail -> Pay bail)

    Arguments:
        0: OBJECT - the prisoner
*/

params [["_unit", objNull, [objNull]]];
if (!isServer || { isNull _unit }) exitWith {};
if (isRemoteExecuted && { remoteExecutedOwner != owner _unit }) exitWith {};
["bail", _unit] call ALife_fnc_jail;
