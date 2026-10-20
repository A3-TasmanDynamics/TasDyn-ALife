/*
    ALife_fnc_vehicleApply (client, called by the server)

    Applies a vehicle action where the vehicle is local (whoever it belongs
    to on the network); ALife_fnc_vehicleAction has already checked it.

        ["push", vehicle, velocity]
        ["tyres", vehicle]         - spike strip: pop its tyres
*/

params [["_mode", "", [""]], ["_vehicle", objNull, [objNull]], ["_value", [], [[]]]];

if (isNull _vehicle || { !local _vehicle }) exitWith {};

switch (_mode) do {
    case "tyres": { [_vehicle] call ALife_fnc_vehicleTyres; };
    case "push": {
        if (_value isEqualTypeArray [0, 0, 0] && { vectorMagnitude _value <= 6 }) then { _vehicle setVelocity _value; };
    };
};
