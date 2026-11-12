/*
    ALife_fnc_evidenceClient (client; "progress", "crack", "locked",
    "cracked" and "alarm" come from the server -- ALife_fnc_evidence)

    The evidence vault on this side: the progress bar of a break-in stage
    (cutting the door, setting the charge), the keypad (ui\vaultCrack.hpp:
    type a code, each guess shows how many digits are right and in place,
    and how many are right but elsewhere), and the police alarm (a map
    marker and a notice).

    Modes:
        ["progress", [stage, seconds, text]]
        ["crack", [codeLength, guessesLeft, [[guess, exact, near], ...]]]
        ["key", digit] / ["clear"] / ["enter"]   - the keypad's buttons
        ["locked", seconds] / ["cracked"]
        ["alarm", [on, position, text]]          - police only
        ["lock", [vault, locked]]                - the vault's inventory locked / open here
        ["init"]                                 - (initPlayerLocal) the same, on joining
*/

#define IDD 6100
#define IDC_TITLE 6101
#define IDC_STATUS 6102
#define IDC_ENTRY 6103
#define IDC_HISTORY 6130

params [["_mode", "", [""]], ["_arg", []]];

private _display = findDisplay IDD;
private _showEntry = {
    if (isNull _display) exitWith {};
    private _len = missionNamespace getVariable ["ALife_vcLength", 4];
    private _entry = missionNamespace getVariable ["ALife_vcEntry", ""];
    private _shown = _entry;
    for "_i" from count _entry to _len - 1 do { _shown = _shown + "_"; };
    (_display displayCtrl IDC_ENTRY) ctrlSetStructuredText parseText format [
        "<t align='center' size='2.4' font='EtelkaMonospacePro' color='#4ade80'>%1</t>", (_shown splitString "") joinString " "];
};

