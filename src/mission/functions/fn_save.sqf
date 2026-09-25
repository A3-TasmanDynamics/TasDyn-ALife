/*
    File: fn_save.sqf
    Author: Tasman Dynamics

    Description:
        Persists one allowlisted field for a player. Implements the "save"
        side of docs/DATA_CONTRACT.md — see that doc for which fields are
        absolute-set vs. a signed delta (civ_cash/cop_cash/medic_cash are
        deltas with their own idempotency check; everything else is
        absolute-set and doesn't need one).

        The field allowlist below is defense in depth, not the only check —
        the C++ extension independently re-validates field against its own
        allowlist before it ever reaches a query. Neither side trusts the
        other to have already checked.

        Server-only — allowlisted in CfgRemoteExec.hpp as ALife_fnc_save.

    Parameter(s):
        0: STRING - uid
        1: STRING - field
        2: STRING - value — a delta (e.g. "-500") for *_cash fields,
                    the new absolute value for everything else
        3: STRING - requestToken — required for *_cash fields (idempotency),
                    ignored for absolute-set fields

    Returns:
        STRING - "OK", "ERROR", or "DUPLICATE" (cash fields only)
*/

params ["_uid", "_field", "_value", "_requestToken"];

if (!isServer) exitWith { "ERROR" };

private _allowedFields = [
    "name",
    "civ_cash", "cop_cash", "medic_cash",
    "civ_licence", "cop_licence", "medic_licence",
    "civ_gear", "cop_gear", "medic_gear",
    "cop_level", "medic_level",
    "cop_dept", "medic_dept",
    "civ_alive", "cop_alive", "medic_alive",
    "civ_position", "cop_position", "medic_position"
];

if !(_field in _allowedFields) exitWith {
    diag_log format ["[ALife] save rejected — field not allowlisted: %1", _field];
    "ERROR"
};

"tasdyn_alife" callExtension ["save", [_uid, _field, _value, _requestToken]]
