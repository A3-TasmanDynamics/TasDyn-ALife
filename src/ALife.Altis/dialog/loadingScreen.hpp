// TasDyn-ALife — connection loading screen.
// Shown from the moment a client connects until the spawn menu is ready to
// open (core/functions/ui/fn_loadingScreen.sqf, mode-dispatched like
// spawnMenu.hpp). The status text and progress bar reflect the ACTUAL join
// sequence -- initPlayerLocal.sqf and fn_playerJoin.sqf push real
// milestones to it (player object ready, join request sent, DB record
// loading, spawn menu about to open) -- not a simulated/fake timer. Based
// on a mockup design; reuses only common_ui.hpp's already-verified base
// classes (ALife_RscBackground/RscText/RscStructuredText/RscProgress).

#include "common_ui.hpp"

#define ALIFE_IDD_LOADING_SCREEN 4600

#define ALIFE_IDC_LOADING_TIP      4610
#define ALIFE_IDC_LOADING_STATUS   4611
#define ALIFE_IDC_LOADING_PCT      4612
#define ALIFE_IDC_LOADING_BAR      4613

class ALife_LoadingScreen
{
    idd = ALIFE_IDD_LOADING_SCREEN;
    movingEnable = 0;
    enableSimulation = 0;
    onLoad = "['onLoad'] call ALife_fnc_loadingScreen";
    onUnload = "['onUnload'] call ALife_fnc_loadingScreen";

    class controlsBackground
    {
        class Background: ALife_RscBackground
        {
            idc = -1;
            x = 0; y = 0; w = 1; h = 1;
            colorBackground[] = { 0.059, 0.09, 0.165, 1 };
        };

        class TipPanel: ALife_RscBackground
        {
            idc = -1;
            x = 0.3; y = 0.46; w = 0.4; h = 0.14;
            colorBackground[] = { 0.09, 0.11, 0.15, 0.7 };
        };

        class StatusBar: ALife_RscBackground
        {
            idc = -1;
            x = 0; y = 0.86; w = 1; h = 0.14;
            colorBackground[] = { 0.04, 0.055, 0.09, 0.95 };
        };
    };

    class controls
    {
        class LogoTitle: ALife_RscStructuredText
        {
            idc = -1;
            text = "<t align='center' size='2.4'>TASDYN<t color='#1de9b6'>-ALIFE</t></t>";
            x = 0.3; y = 0.28; w = 0.4; h = 0.08;
            colorBackground[] = { 0, 0, 0, 0 };
        };

        class Subtitle: ALife_RscText
        {
            idc = -1;
            text = "CUSTOM POSTGRESQL-BACKED ALTIS LIFE";
            style = 2; // ST_CENTER
            x = 0.3; y = 0.37; w = 0.4; h = 0.03;
            sizeEx = 0.018;
            colorText[] = { 0.58, 0.639, 0.722, 1 };
            colorBackground[] = { 0, 0, 0, 0 };
        };

        class TipLabel: ALife_RscText
        {
            idc = -1;
            text = "SERVER TIP";
            style = 2; // ST_CENTER
            x = 0.3; y = 0.475; w = 0.4; h = 0.02;
            sizeEx = 0.016;
            colorText[] = { 0.114, 0.914, 0.714, 1 };
            colorBackground[] = { 0, 0, 0, 0 };
        };

        class TipText: ALife_RscStructuredText
        {
            idc = ALIFE_IDC_LOADING_TIP;
            x = 0.32; y = 0.5; w = 0.36; h = 0.08;
            text = "";
        };

        class StatusLabel: ALife_RscText
        {
            idc = -1;
            text = "SYSTEM STATUS";
            x = 0.05; y = 0.885; w = 0.4; h = 0.02;
            sizeEx = 0.016;
            colorText[] = { 0.114, 0.914, 0.714, 1 };
            colorBackground[] = { 0, 0, 0, 0 };
        };

        class StatusText: ALife_RscText
        {
            idc = ALIFE_IDC_LOADING_STATUS;
            text = "Initializing connection...";
            x = 0.05; y = 0.908; w = 0.6; h = 0.035;
            sizeEx = 0.028;
            colorBackground[] = { 0, 0, 0, 0 };
        };

        class ProgressLabel: ALife_RscText
        {
            idc = -1;
            text = "PROGRESS";
            style = 1; // ST_RIGHT
            x = 0.65; y = 0.885; w = 0.3; h = 0.02;
            sizeEx = 0.016;
            colorText[] = { 0.58, 0.639, 0.722, 1 };
            colorBackground[] = { 0, 0, 0, 0 };
        };

        class ProgressPct: ALife_RscText
        {
            idc = ALIFE_IDC_LOADING_PCT;
            text = "0%";
            style = 1; // ST_RIGHT
            x = 0.65; y = 0.903; w = 0.3; h = 0.04;
            sizeEx = 0.032;
            colorBackground[] = { 0, 0, 0, 0 };
        };

        class ProgressBar: ALife_RscProgress
        {
            idc = ALIFE_IDC_LOADING_BAR;
            x = 0.05; y = 0.955; w = 0.9; h = 0.014;
        };
    };
};
