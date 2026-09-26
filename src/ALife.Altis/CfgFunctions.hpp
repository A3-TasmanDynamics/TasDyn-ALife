// TasDyn-ALife — function library registration.
//
// One category per functions/<name>/ subfolder — mirrors how Tonic's
// framework splits core/ by area (civilian, cop, session, admin, ...),
// used as an organizing idea rather than copied structure. Add a new
// category+folder as new systems get built (player, admin, economy, ...)
// rather than dumping everything in one flat folder — every function must
// be registered here before it can be referenced in CfgRemoteExec.hpp.

class CfgFunctions
{
    class ALife
    {
        class Data
        {
            file = "functions\data";
            class load {};                 // -> ALife_fnc_load
            class save {};                 // -> ALife_fnc_save
            class parseStoredPosition {};  // -> ALife_fnc_parseStoredPosition
        };

        class Spawn
        {
            file = "functions\spawn";
            class getSpawnPoints {};           // -> ALife_fnc_getSpawnPoints
            class spawnPlayer {};              // -> ALife_fnc_spawnPlayer (server)
            class spawnMenu {};                // -> ALife_fnc_spawnMenu (client, opens the dialog)
            class spawnMenuOpen {};            // -> ALife_fnc_spawnMenuOpen (client, dialog onLoad)
            class spawnMenuSelectSide {};       // -> ALife_fnc_spawnMenuSelectSide (client)
            class spawnMenuSelectLocation {};  // -> ALife_fnc_spawnMenuSelectLocation (client)
            class spawnMenuSpawn {};           // -> ALife_fnc_spawnMenuSpawn (client)
            class spawnMenuClose {};           // -> ALife_fnc_spawnMenuClose (client, dialog onUnload)
        };
    };
};
