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
      - the HUD shows health, food and water in the bottom-right corner,
        as icons + numbers or rings (phone Settings; ui\hud.hpp).

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

    // The HUD: on while you're spawned, in the player's style (phone
    // Settings): "minimal" icons + numbers, "rings", or "off".
    private _style = (["get"] call ALife_fnc_phoneSettings) param [3, "minimal"];
    private _hud = uiNamespace getVariable ["ALife_Hud", displayNull];
    if (_showHud && { _active } && { _style != "off" }) then {
        if (isNull _hud) then {
            "ALife_hud" cutRsc ["ALife_Hud", "PLAIN", 0, false];
            _hud = uiNamespace getVariable ["ALife_Hud", displayNull];
        };
        if (!isNull _hud) then {
            private _rings = _style == "rings";
            {
                _x params ["_row", "_value", "_colour", "_low"];
                _value = (round _value max 0) min 100;
                // Numbers go amber when low and red when very low.
                private _alert = [1, 1, 1, 1];
                if (_value <= _low) then { _alert = [0.984, 0.749, 0.141, 1]; };
                if (_value <= _low / 2) then { _alert = [0.97, 0.44, 0.41, 1]; };
                private _base = 9300 + _row * 10;
                { (_hud displayCtrl (_base + _x)) ctrlShow !_rings; } forEach [1, 2, 3];
                { (_hud displayCtrl (_base + _x)) ctrlShow _rings; } forEach [4, 5, 6, 7];
                if (_rings) then {
                    private _arc = _hud displayCtrl (_base + 5);
                    _arc ctrlSetText format ["ui\hud\ring_%1_ca.paa", [str round (_value / 5), "0" + str round (_value / 5)] select (round (_value / 5) < 10)];
                    _arc ctrlSetTextColor ([_colour, _alert] select (_value <= _low));
                    (_hud displayCtrl (_base + 6)) ctrlSetTextColor ([[1, 1, 1, 1], _alert] select (_value <= _low));
                } else {
                    (_hud displayCtrl (_base + 1)) ctrlSetTextColor _colour;
                    private _text = _hud displayCtrl (_base + 3);
                    _text ctrlSetText str _value;
                    _text ctrlSetTextColor _alert;
                };
            } forEach [
                [1, (1 - damage player) * 100, [0.29, 0.81, 0.59, 1], 30],
                [2, ALife_food, [0.984, 0.749, 0.141, 1], _warnAt],
                [3, ALife_water, [0.34, 0.62, 1, 1], _warnAt]
            ];
        };
    } else {
        if (!isNull _hud) then { "ALife_hud" cutText ["", "PLAIN"]; };
    };

    uiSleep 0.5;
};
