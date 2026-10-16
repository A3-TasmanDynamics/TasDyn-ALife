/*
    ALife_fnc_containerCreateBag (server)

    Puts a bag on the ground in front of a player for the items they're
    dropping -- or, for cash only, a stack of money (CfgALife >> cashClass). It's a simple object built from the model of
    CfgALife >> bagClass: it has no Arma inventory of its own, so it can only
    be opened through our inventory screen (Windows key). It disappears
    after CfgALife >> bagLifetime, or as soon as it's empty.

    Arguments:
        0: OBJECT - the player dropping things
        1: BOOL   - cash only (a money stack instead of a bag)

    Returns:
        STRING - the new container id
*/

params ["_unit", ["_cashOnly", false]];

if (isNil "ALife_containers") then { ALife_containers = createHashMap; ALife_containerCount = 0; };

// The backpack's model, as createSimpleObject wants it: no leading "\", ".p3d" on the end.
private _class = getText (missionConfigFile >> "CfgALife" >> (["bagClass", "cashClass"] select _cashOnly));
private _model = getText (configFile >> "CfgVehicles" >> _class >> "model");
if ((_model select [0, 1]) == "\") then { _model = _model select [1]; };
if ((toLower _model select [count _model - 4]) != ".p3d") then { _model = _model + ".p3d"; };

private _front = _unit modelToWorld [0, 0.8, 0];
_front set [2, (getPosATL _unit) select 2];
private _bag = createSimpleObject [_model, ATLToASL _front];
// Sit it on the ground: lift it by however far the model reaches below its centre.
private _bottom = ((boundingBoxReal _bag) select 0) select 2;
_bag setPosASL ((ATLToASL _front) vectorAdd [0, 0, -_bottom]);
_bag setDir (getDir _unit);

ALife_containerCount = ALife_containerCount + 1;
private _id = format ["bag_%1", ALife_containerCount];
ALife_containers set [_id, createHashMapFromArray [
    ["object", _bag],
    ["kind", "ground"],
    ["title", ["Dropped items", "Cash"] select _cashOnly],
    ["items", createHashMap],
    ["cash", 0],
    ["capacity", 1000],
    ["created", time]
]];
// Clients spot bags by this public list (the object itself carries no
// data -- simple objects aren't guaranteed to support variables).
if (isNil "ALife_groundBags") then { ALife_groundBags = []; };
ALife_groundBags pushBack _bag;
publicVariable "ALife_groundBags";
_id
