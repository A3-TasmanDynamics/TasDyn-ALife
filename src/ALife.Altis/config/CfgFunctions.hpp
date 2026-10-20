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
            class itemsChange {};    // add/remove virtual items (saved)
            class requestPhoneData {}; // send a player their phone data
            class market {};
            class shopCharge {};      // take a shop payment (cash or bank)
            class shopBuyClothing {}; // client -> server: buy the clothing being tried on
            class shopBuyVehicle {};  // client -> server: buy a vehicle (spawned, owned, recorded)
            class vehicleLock {};     // client -> server: U on your own vehicle         // the dynamic market: prices, trades, recovery (CfgMarket)
            class shopBuy {};        // client -> server: buy from a shop
            class giveItem {};       // client -> server: give/discard items
            class bankTransfer {};   // client -> server: send money to a player
            class sendMessage {};    // client -> server: phone message
            class requestRevive {};  // medic's client -> server: revive finished
            class completeRevive {}; // revived player's client -> server: put me back
            class requestUseItem {}; // client -> server: the Use button finished
            class jail {};           // the jail: arrest, sentence clock, release, bail
            class jailBail {};       // client -> server: pay bail
            class wantedBoard {};    // client -> server: the Wanted app (police)
            class placeables {};     // police placeables: spike strips, picking up
            class vehicleAction {};  // client -> server: push / give a key / impound
            class personAction {};
            class dropGear {};        // a player's Arma gear onto the ground (strip)   // client -> server: restrain / fine / strip / heal
            class adminAction {};    // client -> server: the admin menu (checked against website permissions)
            class shopSell {};       // client -> server: sell to a trader
            class requestGather {};  // client -> server: gather a resource (timed)
            class requestProcess {}; // client -> server: process resources (timed)
            class buyLicence {};     // client -> server: buy a processing licence at its processor
            class garage {};         // client -> server: the garage (list / take out / store)
            class vehicleSpawn {};   // an owned vehicle into the world (shop, garage)
            class vehicleState {};   // an owned vehicle's fuel / damage / trunk, for saving
            class containerGet {};   // the container (bag / vehicle trunk) behind an object
            class containerCreateBag {}; // drop a bag at a player's feet
            class containerSend {};  // send a player a container's contents
            class containerOpen {};  // client -> server: open a bag / trunk
            class containerTransfer {}; // client -> server: take / store items and cash
            class atmAction {};      // client -> server: ATM deposit / withdraw
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
            class setupUnit {};       // per-spawn setup: death screen, revive/shop actions, settings
            class notify {};          // server -> client: short notice (as a toast)
            class toast {};           // our in-game notifications (instead of hints)
            class phone {};           // the phone (Y)
            class phoneData {};       // server -> client: items, money, levels
            class phoneSettings {};   // view distance / grass
            class messageReceived {}; // server -> client: phone message
            class shop {};
            class clothingShop {};    // clothing shop: 3D try-on
            class applySkins {};      // uniform textures by group; hidden cop / medic backpacks
            class survival {};        // food, water and the player HUD
            class playerTags {};      // name tags (police: rank under the name)
            class groupMarkers {};    // map / GPS markers for your police, EMS or gang (with a GPS)
            class vehicleShop {};     // vehicle shop: 3D showroom, colours
            class vehicleLockApply {}; // set a lock where the vehicle is local            // shop window
            class revive {};          // medic: the Revive action
            class revived {};         // server -> downed client: a medic revived you
            class reviveApplied {};   // server -> client: back up where you fell
            class jailClient {};      // server -> client: into / out of jail
            class wanted {};          // the phone's Wanted app data (police)
            class useItem {};         // the Use button: progress bar, then the server
            class radial {};          // the Windows-key radial menu
            class vehicleMenu {};     // a vehicle's radial menu
            class vehicleApply {};    // server -> client: push a vehicle where it's local
            class personMenu {};      // a player's radial menu
            class personAct {};       // timed things done to a player (heal)
            class restrained {};      // server -> client: you're restrained
            class surrender {};       // Left Shift + B: hands on your head
            class holster {};         // Left Shift + H: weapon away / out
            class jump {};            // Left Shift + Space: jump while running
            class placeMode {};       // choosing where a police object goes
            class emergency {};       // police / EMS lights and sirens (keys, watcher)
            class emergencyLights {}; // one vehicle's flashing light bar, here
            class emergencySiren {};  // one vehicle's siren, here
            class fine {};            // server -> client: a fine to pay or refuse
            class personApply {};     // server -> client: police strip you
            class admin {};           // the admin menu (phone -> Admin)
            class adminApply {};      // server -> client: staff actions on you (bring, freeze, kick, ...)
            class resourceProgress {}; // server -> client: progress bar while gathering / processing
            class garageMenu {};      // the garage window
            class licenceShop {};     // the licence office window
            class itemUsed {};        // server -> client: apply a used item where it's local
            class interact {};        // the Windows key
            class transfer {};        // the inventory screen (bags, trunks, dropping)
            class containerData {};   // server -> client: what's in the open bag / trunk
            class atm {};             // the ATM / bank menu
        };

        class Common
        {
            file = "functions\common";
            class sideToFaction {};   // west/independent/civilian -> "cop"/"medic"/"civ"
            class vehicleTyres {};    // pop a vehicle's tyres (spike strips)
            class atPlace {};         // is a unit at a shop / processor / gathering area (markers or NPCs)
            class parsePosition {};   // stored DB position -> [x,y,z]
            class killInfo {};        // killer name, weapon, distance
            class itemsWeight {};     // weight of a list of items
            class isAtm {};           // is this object an ATM (any on the map)
            class shopAccess {};      // licence / rank a shop or shop entry needs
            class marketPrice {};     // what a market item costs right now
            class shopInfo {};        // a CfgShops placement -> its type and shop
            class vehicleSkin {};     // a skin choice -> its textures
        };
    };
};
