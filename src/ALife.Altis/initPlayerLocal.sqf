// A player has joined: black screen, load the character, spawn menu.
// See functions\client\fn_startSession.sqf.

if (!hasInterface) exitWith {};

waitUntil { !isNil "ALife_fnc_startSession" };
[] spawn ALife_fnc_startSession;
