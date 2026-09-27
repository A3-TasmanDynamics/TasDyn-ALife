// TasDyn-ALife — spawn menu dialog.
// Side-select buttons (civ/cop/medic) -> spawn point list, filtered per side
// via ALife_fnc_getSpawnPoints (config/spawn_config.hpp) -> a map preview of
// the selected point -> Spawn. Reviewed a real framework's dialog for the
// general "side buttons + list + map preview" shape and its self-contained
// ALife_Rsc*-style base classes (common_ui.hpp) -- not copied wholesale,
// rebuilt against this mission's own data contract and spawn functions.

#include "common_ui.hpp"

#define ALIFE_IDD_SPAWN_MENU 4700

#define ALIFE_IDC_SPAWN_SIDE_CIV   4710
#define ALIFE_IDC_SPAWN_SIDE_COP   4711
#define ALIFE_IDC_SPAWN_SIDE_MEDIC 4712
#define ALIFE_IDC_SPAWN_LIST       4720
#define ALIFE_IDC_SPAWN_MAP        4730
#define ALIFE_IDC_SPAWN_INFO       4740
#define ALIFE_IDC_SPAWN_BTN_SPAWN  4750
#define ALIFE_IDC_SPAWN_BTN_CANCEL 4751

class ALife_SpawnMenu
{
    idd = ALIFE_IDD_SPAWN_MENU;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad'] call ALife_fnc_spawnMenu";
    onUnload = "['onUnload'] call ALife_fnc_spawnMenu";

    class controlsBackground
    {
        class Background: ALife_RscBackground
        {
            idc = -1;
            x = 0.18; y = 0.14; w = 0.64; h = 0.66;
            colorBackground[] = { 0, 0, 0, 0.85 };
        };

        class HeaderBar: ALife_RscBackground
        {
            idc = -1;
            x = 0.18; y = 0.14; w = 0.64; h = 0.05;
            colorBackground[] = { 0.1, 0.1, 0.1, 1 };
        };
    };

    class controls
    {
        class Title: ALife_RscTitle
        {
            idc = -1;
            text = "Select Faction & Spawn Point";
            x = 0.18; y = 0.14; w = 0.64; h = 0.05;
            colorBackground[] = { 0, 0, 0, 0 };
        };

        class SideCivilian: ALife_RscButton
        {
            idc = ALIFE_IDC_SPAWN_SIDE_CIV;
            text = "Civilian";
            x = 0.19; y = 0.2; w = 0.14; h = 0.04;
            colorBackground[] = { 0.2, 0.5, 0.2, 1 };
            action = "['selectSide', 'civ'] call ALife_fnc_spawnMenu";
        };

        class SideCop: ALife_RscButton
        {
            idc = ALIFE_IDC_SPAWN_SIDE_COP;
            text = "Police";
            x = 0.34; y = 0.2; w = 0.14; h = 0.04;
            colorBackground[] = { 0.2, 0.2, 0.6, 1 };
            action = "['selectSide', 'cop'] call ALife_fnc_spawnMenu";
        };

        class SideMedic: ALife_RscButton
        {
            idc = ALIFE_IDC_SPAWN_SIDE_MEDIC;
            text = "Medic";
            x = 0.49; y = 0.2; w = 0.14; h = 0.04;
            colorBackground[] = { 0.6, 0.2, 0.2, 1 };
            action = "['selectSide', 'medic'] call ALife_fnc_spawnMenu";
        };

        class SpawnList: ALife_RscListBox
        {
            idc = ALIFE_IDC_SPAWN_LIST;
            x = 0.19; y = 0.25; w = 0.29; h = 0.4;
            onLBSelChanged = "(['selectLocation'] + _this) call ALife_fnc_spawnMenu";
        };

        class SpawnMap: ALife_RscMap
        {
            idc = ALIFE_IDC_SPAWN_MAP;
            x = 0.5; y = 0.25; w = 0.3; h = 0.4;
        };

        class InfoText: ALife_RscStructuredText
        {
            idc = ALIFE_IDC_SPAWN_INFO;
            x = 0.19; y = 0.66; w = 0.61; h = 0.06;
            text = "";
        };

        class BtnSpawn: ALife_RscButton
        {
            idc = ALIFE_IDC_SPAWN_BTN_SPAWN;
            text = "Spawn";
            x = 0.6; y = 0.73; w = 0.1; h = 0.045;
            colorBackground[] = { 0.2, 0.55, 0.2, 1 };
            action = "['spawn'] call ALife_fnc_spawnMenu";
        };

        class BtnCancel: ALife_RscButton
        {
            idc = ALIFE_IDC_SPAWN_BTN_CANCEL;
            text = "Cancel";
            x = 0.71; y = 0.73; w = 0.1; h = 0.045;
            colorBackground[] = { 0.5, 0.2, 0.2, 1 };
            action = "closeDialog 0";
        };
    };
};
