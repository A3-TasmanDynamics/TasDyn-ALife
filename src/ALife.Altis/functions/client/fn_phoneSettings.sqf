/*
    ALife_fnc_phoneSettings (client)

    The phone's Settings: view distance, grass and wallpaper, saved in your
    Arma profile so they stick between sessions.

    Modes:
        ["apply"]              - apply the saved settings (on every spawn)
        ["viewDistance", m]    - set and save the view distance
        ["toggleGrass"]        - grass on/off
        ["wallpaper", key]     - set and save the wallpaper (CfgPhone >> Wallpapers)
        ["wallpaperTexture"]   - returns the saved wallpaper's texture
        ["get"]                - returns [viewDistance, grassOn, wallpaperKey]
*/

params ["_mode", ["_value", 0]];

private _viewDistance = profileNamespace getVariable ["ALife_viewDistance", 2500];
private _grass = profileNamespace getVariable ["ALife_grass", true];
private _wallpapers = missionConfigFile >> "CfgPhone" >> "Wallpapers";
private _wallpaper = profileNamespace getVariable ["ALife_wallpaper", getText (missionConfigFile >> "CfgPhone" >> "defaultWallpaper")];
if (!isClass (_wallpapers >> _wallpaper)) then { _wallpaper = getText (missionConfigFile >> "CfgPhone" >> "defaultWallpaper"); };

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
    case "wallpaper": {
        if (_value isEqualType "" && { isClass (_wallpapers >> _value) }) then {
            profileNamespace setVariable ["ALife_wallpaper", _value];
            saveProfileNamespace;
        };
    };
    case "wallpaperTexture": { getText (_wallpapers >> _wallpaper >> "texture") };
    case "get": { [_viewDistance, _grass, _wallpaper] };
};
