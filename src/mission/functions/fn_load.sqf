/*
    File: fn_load.sqf
    Author: Tasman Dynamics

    Description:
        Loads a player's full record from the backend on join. Implements
        the "load" side of docs/DATA_CONTRACT.md — see that doc for the
        full field list and why the response is parsed this way (SQF's
        parseSimpleArray has no object literal, so every structured field
        is an array of [key,value] pairs, not JSON `{}`).

        Server-only — allowlisted in CfgRemoteExec.hpp as ALife_fnc_load.
        Never call this from a client; it isn't reachable from one.

    Parameter(s):
        0: OBJECT - the player's unit

    Returns:
        HASHMAP - the parsed record on OK, or a HashMap containing only
        ["status","ERROR"] on failure. ALWAYS check "status" before reading
        any other key — an ERROR response has none of the other keys.
*/

params ["_unit"];

if (!isServer) exitWith { createHashMapFromArray [["status", "ERROR"]] };

private _uid = getPlayerUID _unit;
private _rawResponse = "tasdyn_alife" callExtension ["load", [_uid]];
private _record = (_rawResponse call parseSimpleArray) createHashMapFromArray;

if ((_record getOrDefault ["status", "ERROR"]) != "OK") then {
    diag_log format ["[ALife] load failed for uid %1: %2", _uid, _rawResponse];
};

_record
