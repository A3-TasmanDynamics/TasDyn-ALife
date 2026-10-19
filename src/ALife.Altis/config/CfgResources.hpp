// Gathering and processing (resources). Items are in CfgItems, prices in
// CfgMarket, and what buys them in VirtualShops + CfgShops (the Resource
// Trader).
//
// Markers: gathering zones are AREA markers (ellipse / rectangle);
// processors are point markers where the worker stands. A missing marker
// just means that spot doesn't exist yet. Add more spots for one resource by
// listing more markers.
//
// Press Windows inside a gathering area to gather (a pickaxe for mining).
// Processing is done by an NPC: the server puts a worker (npcClass) at each
// processor marker, facing the marker's direction; look at them and press
// Windows to process. The server times both and checks again
// before anything changes (ALife_fnc_requestGather, ALife_fnc_requestProcess).

class CfgGathering
{
    //   name      -- shown while gathering ("Mining copper...")
    //   item      -- CfgItems class you get
    //   amount[]  -- {min, max} per gather (what fits your carry weight)
    //   time      -- seconds per gather
    //   tool      -- CfgItems class needed ("" = none)
    //   zones[]   -- area markers it can be gathered in
    class oil     { name = "oil";      item = "oil_raw";     amount[] = { 1, 2 }; time = 5; tool = "pickaxe"; zones[] = { "mine_oil" }; };
    class copper  { name = "copper";   item = "copper_ore";  amount[] = { 1, 3 }; time = 4; tool = "pickaxe"; zones[] = { "mine_copper" }; };
    class iron    { name = "iron";     item = "iron_ore";    amount[] = { 1, 3 }; time = 4; tool = "pickaxe"; zones[] = { "mine_iron" }; };
    class salt    { name = "salt";     item = "salt_raw";    amount[] = { 1, 3 }; time = 3; tool = "pickaxe"; zones[] = { "mine_salt" }; };
    class sand    { name = "sand";     item = "sand";        amount[] = { 1, 3 }; time = 3; tool = "pickaxe"; zones[] = { "mine_sand" }; };
    class diamond { name = "diamonds"; item = "diamond_raw"; amount[] = { 1, 1 }; time = 6; tool = "pickaxe"; zones[] = { "mine_diamond" }; };
    class rock    { name = "rock";     item = "rock";        amount[] = { 1, 2 }; time = 4; tool = "pickaxe"; zones[] = { "mine_rock" }; };
};

class CfgProcessing
{
    npcClass = "C_Man_ConstructionWorker_01_Black_F"; // the worker at each processor

    //   name       -- the processor, shown in the menu ("Process at <name>")
    //   from[]     -- {{item, count}, ...} used per batch
    //   to[]       -- {{item, count}, ...} made per batch
    //   time       -- seconds per batch (everything you carry is processed in one go)
    //   maxTime    -- the whole job never takes longer than this
    //   licence    -- CfgLicences key that makes it free ("" = always free)
    //   fee        -- dollars per batch without that licence (taken from cash)
    //   markers[]  -- point markers: a worker stands at each one
//   radius     -- how close to the worker you must be
    class oil     { name = "Oil Refinery";    from[] = { { "oil_raw", 1 } };     to[] = { { "oil_refined", 1 } }; time = 1.5; maxTime = 30; licence = "oil";     fee = 400; markers[] = { "process_oil" }; radius = 5; };
    class copper  { name = "Copper Smelter";  from[] = { { "copper_ore", 1 } };  to[] = { { "copper_bar", 1 } };  time = 1;   maxTime = 30; licence = "copper";  fee = 250; markers[] = { "process_copper" }; radius = 5; };
    class iron    { name = "Iron Smelter";    from[] = { { "iron_ore", 1 } };    to[] = { { "iron_bar", 1 } };    time = 1;   maxTime = 30; licence = "iron";    fee = 300; markers[] = { "process_iron" }; radius = 5; };
    class salt    { name = "Salt Refinery";   from[] = { { "salt_raw", 1 } };    to[] = { { "salt", 1 } };        time = 1;   maxTime = 30; licence = "salt";    fee = 200; markers[] = { "process_salt" }; radius = 5; };
    class glass   { name = "Glass Works";     from[] = { { "sand", 1 } };        to[] = { { "glass", 1 } };       time = 1;   maxTime = 30; licence = "sand";    fee = 200; markers[] = { "process_glass" }; radius = 5; };
    class diamond { name = "Diamond Cutter";  from[] = { { "diamond_raw", 1 } }; to[] = { { "diamond", 1 } };     time = 2;   maxTime = 30; licence = "diamond"; fee = 600; markers[] = { "process_diamond" }; radius = 5; };
    class cement  { name = "Cement Plant";    from[] = { { "rock", 1 } };        to[] = { { "cement", 1 } };      time = 1.5; maxTime = 30; licence = "cement";  fee = 300; markers[] = { "process_cement" }; radius = 5; };
};
