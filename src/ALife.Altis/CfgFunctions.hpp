// TasDyn-ALife — function library registration.
//
// One category per core/functions/<name>/ subfolder — mirrors how Tonic's
// framework splits everything under core/ by area (civilian, cop, session,
// admin, ...). Add a new category+folder as new systems get built (player,
// admin, economy, ...) rather than dumping everything in one flat folder —
// every function must be registered here before it can be referenced in
// CfgRemoteExec.hpp.

class CfgFunctions
{
    class ALife
    {
        class Data
        {
            file = "core\functions\data";
            class callExtension {};        // -> ALife_fnc_callExtension
            class load {};                 // -> ALife_fnc_load
            class save {};                 // -> ALife_fnc_save
            class parseStoredPosition {};  // -> ALife_fnc_parseStoredPosition
            class savePlayerState {};      // -> ALife_fnc_savePlayerState (server)
            class sync {};                 // -> ALife_fnc_sync (server, spawned once, loops forever)
            class playerJoin {};           // -> ALife_fnc_playerJoin (server) -- remoteExec'd from
                                            // initPlayerLocal.sqf, see CfgRemoteExec.hpp
        };

        class Spawn
        {
            file = "core\functions\spawn";
            class getSpawnPoints {};  // -> ALife_fnc_getSpawnPoints
            class sideToFaction {};   // -> ALife_fnc_sideToFaction -- maps an Arma side to
                                      // "civ"/"cop"/"medic"; the one source of truth for that,
                                      // client or server (see docs/TONIC_REFERENCE.md §3)
            class spawnPlayer {};     // -> ALife_fnc_spawnPlayer (server)
            class spawnMenu {};       // -> ALife_fnc_spawnMenu (client) -- one file, mode-dispatched:
                                      // open/onLoad/selectLocation/spawn/onUnload
        };

        class UI
        {
            file = "core\functions\ui";
            class loadingScreen {};  // -> ALife_fnc_loadingScreen (client) -- one file, mode-
                                     // dispatched: open/onLoad/setProgress/onUnload -- see
                                     // initPlayerLocal.sqf and fn_playerJoin.sqf for the real
                                     // join-progress milestones that drive it
        };
    };
};
