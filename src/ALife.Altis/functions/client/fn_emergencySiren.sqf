/*
    ALife_fnc_emergencySiren (client, scheduled; from ALife_fnc_emergency)

    One police / EMS vehicle's siren, on this machine: the game's own
    police / ambulance siren (CfgSounds, description.ext) on a loop from the
    vehicle, while it's on, someone's in it and you're in earshot -- the
    cabin version while you're inside it.

    Arguments:
        _this: OBJECT - the vehicle
*/

private _v = _this;
private _cfg = missionConfigFile >> "CfgALife" >> "Emergency";
private _type = (missionNamespace getVariable ["ALife_emergencyTypes", createHashMap]) getOrDefault [toLower typeOf _v, "cop"];
private _outside = getText (_cfg >> _type >> "siren");
private _inside = getText (_cfg >> _type >> "sirenInside");
private _length = getNumber (_cfg >> _type >> "sirenLength") max 0.5;
private _range = getNumber (_cfg >> "sirenRange");
private _on = { !isNull _v && { alive _v } && { _v getVariable ["alife_siren", false] } && { player distance _v < _range + 50 } };

while { call _on } do {
    if ((crew _v) isEqualTo []) then { uiSleep 1; } else {
        private _in = vehicle player == _v;
        private _source = _v say3D [[_outside, _inside] select (_in && { _inside != "" }), _range, 1];
        // The sound's own source where the game gives one back (stopped
        // straight away when the siren goes off or you get in / out); else
        // its length.
        if (!isNil "_source" && { _source isEqualType objNull } && { !isNull _source }) then {
            waitUntil { uiSleep 0.1; isNull _source || { !(call _on) } || { (vehicle player == _v) != _in } };
            if (!isNull _source) then { deleteVehicle _source; };
        } else {
            uiSleep _length;
        };
    };
};

if (!isNull _v) then { _v setVariable ["alife_sirenPlaying", false]; };