switch (_mode) do {
    case "progress": {
        _arg params ["_stage", "_seconds", "_text"];
        if (!isNil "ALife_using") exitWith {};
        ALife_using = true;
        [_stage, _seconds, _text] spawn {
            params ["_stage", "_seconds", "_text"];
            private _start = getPosATL player;
            private _began = diag_tickTime;
            private _done = false;
            "ALife_progress" cutRsc ["ALife_Progress", "PLAIN", 0, false];
            player playMoveNow "AinvPknlMstpSnonWnonDnon_medic1";
            while { true } do {
                private _elapsed = diag_tickTime - _began;
                private _d = uiNamespace getVariable ["ALife_Progress", displayNull];
                if (!isNull _d) then {
                    (_d displayCtrl 9201) ctrlSetText _text;
                    (_d displayCtrl 9202) progressSetPosition (_elapsed / _seconds);
                };
                if (_elapsed >= _seconds) exitWith { _done = true; };
                if (!alive player || { player getVariable ["alife_downed", false] } || { player getVariable ["alife_restrained", false] } || { player distance _start > 1.5 }) exitWith {};
                if ((animationState player) find "ainvpknlmstpsnonwnondnon_medic" != 0 && { _elapsed > 2 }) then { player playMoveNow "AinvPknlMstpSnonWnonDnon_medic1"; };
                sleep 0.1;
            };
            "ALife_progress" cutText ["", "PLAIN"];
            if (alive player) then { player playMoveNow "AinvPknlMstpSnonWnonDnon_medicEnd"; };
            ALife_using = nil;
            if (_done) then { [player, "finish", _stage] remoteExecCall ["ALife_fnc_evidence", 2]; } else { ["Stopped.", "warning"] call ALife_fnc_toast; };
        };
    };

    case "crack": {
        _arg params [["_len", 4], ["_left", 0], ["_history", []]];
        ALife_vcLength = _len;
        ALife_vcWaiting = false;
        if (isNull _display) then {
            ALife_vcEntry = "";
            if (dialog) then { closeDialog 0; };
            createDialog "ALife_VaultCrack";
            _display = findDisplay IDD;
        };
        ALife_vcEntry = "";
        call _showEntry;
        (_display displayCtrl IDC_STATUS) ctrlSetStructuredText parseText format [
            "<t align='center' color='#8b98b3'>%1 digits  ·  </t><t align='center' color='%3'>%2 tries left</t>", _len, _left, ["#fbbf24", "#f87171"] select (_left <= 2)];
        private _list = _display displayCtrl IDC_HISTORY;
        lnbClear _list;
        {
            _x params ["_guess", "_exact", "_near"];
            private _row = _list lnbAddRow [(_guess splitString "") joinString " ", format ["%1 right place", _exact], format ["%1 wrong place", _near]];
            _list lnbSetColor [[_row, 1], [0.29, 0.81, 0.59, 1]];
            _list lnbSetColor [[_row, 2], [0.98, 0.75, 0.14, 1]];
        } forEach _history;
    };

    case "key": {
        if (isNull _display || { missionNamespace getVariable ["ALife_vcWaiting", false] }) exitWith {};
        if (count ALife_vcEntry < ALife_vcLength) then { ALife_vcEntry = ALife_vcEntry + str _arg; };
        call _showEntry;
    };

    case "clear": {
        ALife_vcEntry = "";
        call _showEntry;
    };

    case "enter": {
        if (isNull _display || { missionNamespace getVariable ["ALife_vcWaiting", false] }) exitWith {};
        if (count ALife_vcEntry != ALife_vcLength) exitWith { [format ["Enter %1 digits.", ALife_vcLength], "warning"] call ALife_fnc_toast; };
        ALife_vcWaiting = true;
        [player, "guess", ALife_vcEntry] remoteExecCall ["ALife_fnc_evidence", 2];
    };

    case "locked": {
        if (!isNull _display) then { _display closeDisplay 2; };
        [format ["The keypad locked you out. It resets with a new code in %1 s.", _arg], "warning"] call ALife_fnc_toast;
    };

    case "cracked": {
        if (!isNull _display) then { _display closeDisplay 1; };
        ["Cracked it! The vault is open: the loot is in the crate beside it (Windows for the cash and items).", "success"] call ALife_fnc_toast;
    };

    case "init": {
        // Joining: the vault's inventory as the server has it (ALife_evidenceOpen).
        private _name = getText (missionConfigFile >> "CfgALife" >> "Evidence" >> "vault");
        private _until = time + 60;
        waitUntil { sleep 1; !isNil "ALife_evidenceOpen" || { time > _until } };
        private _v = missionNamespace getVariable [_name, objNull];
        if (!isNull _v) then { _v lockInventory !(missionNamespace getVariable ["ALife_evidenceOpen", false]); };
    };

    case "lock": {
        _arg params [["_vault", objNull], ["_locked", true]];
        if (!isNull _vault) then { _vault lockInventory _locked; };
    };

    case "alarm": {
        _arg params [["_on", false], ["_pos", [0, 0, 0]], ["_text", ""]];
        if ((missionNamespace getVariable ["ALife_faction", "civ"]) != "cop") exitWith {};
        deleteMarkerLocal "alife_evidence_alarm";
        deleteMarkerLocal "alife_evidence_alarm_area";
        if (!_on) exitWith { ["The evidence vault is secure again.", "info"] call ALife_fnc_toast; };
        private _area = createMarkerLocal ["alife_evidence_alarm_area", _pos];
        _area setMarkerShapeLocal "ELLIPSE";
        _area setMarkerSizeLocal [120, 120];
        _area setMarkerColorLocal "ColorRed";
        _area setMarkerBrushLocal "FDiagonal";
        private _m = createMarkerLocal ["alife_evidence_alarm", _pos];
        _m setMarkerTypeLocal "mil_warning";
        _m setMarkerColorLocal "ColorRed";
        _m setMarkerTextLocal "Evidence vault alarm";
        [_text, "error", "Evidence vault", 15] call ALife_fnc_toast;
    };
};
