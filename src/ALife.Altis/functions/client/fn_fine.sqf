/*
    ALife_fnc_fine (client, called by the server)

    A police officer fined you (ALife_fnc_personAction "fine"): a radial
    menu to pay it in cash or from the bank, or refuse. It runs out after
    CfgALife >> fineTimeout seconds; the server has the final say.

        ["offer", officer name, amount, seconds]
*/

params [["_mode", "", [""]], ["_by", "", [""]], ["_amount", 0, [0]], ["_timeout", 60, [0]]];

if (_mode != "offer" || { !hasInterface } || { _amount <= 0 }) exitWith {};
private _money = "$" + ([_amount] call BIS_fnc_numberText);
[format ["%1 fined you %2. Pay or refuse within %3 seconds.", _by, _money, round _timeout], "warning", "Fine"] call ALife_fnc_toast;
["open", format ["Fine %1", _money], [
    ["Pay (cash)", { [player, "finePay", objNull, "cash"] remoteExecCall ["ALife_fnc_personAction", 2]; }, []],
    ["Pay (bank)", { [player, "finePay", objNull, "bank"] remoteExecCall ["ALife_fnc_personAction", 2]; }, []],
    ["Refuse", { [player, "fineRefuse", objNull, ""] remoteExecCall ["ALife_fnc_personAction", 2]; }, []]
]] call ALife_fnc_radial;
