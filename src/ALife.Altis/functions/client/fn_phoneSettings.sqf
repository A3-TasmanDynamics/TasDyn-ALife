/*
    ALife_fnc_phoneSettings (client)

    The phone's Settings: view distance and grass, saved in your Arma
    profile so they stick between sessions.

    Modes:
        ["apply"]              - apply the saved settings (on every spawn)
        ["viewDistance", m]    - set and save the view distance
        ["toggleGrass"]        - grass on/off
        ["get"]                - returns [viewDistance, grassOn]
*/

params ["_mode", ["_value", 0]];

private _viewDistance = profileNamespace getVariable ["ALife_viewDistance", 2500];
private _grass = profileNamespace getVariable ["ALife_grass", true];

private _fnc_apply = {
    setViewDistance _viewDistance;
    setObjectViewDistance [_viewDistance min 3000, 100];
    setTerrainGrid ([50, 25] select _grass);
};

switch (_mode) do {
    case "apply": { call _fnc_apply; };
    case "viewDistance": {
        _viewDistance = (_value max 500) min 6000;
        profileNamespace setVariable ["ALife_viewDistance", _viewDistance];
        saveProfileNamespace;
        call _fnc_apply;
    };
    case "toggleGrass": {
        _grass = !_grass;
        profileNamespace setVariable ["ALife_grass", _grass];
        saveProfileNamespace;
        call _fnc_apply;
    };
    case "get": { [_viewDistance, _grass] };
};
