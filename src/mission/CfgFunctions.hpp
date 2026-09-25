// TasDyn-ALife — function library registration.
// Add new categories/functions here as the SQF side grows — every
// function must be registered here before it can be referenced in
// CfgRemoteExec.hpp.

class CfgFunctions
{
    class ALife
    {
        class Persistence
        {
            file = "functions";
            class load {};    // -> ALife_fnc_load, functions\fn_load.sqf
            class save {};    // -> ALife_fnc_save, functions\fn_save.sqf
        };

        class Spawning
        {
            file = "functions";
            class getSpawnPoints {};        // -> ALife_fnc_getSpawnPoints
            class spawnPlayer {};            // -> ALife_fnc_spawnPlayer (server)
            class spawnMenu {};               // -> ALife_fnc_spawnMenu (client)
            class spawnMenuFactionChanged {}; // -> ALife_fnc_spawnMenuFactionChanged (client)
            class confirmSpawn {};             // -> ALife_fnc_confirmSpawn (client)
        };
    };
};
