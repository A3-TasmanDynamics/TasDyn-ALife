/*
    ALife_fnc_survival (client)

    Food, water and the player HUD (ui\hud.hpp), as set in
    CfgALife >> Survival:

      - food and water (0-100, ALife_food / ALife_water) drop every second
        while you're spawned; eating / drinking brings them back
        (ALife_fnc_itemUsed);
      - at 0 you lose health until you eat or drink, with a notice when one
        gets low;
      - the values are copied to your unit for the server
        (alife_food / alife_water), which saves them with your gear;
      - the HUD shows health, food and water in the bottom-right corner.

    Started once from initPlayerLocal.sqf; runs for the whole session. A
    spawn sets the starting values (ALife_fnc_spawnApproved).
*/

if (!hasInterface) exitWith {};

private _cfg = missionConfigFile >> "CfgALife" >> "Survival";
private _foodRate = getNumber (_cfg >> "foodPerMinute") / 60;
private _waterRate = getNumber (_cfg >> "waterPerMinute") / 60;
private _emptyDamage = getNumber (_cfg >> "emptyDamagePerMinute") / 60;
private _warnAt = getNumber (_cfg >> "warnAt");
private _showHud = getNumber (_cfg >> "hud") == 1;

if (isNil "ALife_food") then { ALife_food = 100; };
if (isNil "ALife_water") then { ALife_water = 100; };
private _warned = [false, false];
private _sent = [objNull, -1, -1]; // [unit, food, water] last copied for the server
private _last = diag_tickTime;

while { true } do {
    private _dt = diag_tickTime - _last;
    _last = diag_tickTime;
    private _active = (missionNamespace getVariable ["ALife_spawned", false]) && { alive player };

    if (_active) then {
        ALife_food = (ALife_food - _foodRate * _dt) max 0;
        ALife_water = (ALife_water - _waterRate * _dt) max 0;

        // Empty: health drains (one rate each for food and water).
        private _empty = ([ALife_food, ALife_water] select { _x <= 0 }) apply { _emptyDamage * _dt };
        if (_empty isNotEqualTo [] && { !(player getVariable ["alife_downed", false]) }) then {
            private _loss = 0;
            { _loss = _loss + _x; } forEach _empty;
            player setDamage ((damage player + _loss) min 1);
        };

        // A notice when one gets low (again after eating / drinking).
        {
            _x params ["_value", "_i", "_text"];
            if (_value <= _warnAt && { !(_warned select _i) }) then {
                _warned set [_i, true];
                [_text, "warning"] call ALife_fnc_toast;
            };
            if (_value > _warnAt) then { _warned set [_i, false]; };
        } forEach [
            [ALife_food, 0, "You're getting hungry. Eat something soon."],
            [ALife_water, 1, "You're getting thirsty. Drink something soon."]
        ];

        // For the server's saves: only when the whole number changes.
        private _food = round ALife_food;
        private _water = round ALife_water;
        if (_sent isNotEqualTo [player, _food, _water]) then {
            player setVariable ["alife_food", _food, 2];
            player setVariable ["alife_water", _water, 2];
            _sent = [player, _food, _water];
        };
    };

    // The HUD: on while you're spawned.
    private _hud = uiNamespace getVariable ["ALife_Hud", displayNull];
    if (_showHud && { _active }) then {
        if (isNull _hud) then {
            "ALife_hud" cutRsc ["ALife_Hud", "PLAIN", 0, false];
            _hud = uiNamespace getVariable ["ALife_Hud", displayNull];
        };
        if (!isNull _hud) then {
            {
                _x params ["_row", "_value"];
                (_hud displayCtrl (9302 + _row * 10)) progressSetPosition (_value / 100);
                (_hud displayCtrl (9303 + _row * 10)) ctrlSetText str round _value;
            } forEach [[1, (1 - damage player) * 100], [2, ALife_food], [3, ALife_water]];
        };
    } else {
        if (!isNull _hud) then { "ALife_hud" cutText ["", "PLAIN"]; };
    };

    uiSleep 0.5;
};
