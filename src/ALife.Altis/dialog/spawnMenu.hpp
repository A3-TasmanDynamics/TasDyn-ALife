// TasDyn-ALife — spawn menu dialog.
// Spawn point list (filtered by the player's actual engine side, resolved
// via ALife_fnc_sideToFaction -- see fn_spawnMenu.sqf) -> a map preview of
// the selected point -> Spawn. Matches Tonic's AsYetUntitled/Framework:
// faction is chosen via Arma's own multiplayer role-selection screen
// (multiple playable slots per side in mission.sqm), not a custom in-dialog
// picker -- this dialog only ever picks WHERE to spawn within the side
// you already are. See docs/TONIC_REFERENCE.md §3 for why the earlier
// custom side-button version was replaced.

#include "common_ui.hpp"

#define ALIFE_IDD_SPAWN_MENU 4700

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
            text = "Select Spawn Point";
            x = 0.18; y = 0.14; w = 0.64; h = 0.05;
            colorBackground[] = { 0, 0, 0, 0 };
        };

        class SpawnList: ALife_RscListBox
        {
            idc = ALIFE_IDC_SPAWN_LIST;
            x = 0.19; y = 0.2; w = 0.29; h = 0.45;
            onLBSelChanged = "(['selectLocation'] + _this) call ALife_fnc_spawnMenu";
        };

        class SpawnMap: ALife_RscMap
        {
            idc = ALIFE_IDC_SPAWN_MAP;
            x = 0.5; y = 0.2; w = 0.3; h = 0.45;
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
