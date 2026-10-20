/*
    ALife_fnc_emergencyLights (client, scheduled; from ALife_fnc_emergency)

    The flashing light bar of one police / EMS vehicle, on this machine: two
    coloured light points on the roof, flashing in turn, until the lights
    go off, the vehicle is destroyed or you're far away. Colours per faction
    (CfgALife >> Emergency); where they sit: the vehicle's own `lights[]` in
    CfgVehicles, else the middle of its roof.

    Arguments:
        _this: OBJECT - the vehicle
*/

private _v = _this;
private _cfg = missionConfigFile >> "CfgALife" >> "Emergency";
private _type = (missionNamespace getVariable ["ALife_emergencyTypes", createHashMap]) getOrDefault [toLower typeOf _v, "cop"];
private _colours = getArray (_cfg >> _type >> "colours");
private _flash = getNumber (_cfg >> "flash") max 0.1;
private _range = getNumber (_cfg >> "lightRange") + 100;

// Where: CfgVehicles >> <class> >> lights[] = {{x, y, z}, {x, y, z}}, else
// either side of the middle of the roof.
private _spots = getArray (missionConfigFile >> "CfgVehicles" >> typeOf _v >> "lights");
if (count _spots < 2) then {
    (boundingBoxReal _v) params ["_min", "_max"];
    private _half = (((_max select 0) - (_min select 0)) / 4) min 0.45;
    private _top = (_max select 2) * 0.88;
    _spots = [[-_half, 0, _top], [_half, 0, _top]];
};

private _lights = [];
{
    private _l = "#lightpoint" createVehicleLocal getPosATL _v;
    _l setLightColor (_colours param [_forEachIndex, [1, 0, 0]]);
    _l setLightAmbient (_colours param [_forEachIndex, [1, 0, 0]]);
    _l setLightAttenuation [0.2, 0, 900, 120];
    _l setLightUseFlare true;
    _l setLightFlareSize 0.4;
    _l setLightFlareMaxDistance 200;
    _l setLightDayLight true;
    _l setLightIntensity 0;
    _l lightAttachObject [_v, _x];
    _lights pushBack _l;
} forEach (_spots select [0, 2]);

private _left = true;
while { !isNull _v && { alive _v } && { _v getVariable ["alife_lights", false] } && { player distance _v < _range } } do {
    (_lights select 0) setLightIntensity ([0, 12] select _left);
    (_lights select 1) setLightIntensity ([12, 0] select _left);
    _left = !_left;
    uiSleep _flash;
};

{ deleteVehicle _x; } forEach _lights;
if (!isNull _v) then {
    _v setVariable ["alife_lightsShown", false];
    // Destroyed with the lights on: off for everyone.
    if (!alive _v && { _v getVariable ["alife_lights", false] } && { local _v }) then { _v setVariable ["alife_lights", false, true]; };
};
