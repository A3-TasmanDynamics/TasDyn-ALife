// TasDyn-ALife — spawn menu dialog.
// Faction list -> spawn point list (populated from config/spawn_config.hpp
// via ALife_fnc_getSpawnPoints) -> Spawn button. Deliberately simpler than
// a full map-preview dialog (reviewed a real one for the general "list,
// then confirm" shape, didn't copy its layout or code) -- appropriately
// scoped for a first pass, not the final word on this UI.

// Forward declarations -- the engine's own RscText/RscButton/RscListbox
// exist at runtime, but the mission config compiler needs them declared
// before they're used as a base class here, or every `class X : RscText`
// below fails with "Undefined base class 'RscText'".
class RscText;
class RscButton;
class RscListbox;

class ALife_SpawnMenu
{
    idd = 4700;
    movingEnable = 0;
    enableSimulation = 1;

    class controlsBackground
    {
        class Background : RscText
        {
            idc = -1;
            x = 0.3; y = 0.2; w = 0.4; h = 0.55;
            colorBackground[] = { 0, 0, 0, 0.85 };
        };
    };

    class controls
    {
        class Title : RscText
        {
            idc = -1;
            text = "Select Faction & Spawn Point";
            x = 0.32; y = 0.21; w = 0.36; h = 0.04;
            sizeEx = 0.03;
        };

        class FactionList : RscListbox
        {
            idc = 4701;
            x = 0.32; y = 0.26; w = 0.36; h = 0.15;
            onLBSelChanged = "[] call ALife_fnc_spawnMenuFactionChanged";
        };

        class SpawnPointList : RscListbox
        {
            idc = 4702;
            x = 0.32; y = 0.43; w = 0.36; h = 0.2;
        };

        class SpawnButton : RscButton
        {
            idc = 4703;
            text = "Spawn";
            x = 0.32; y = 0.65; w = 0.36; h = 0.05;
            action = "[] call ALife_fnc_confirmSpawn";
        };
    };
};
