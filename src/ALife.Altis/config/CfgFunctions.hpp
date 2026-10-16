// Function library. Server functions only ever run on the server; client
// functions only on a player's machine. Anything called over the network
// must also be listed in CfgRemoteExec.hpp.

class CfgFunctions
{
    class ALife
    {
        class Server
        {
            file = "functions\server";
            class extension {};      // the only place that calls the database DLL
            class serverInit {};     // database check, autosave, disconnect save
            class loadRecord {};     // load a player's record (retries)
            class requestSession {}; // client -> server: "load me"
            class spawnOptions {};   // spawn points a player may use
            class requestSpawn {};   // client -> server: "spawn me here"
            class savePlayer {};     // save alive/position
            class syncLoop {};       // periodic ping + autosave
        };

        class Client
        {
            file = "functions\client";
            class startSession {};    // ask the server for our record (join + respawn)
            class loading {};         // black loading overlay
            class sessionReceived {}; // server -> client: record loaded (or refused)
            class spawnMenu {};       // the spawn point picker
            class spawnApproved {};   // server -> client: go here
            class deathScreen {};     // death screen: killer, medic distance, respawn timer
        };

        class Common
        {
            file = "functions\common";
            class sideToFaction {};   // west/independent/civilian -> "cop"/"medic"/"civ"
            class parsePosition {};   // stored DB position -> [x,y,z]
            class killInfo {};        // killer name, weapon, distance
        };
    };
};
