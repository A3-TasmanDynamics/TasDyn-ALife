// TasDyn-ALife — spawn menu dialog.
// Spawn point list (filtered by the player's actual engine side, resolved
// via ALife_fnc_sideToFaction -- see fn_spawnMenu.sqf) -> a full-screen map
// preview -> Spawn. Matches Tonic's AsYetUntitled/Framework: faction is
// chosen via Arma's own multiplayer role-selection screen (multiple
// playable slots per side in mission.sqm), not a custom in-dialog picker --
// this dialog only ever picks WHERE to spawn within the side you already
// are. See docs/TONIC_REFERENCE.md §3 for why the earlier custom
// side-button version was replaced.
//
// Layout (dark/teal "modern HUD" look, from a mockup the project owner
// commissioned separately): a full-safezone interactive map as the
// background, with floating semi-transparent panels on top -- a header +
// spawn list top-left, a bottom action bar. Deliberately reuses only the
// already-verified common_ui.hpp base classes (ALife_RscMap/RscText/
// RscListBox/RscButton/RscStructuredText), just repositioned and
// recolored -- no new base class means no new missing-property risk, the
// exact class of bug common_ui.hpp's own header comment documents.

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
        // Full-safezone map background. x/w use the *Abs safezone variants
        // (multi-monitor-correct); y/h use Tonic's own proven expression
        // for the same purpose (docs/TONIC_REFERENCE.md §3) -- both
        // verified against docs/arma/arma3.db before use, not guessed.
        class SpawnMap: ALife_RscMap
        {
            idc = ALIFE_IDC_SPAWN_MAP;
            x = "safeZoneXAbs";
            y = "safeZoneY + 1.5 * ( ( ((safezoneW / safezoneH) min 1.2) / 1.2) / 25)";
            w = "safeZoneWAbs";
            h = "safeZoneH - 1.5 * ( ( ((safezoneW / safezoneH) min 1.2) / 1.2) / 25)";
            colorBackground[] = { 0.03, 0.03, 0.04, 1 };
        };

        class BottomBar: ALife_RscBackground
        {
            idc = -1;
            x = 0; y = 0.91; w = 1; h = 0.09;
            colorBackground[] = { 0.043, 0.051, 0.055, 0.95 };
        };
    };

    class controls
    {
        class HeaderTitle: ALife_RscText
        {
            idc = -1;
            text = "SPAWN";
            x = 0.03; y = 0.05; w = 0.25; h = 0.045;
            sizeEx = 0.045;
            colorText[] = { 0.231, 0.886, 0.702, 1 };
            colorBackground[] = { 0, 0, 0, 0 };
        };

        class HeaderSubtitle: ALife_RscText
        {
            idc = -1;
            text = "SELECTOR";
            x = 0.03; y = 0.095; w = 0.25; h = 0.025;
            sizeEx = 0.018;
            colorText[] = { 0.545, 0.58, 0.62, 1 };
            colorBackground[] = { 0, 0, 0, 0 };
        };

        // Icon + title "card" rows, matching the mockup's location list
        // more closely than a plain single-line listbox -- see
        // common_ui.hpp's ALife_RscListNBox.
        class SpawnList: ALife_RscListNBox
        {
            idc = ALIFE_IDC_SPAWN_LIST;
            x = 0.03; y = 0.13; w = 0.26; h = 0.55;
            onLBSelChanged = "(['selectLocation'] + _this) call ALife_fnc_spawnMenu";
        };

        // Big "selected point" title, positioned like the mockup's floating
        // preview title just above the bottom bar (no location photo --
        // no such asset exists yet, see fn_spawnMenu.sqf's onLoad comment).
        class InfoText: ALife_RscStructuredText
        {
            idc = ALIFE_IDC_SPAWN_INFO;
            x = 0.03; y = 0.8; w = 0.4; h = 0.09;
            text = "";
        };

        class BtnCancel: ALife_RscButton
        {
            idc = ALIFE_IDC_SPAWN_BTN_CANCEL;
            text = "CANCEL";
            x = 0.03; y = 0.935; w = 0.16; h = 0.045;
            colorBackground[] = { 0.227, 0.247, 0.278, 1 };
            action = "closeDialog 0";
        };

        class BtnSpawn: ALife_RscButton
        {
            idc = ALIFE_IDC_SPAWN_BTN_SPAWN;
            text = "SPAWN";
            x = 0.81; y = 0.935; w = 0.16; h = 0.045;
            colorBackground[] = { 0.231, 0.886, 0.702, 1 };
            colorText[] = { 0.04, 0.047, 0.055, 1 };
            action = "['spawn'] call ALife_fnc_spawnMenu";
        };
    };
};
