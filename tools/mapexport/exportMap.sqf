/*
    ALife map export: reads Altis's terrain in a LOCAL editor preview and
    writes it to your client's RPT log, for tools/mapexport to turn into the
    admin panel's Live Map tiles. Paste the whole file into the Debug
    Console (Esc during an Eden preview) and press LOCAL EXEC. Never run it
    on the live server.

    Writes lines starting "ALMAP|":
        ALMAP|meta|<world>|<worldSize>|<heightStep>|<treeStep>
        ALMAP|h|<row>|<h0>,<h1>,...         terrain height ASL (m), one row (south to north) per line
        ALMAP|t|<row>|<n0>,<n1>,...         trees + bushes per tree cell, one row per line
        ALMAP|r|<type>,<width>,<x1>,<y1>,<x2>,<y2>;...   road pieces
        ALMAP|b|<x>,<y>,<dir>,<w>,<l>;...    buildings (centre, heading, footprint)
        ALMAP|l|<type>|<name>|<x>|<y>         named places
        ALMAP|done|<seconds>
*/
[] spawn {
    private _t0 = time;
    private _size = worldSize;
    private _hStep = 40;   // height grid (m)
    private _tStep = 100;  // tree density grid (m)
    private _cell = 1000;  // roads / buildings are read 1 km square at a time
    private _put = { diag_log text ("ALMAP|" + _this); };
    format ["meta|%1|%2|%3|%4", worldName, _size, _hStep, _tStep] call _put;

    // Heights, south to north.
    private _n = floor (_size / _hStep);
    for "_r" from 0 to _n do {
        private _row = [];
        for "_c" from 0 to _n do {
            _row pushBack ((getTerrainHeightASL [_c * _hStep, _r * _hStep]) toFixed 0);
        };
        format ["h|%1|%2", _r, _row joinString ","] call _put;
        if (_r mod 50 == 0) then { hintSilent format ["Map export: heights %1 pct", round (_r / _n * 100)]; };
    };

    // Trees and bushes per cell (forest shading).
    private _tn = floor (_size / _tStep) - 1;
    for "_r" from 0 to _tn do {
        private _row = [];
        for "_c" from 0 to _tn do {
            private _centre = [(_c + 0.5) * _tStep, (_r + 0.5) * _tStep];
            _row pushBack str (count (nearestTerrainObjects [_centre, ["TREE", "SMALL TREE", "BUSH"], _tStep * 0.71, false, true]));
        };
        format ["t|%1|%2", _r, _row joinString ","] call _put;
        if (_r mod 20 == 0) then { hintSilent format ["Map export: trees %1 pct", round (_r / _tn * 100)]; };
    };

    // Roads and buildings, 1 km at a time (each kept only in the cell its
    // centre is in, so nothing is written twice).
    private _cells = ceil (_size / _cell) - 1;
    private _buildingTypes = ["BUILDING", "HOUSE", "CHURCH", "CHAPEL", "FUELSTATION", "HOSPITAL", "LIGHTHOUSE", "FORTRESS", "BUNKER", "TOWER", "RUIN", "TRANSMITTER", "WATERTOWER", "VIEW-TOWER", "POWERSOLAR", "POWERWAVE", "POWERWIND", "SHIPWRECK", "QUAY", "FOUNTAIN"];
    for "_gy" from 0 to _cells do {
        for "_gx" from 0 to _cells do {
            private _x0 = _gx * _cell; private _y0 = _gy * _cell;
            private _centre = [_x0 + _cell / 2, _y0 + _cell / 2];
            private _inCell = { params ["_p"]; (_p select 0) >= _x0 && { (_p select 0) < _x0 + _cell } && { (_p select 1) >= _y0 } && { (_p select 1) < _y0 + _cell } };

            private _roads = [];
            {
                private _info = getRoadInfo _x;
                _info params ["_type", "_width", "_ped", "", "", "", "_beg", "_end"];
                if (_type != "HIDE" && { [getPosWorld _x] call _inCell }) then {
                    _roads pushBack format ["%1,%2,%3,%4,%5,%6", _type, _width toFixed 1,
                        (_beg select 0) toFixed 1, (_beg select 1) toFixed 1, (_end select 0) toFixed 1, (_end select 1) toFixed 1];
                };
                if (count _roads >= 60) then { ("r|" + (_roads joinString ";")) call _put; _roads = []; };
            } forEach (_centre nearRoads (_cell * 0.71));
            if (count _roads > 0) then { ("r|" + (_roads joinString ";")) call _put; };

            private _bs = [];
            {
                private _p = getPosWorld _x;
                if ([_p] call _inCell) then {
                    (boundingBoxReal _x) params ["_min", "_max"];
                    private _w = (_max select 0) - (_min select 0);
                    private _l = (_max select 1) - (_min select 1);
                    if (_w > 1.5 && { _l > 1.5 } && { _w < 120 } && { _l < 120 }) then {
                        _bs pushBack format ["%1,%2,%3,%4,%5", (_p select 0) toFixed 1, (_p select 1) toFixed 1, (getDir _x) toFixed 0, _w toFixed 1, _l toFixed 1];
                    };
                };
                if (count _bs >= 80) then { ("b|" + (_bs joinString ";")) call _put; _bs = []; };
            } forEach (nearestTerrainObjects [_centre, _buildingTypes, _cell * 0.71, false, true]);
            if (count _bs > 0) then { ("b|" + (_bs joinString ";")) call _put; };
        };
        hintSilent format ["Map export: roads and buildings %1 pct", round (_gy / _cells * 100)];
    };

    // Named places.
    {
        private _p = locationPosition _x;
        format ["l|%1|%2|%3|%4", type _x, text _x, (_p select 0) toFixed 0, (_p select 1) toFixed 0] call _put;
    } forEach (nearestLocations [[_size / 2, _size / 2], ["NameCityCapital", "NameCity", "NameVillage", "NameLocal", "NameMarine", "Airport", "Hill", "Mount"], _size]);

    private _secs = round (time - _t0);
    format ["done|%1", _secs] call _put;
    hint format ["Map export finished in %1 s. Send the newest Arma3_x64 .rpt log to staff.", _secs];
};
