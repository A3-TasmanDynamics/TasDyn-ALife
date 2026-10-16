// Server start-up. Everything lives in functions\server\fn_serverInit.sqf.
// (The guard waits out the short gap before the function library is
// compiled on a dedicated server.)

waitUntil { !isNil "ALife_fnc_serverInit" };
[] call ALife_fnc_serverInit;
