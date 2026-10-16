/*
    ALife_fnc_extension (server)

    The only place that talks to the database DLL (tasdyn_alife_x64.dll,
    src/cpp_extension). Commands: "ping", "load", "save", "combatlog".

    Arguments:
        0: STRING - command
        1: ARRAY  - arguments (sent as strings)

    Returns:
        STRING - the DLL's answer, or "ERROR:<why>" if the DLL couldn't be
        called at all (e.g. the DLL isn't next to arma3server_x64.exe).
*/

params ["_command", ["_args", []]];

if (!isServer) exitWith { "ERROR:not the server" };

// The array form returns [answer, returnCode, errorCode]; a non-zero
// errorCode means the DLL never ran.
private _result = "tasdyn_alife" callExtension [_command, _args];
_result params [["_answer", ""], ["_returnCode", 0], ["_errorCode", 0]];

if (_errorCode != 0) exitWith {
    format ["ERROR:database extension could not be called (error %1) -- is tasdyn_alife_x64.dll deployed next to the server?", _errorCode]
};
if (_answer == "") exitWith {
    "ERROR:no answer from the database extension -- is tasdyn_alife_x64.dll deployed next to the server?"
};

_answer
