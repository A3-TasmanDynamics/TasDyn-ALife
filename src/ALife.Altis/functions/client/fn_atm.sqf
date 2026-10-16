/*
    ALife_fnc_atm (client)

    The ATM / bank menu (ui\atm.hpp), opened with the Windows key at any ATM
    on the map. Balances for the faction you're playing; deposit and
    withdraw (ALife_fnc_atmAction, checked by the server -- you must be at an
    ATM) and send money to a player (ALife_fnc_bankTransfer). The server
    answers with fresh phone data, which refreshes this menu.

    Modes:
        ["open"] / ["onLoad", display] / ["refresh"]
        ["quick", amount]        - fill the amount box
        ["deposit", all]         - all = all your cash
        ["withdraw", all]        - all = everything in the bank
        ["send"]                 - send the amount to the selected player
*/

#define IDD_ATM      5400
#define IDC_TITLE    5401
#define IDC_ACCOUNT  5402
#define IDC_BALANCE  5403
#define IDC_AMOUNT   5410
#define IDC_PLAYERS  5430

params ["_mode", ["_arg", false]];

private _display = uiNamespace getVariable ["ALife_atmDisplay", displayNull];
private _phone = missionNamespace getVariable ["ALife_phone", createHashMap];
private _cash = _phone getOrDefault ["cash", 0];
private _bank = _phone getOrDefault ["bank", 0];
private _fnc_money = { "$" + ([_this] call BIS_fnc_numberText) };
private _fnc_amount = { floor parseNumber ctrlText (_display displayCtrl IDC_AMOUNT) };

switch (_mode) do {
    case "open": {
        if (!isNull (findDisplay IDD_ATM)) exitWith {};
        createDialog "ALife_Atm";
        [player] remoteExec ["ALife_fnc_requestPhoneData", 2];
    };

    case "onLoad": {
        uiNamespace setVariable ["ALife_atmDisplay", _arg];
        _display = _arg;
        ["refresh"] call ALife_fnc_atm;
    };

    case "refresh": {
        if (isNull _display) exitWith {};
        (_display displayCtrl IDC_TITLE) ctrlSetText "ALTIS BANK";
        private _faction = _phone getOrDefault ["faction", missionNamespace getVariable ["ALife_faction", "civ"]];
        (_display displayCtrl IDC_ACCOUNT) ctrlSetText format ["%1 account", switch (_faction) do {
            case "cop": { "Police" }; case "medic": { "EMS" }; default { "Civilian" };
        }];
        (_display displayCtrl IDC_BALANCE) ctrlSetStructuredText parseText format [
            "<t size='2' font='RobotoCondensedBold'>%1</t><t size='0.9' color='#c0c7d1'>  bank</t><br/><t size='1' color='#d6dbe2'>Cash on hand  </t><t size='1' font='RobotoCondensedBold'>%2</t>",
            _bank call _fnc_money, _cash call _fnc_money];

        // Everyone else who's spawned in.
        private _list = _display displayCtrl IDC_PLAYERS;
        private _keep = lnbCurSelRow _list;
        lnbClear _list;
        ALife_atmTargets = [allPlayers select { _x != player && { alive _x } && { !isObjectHidden _x } }, [], { name _x }] call BIS_fnc_sortBy;
        { _list lnbSetValue [[_list lnbAddRow [(name _x splitString "<>&") joinString ""], 0], _forEachIndex]; } forEach ALife_atmTargets;
        if (ALife_atmTargets isEqualTo []) then {
            _list lnbAddRow ["Nobody else is online"];
            _list lnbSetColor [[0, 0], [0.62, 0.66, 0.72, 1]];
        } else {
            _list lnbSetCurSelRow ((_keep max 0) min (count ALife_atmTargets - 1));
        };
    };

    case "quick": {
        if (isNull _display) exitWith {};
        (_display displayCtrl IDC_AMOUNT) ctrlSetText str _arg;
    };

    case "deposit";
    case "withdraw": {
        if (isNull _display) exitWith {};
        private _amount = if (_arg isEqualTo true) then { [_bank, _cash] select (_mode == "deposit") } else { call _fnc_amount };
        if (_amount < 1) exitWith {
            hint (if (_arg isEqualTo true) then { ["The bank account is empty.", "You have no cash on you."] select (_mode == "deposit") } else { "Enter an amount." });
        };
        [player, _mode, _amount] remoteExecCall ["ALife_fnc_atmAction", 2];
        (_display displayCtrl IDC_AMOUNT) ctrlSetText "";
    };

    case "send": {
        if (isNull _display) exitWith {};
        private _list = _display displayCtrl IDC_PLAYERS;
        private _row = lnbCurSelRow _list;
        private _targets = missionNamespace getVariable ["ALife_atmTargets", []];
        private _target = if (_row < 0 || { _targets isEqualTo [] }) then { objNull } else { _targets param [_list lnbValue [_row, 0], objNull] };
        private _amount = call _fnc_amount;
        if (isNull _target) exitWith { hint "Pick who to send it to."; };
        if (_amount < 1) exitWith { hint "Enter an amount."; };
        [player, _target, _amount] remoteExec ["ALife_fnc_bankTransfer", 2];
        (_display displayCtrl IDC_AMOUNT) ctrlSetText "";
    };
};
