/*
    ALife_fnc_applySkins (client)

    Puts the player's group look on what they're wearing:

      - uniform: the texture CfgClothing >> UniformTextures gives for their
        faction (cop / medic) or, for a civilian in a gang, for their gang
        (a row with their tag wins over one for any gang);
      - backpack: invisible for cops and medics (Altis Life style -- they
        still carry it, it just isn't drawn).

    setObjectTextureGlobal, so everyone sees it. Cheap; run whenever the
    gear might have changed (spawn, revive, the clothing shop, inventory
    changes -- ALife_fnc_setupUnit hooks those).

    Arguments:
        0: BOOL - true after the gear was reloaded (setUnitLoadout, putting a
                  uniform back on): its texture is back to default even if
                  the uniform is the same, so apply it again. Default false.
*/

params [["_force", false, [false]]];

if (!hasInterface || { !alive player }) exitWith {};
if (_force) then { player setVariable ["alife_skin", ""]; };

private _faction = missionNamespace getVariable ["ALife_faction", "civ"];
private _uniform = uniform player;
private _cfg = missionConfigFile >> "UniformTextures";
private _texture = "";

if (_uniform != "") then {
    if (_faction in ["cop", "medic"]) then {
        private _rows = getArray (_cfg >> _faction >> "textures");
        private _i = _rows findIf { (_x select 0) == _uniform };
        if (_i >= 0) then { _texture = (_rows select _i) select 1; };
    } else {
        private _tag = player getVariable ["alife_gang", ""];
        if (_tag != "") then {
            private _rows = getArray (_cfg >> "gang" >> "textures") select { (_x select 1) == _uniform };
            private _own = _rows findIf { (_x select 0) == _tag };
            private _any = _rows findIf { (_x select 0) == "" };
            private _i = [_any, _own] select (_own >= 0);
            if (_i >= 0) then { _texture = (_rows select _i) select 2; };
        };
    };
};
// Only once per change: re-sending a global texture every call would spam the network.
if (_texture != "" && { (player getVariable ["alife_skin", ""]) != (_uniform + _texture) }) then {
    player setObjectTextureGlobal [0, _texture];
    player setVariable ["alife_skin", _uniform + _texture];
};
if (_texture == "") then { player setVariable ["alife_skin", ""]; };

private _pack = unitBackpack player;
if (_faction in ["cop", "medic"] && { !isNull _pack } && { !(_pack getVariable ["alife_hidden", false]) }) then {
    _pack setObjectTextureGlobal [0, ""];
    _pack setVariable ["alife_hidden", true];
};
