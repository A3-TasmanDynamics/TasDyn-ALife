// Clothing shop (ALife_fnc_clothingShop): a panel on the left, the rest of
// the screen is a camera on the player trying things on.
//
//   CLOTHING STORE                     Bank $750 | Cash $0
//   [Uniforms v]
//   [icon] Polo shirt (blue)      $250
//   ...
//   Firearms Licence needed. / price / requirement
//   [<]  [>]                      (turn the camera)
//   [Pay cash] [Pay bank]                      [Close]

#define ALIFE_SHOP_X "safezoneX + safezoneW * 0.02"
#define ALIFE_SHOP_W "safezoneW * 0.26"

class ALife_ClothingShop
{
    idd = 5500;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_clothingShop";
    onUnload = "['unload'] call ALife_fnc_clothingShop";

    class controlsBackground
    {
        class Panel: ALife_RscBackground
        {
            colorBackground[] = { 0.059, 0.09, 0.165, 0.94 };
            x = ALIFE_SHOP_X;
            y = "safezoneY + safezoneH * 0.1";
            w = ALIFE_SHOP_W;
            h = "safezoneH * 0.8";
        };
        class Accent: ALife_RscBackground
        {
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            x = ALIFE_SHOP_X;
            y = "safezoneY + safezoneH * 0.1";
            w = ALIFE_SHOP_W;
            h = "safezoneH * 0.004";
        };
    };

    class controls
    {
        class Title: ALife_RscText
        {
            idc = 5501;
            font = "RobotoCondensedBold";
            sizeEx = 0.042;
            x = "safezoneX + safezoneW * 0.03";
            y = "safezoneY + safezoneH * 0.115";
            w = "safezoneW * 0.24";
            h = "safezoneH * 0.04";
        };
        class Balance: ALife_RscStructuredText
        {
            idc = 5502;
            size = 0.03;
            x = "safezoneX + safezoneW * 0.03";
            y = "safezoneY + safezoneH * 0.155";
            w = "safezoneW * 0.24";
            h = "safezoneH * 0.03";
        };
        class Category: ALife_RscCombo
        {
            idc = 5505;
            sizeEx = 0.032;
            onLBSelChanged = "['category'] call ALife_fnc_clothingShop";
            x = "safezoneX + safezoneW * 0.03";
            y = "safezoneY + safezoneH * 0.195";
            w = "safezoneW * 0.24";
            h = "safezoneH * 0.035";
        };
        class List: ALife_RscListNBox
        {
            idc = 5510;
            sizeEx = 0.03;
            rowHeight = 0.05;
            columns[] = { 0, 0.72 };
            onLBSelChanged = "['select'] call ALife_fnc_clothingShop";
            x = "safezoneX + safezoneW * 0.03";
            y = "safezoneY + safezoneH * 0.24";
            w = "safezoneW * 0.24";
            h = "safezoneH * 0.46";
        };
        class Description: ALife_RscStructuredText
        {
            idc = 5503;
            size = 0.029;
            x = "safezoneX + safezoneW * 0.03";
            y = "safezoneY + safezoneH * 0.71";
            w = "safezoneW * 0.24";
            h = "safezoneH * 0.06";
        };
        class TurnLeft: ALife_RscButton
        {
            text = "<";
            font = "RobotoCondensedBold";
            sizeEx = 0.04;
            colorBackground[] = { 1, 1, 1, 0.12 };
            colorBackgroundActive[] = { 1, 1, 1, 0.22 };
            colorFocused[] = { 1, 1, 1, 0.16 };
            tooltip = "Turn the camera";
            action = "['rotate', -45] call ALife_fnc_clothingShop";
            x = "safezoneX + safezoneW * 0.03";
            y = "safezoneY + safezoneH * 0.78";
            w = "safezoneW * 0.03";
            h = "safezoneH * 0.04";
        };
        class TurnRight: TurnLeft
        {
            text = ">";
            action = "['rotate', 45] call ALife_fnc_clothingShop";
            x = "safezoneX + safezoneW * 0.065";
        };
        class PayCash: ALife_RscButton
        {
            idc = 5530;
            text = "Pay cash";
            sizeEx = 0.032;
            colorBackground[] = { 1, 1, 1, 0.12 };
            colorBackgroundActive[] = { 1, 1, 1, 0.22 };
            colorFocused[] = { 1, 1, 1, 0.16 };
            action = "['buy', 'cash'] call ALife_fnc_clothingShop";
            x = "safezoneX + safezoneW * 0.03";
            y = "safezoneY + safezoneH * 0.835";
            w = "safezoneW * 0.075";
            h = "safezoneH * 0.045";
        };
        class PayBank: ALife_RscButton
        {
            idc = 5531;
            text = "Pay bank";
            font = "RobotoCondensedBold";
            sizeEx = 0.032;
            colorBackground[] = { 0.96, 0.62, 0.04, 1 };
            colorBackgroundActive[] = { 1, 0.72, 0.2, 1 };
            colorFocused[] = { 1, 0.72, 0.2, 1 };
            colorText[] = { 0.118, 0.071, 0, 1 };
            action = "['buy', 'bank'] call ALife_fnc_clothingShop";
            x = "safezoneX + safezoneW * 0.11";
            y = "safezoneY + safezoneH * 0.835";
            w = "safezoneW * 0.075";
            h = "safezoneH * 0.045";
        };
        class Close: ALife_RscButton
        {
            text = "Close";
            sizeEx = 0.032;
            colorBackground[] = { 0, 0, 0, 0 };
            colorBackgroundActive[] = { 1, 1, 1, 0.1 };
            colorFocused[] = { 0, 0, 0, 0 };
            colorText[] = { 0.75, 0.78, 0.82, 1 };
            action = "closeDialog 0";
            x = "safezoneX + safezoneW * 0.21";
            y = "safezoneY + safezoneH * 0.835";
            w = "safezoneW * 0.06";
            h = "safezoneH * 0.045";
        };
    };
};

// Vehicle shop (ALife_fnc_vehicleShop): the same panel; the camera circles
// a preview of the selected vehicle in the chosen colour.
class ALife_VehicleShop: ALife_ClothingShop
{
    idd = 5600;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_vehicleShop";
    onUnload = "['unload'] call ALife_fnc_vehicleShop";

    class controls: controls
    {
        class Title: Title { idc = 5601; };
        class Balance: Balance { idc = 5602; };
        class Category: Category
        {
            idc = 5605;
            onLBSelChanged = "['colour'] call ALife_fnc_vehicleShop";
            y = "safezoneY + safezoneH * 0.565";
        };
        class List: List
        {
            idc = 5610;
            onLBSelChanged = "['select'] call ALife_fnc_vehicleShop";
            y = "safezoneY + safezoneH * 0.195";
            h = "safezoneH * 0.36";
        };
        class Description: Description
        {
            idc = 5603;
            y = "safezoneY + safezoneH * 0.61";
            h = "safezoneH * 0.16";
        };
        class TurnLeft: TurnLeft { action = "['rotate', -45] call ALife_fnc_vehicleShop"; };
        class TurnRight: TurnRight { action = "['rotate', 45] call ALife_fnc_vehicleShop"; };
        class PayCash: PayCash { idc = 5630; action = "['buy', 'cash'] call ALife_fnc_vehicleShop"; };
        class PayBank: PayBank { idc = 5631; action = "['buy', 'bank'] call ALife_fnc_vehicleShop"; };
        class Close: Close {};
    };
};
