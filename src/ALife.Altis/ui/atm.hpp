// ATM / bank menu (Windows key at any ATM on the map). Driven by
// ALife_fnc_atm; control IDs match the #defines there.
//
//   ALTIS BANK                                   Civilian account
//   $12,500           bank
//   Cash on hand  $350
//   AMOUNT ($)  [        ]
//   [$100] [$500] [$1,000] [$5,000]
//   [Deposit]            [Withdraw]
//   [Deposit all cash]   [Withdraw all]
//   SEND TO A PLAYER
//   | Andrew Thompson |
//   [Send]                                [Close]

class ALife_Atm
{
    idd = 5400;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_atm";

    class controlsBackground
    {
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.06, 0.07, 0.09, 0.96 };
            x = "safezoneX + safezoneW * 0.36";
            y = "safezoneY + safezoneH * 0.2";
            w = "safezoneW * 0.28";
            h = "safezoneH * 0.62";
        };
        class Header: ALife_RscBackground
        {
            colorBackground[] = { 0.13, 0.42, 0.27, 1 };
            x = "safezoneX + safezoneW * 0.36";
            y = "safezoneY + safezoneH * 0.2";
            w = "safezoneW * 0.28";
            h = "safezoneH * 0.045";
        };
        class BalanceCard: ALife_RscBackground
        {
            colorBackground[] = { 0.13, 0.67, 0.37, 0.14 };
            x = "safezoneX + safezoneW * 0.368";
            y = "safezoneY + safezoneH * 0.255";
            w = "safezoneW * 0.264";
            h = "safezoneH * 0.1";
        };
    };

    class controls
    {
        class Title: ALife_RscText
        {
            idc = 5401;
            text = "ALTIS BANK";
            colorText[] = { 1, 1, 1, 1 };
            font = "RobotoCondensedBold";
            sizeEx = 0.038;
            x = "safezoneX + safezoneW * 0.366";
            y = "safezoneY + safezoneH * 0.203";
            w = "safezoneW * 0.14";
            h = "safezoneH * 0.04";
        };
        class Account: ALife_RscText
        {
            idc = 5402;
            style = 1;
            sizeEx = 0.03;
            colorText[] = { 0.85, 0.95, 0.88, 1 };
            x = "safezoneX + safezoneW * 0.5";
            y = "safezoneY + safezoneH * 0.205";
            w = "safezoneW * 0.134";
            h = "safezoneH * 0.035";
        };
        class Balance: ALife_RscStructuredText
        {
            idc = 5403;
            size = 0.03;
            x = "safezoneX + safezoneW * 0.372";
            y = "safezoneY + safezoneH * 0.26";
            w = "safezoneW * 0.256";
            h = "safezoneH * 0.09";
        };

        class AmountLabel: ALife_RscText
        {
            text = "AMOUNT ($)";
            font = "RobotoCondensedBold";
            sizeEx = 0.026;
            colorText[] = { 0.62, 0.66, 0.72, 1 };
            x = "safezoneX + safezoneW * 0.368";
            y = "safezoneY + safezoneH * 0.365";
            w = "safezoneW * 0.1";
            h = "safezoneH * 0.03";
        };
        class Amount: ALife_RscEdit
        {
            idc = 5410;
            maxChars = 10;
            sizeEx = 0.036;
            colorBackground[] = { 1, 1, 1, 0.07 };
            x = "safezoneX + safezoneW * 0.368";
            y = "safezoneY + safezoneH * 0.395";
            w = "safezoneW * 0.264";
            h = "safezoneH * 0.042";
        };
        class Quick0: ALife_RscButton
        {
            text = "$100";
            sizeEx = 0.03;
            colorBackground[] = { 1, 1, 1, 0.08 };
            colorBackgroundActive[] = { 1, 1, 1, 0.18 };
            colorFocused[] = { 1, 1, 1, 0.12 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "['quick', 100] call ALife_fnc_atm";
            x = "safezoneX + safezoneW * 0.368";
            y = "safezoneY + safezoneH * 0.445";
            w = "safezoneW * 0.0625";
            h = "safezoneH * 0.035";
        };
        class Quick1: Quick0
        {
            text = "$500";
            action = "['quick', 500] call ALife_fnc_atm";
            x = "safezoneX + safezoneW * 0.4355";
        };
        class Quick2: Quick0
        {
            text = "$1,000";
            action = "['quick', 1000] call ALife_fnc_atm";
            x = "safezoneX + safezoneW * 0.503";
        };
        class Quick3: Quick0
        {
            text = "$5,000";
            action = "['quick', 5000] call ALife_fnc_atm";
            x = "safezoneX + safezoneW * 0.5695";
        };

        class Deposit: ALife_RscButton
        {
            text = "Deposit";
            font = "RobotoCondensedBold";
            sizeEx = 0.034;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 1, 0.72, 0.2, 1 };
            colorFocused[] = { 1, 0.72, 0.2, 1 };
            colorText[] = { 0.05, 0.05, 0.05, 1 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "['deposit', false] call ALife_fnc_atm";
            x = "safezoneX + safezoneW * 0.368";
            y = "safezoneY + safezoneH * 0.49";
            w = "safezoneW * 0.129";
            h = "safezoneH * 0.042";
        };
        class Withdraw: Deposit
        {
            text = "Withdraw";
            action = "['withdraw', false] call ALife_fnc_atm";
            x = "safezoneX + safezoneW * 0.503";
        };
        class DepositAll: ALife_RscButton
        {
            text = "Deposit all cash";
            sizeEx = 0.03;
            colorBackground[] = { 1, 1, 1, 0.08 };
            colorBackgroundActive[] = { 1, 1, 1, 0.18 };
            colorFocused[] = { 1, 1, 1, 0.12 };
            colorShadow[] = { 0, 0, 0, 0 };
            action = "['deposit', true] call ALife_fnc_atm";
            x = "safezoneX + safezoneW * 0.368";
            y = "safezoneY + safezoneH * 0.538";
            w = "safezoneW * 0.129";
            h = "safezoneH * 0.035";
        };
        class WithdrawAll: DepositAll
        {
            text = "Withdraw all";
            action = "['withdraw', true] call ALife_fnc_atm";
            x = "safezoneX + safezoneW * 0.503";
        };

        class SendLabel: ALife_RscText
        {
            text = "SEND TO A PLAYER (uses the amount above)";
            font = "RobotoCondensedBold";
            sizeEx = 0.026;
            colorText[] = { 0.62, 0.66, 0.72, 1 };
            x = "safezoneX + safezoneW * 0.368";
            y = "safezoneY + safezoneH * 0.585";
            w = "safezoneW * 0.264";
            h = "safezoneH * 0.03";
        };
        class Players: ALife_RscListNBox
        {
            idc = 5430;
            sizeEx = 0.03;
            rowHeight = 0.04;
            columns[] = { 0.03 };
            colorBackground[] = { 1, 1, 1, 0.04 };
            colorSelectBackground[] = { 0.96, 0.62, 0.04, 0.9 };
            colorSelectBackground2[] = { 0.96, 0.62, 0.04, 0.9 };
            colorSelect[] = { 0.05, 0.05, 0.05, 1 };
            colorSelect2[] = { 0.05, 0.05, 0.05, 1 };
            x = "safezoneX + safezoneW * 0.368";
            y = "safezoneY + safezoneH * 0.615";
            w = "safezoneW * 0.264";
            h = "safezoneH * 0.15";
        };
        class Send: Deposit
        {
            text = "Send";
            action = "['send'] call ALife_fnc_atm";
            x = "safezoneX + safezoneW * 0.368";
            y = "safezoneY + safezoneH * 0.772";
            w = "safezoneW * 0.129";
            h = "safezoneH * 0.038";
        };
        class Close: DepositAll
        {
            text = "Close";
            action = "closeDialog 0";
            x = "safezoneX + safezoneW * 0.503";
            y = "safezoneY + safezoneH * 0.772";
            h = "safezoneH * 0.038";
        };
    };
};
