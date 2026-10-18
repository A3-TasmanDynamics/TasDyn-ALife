// Spawn points, per faction ("civ" / "cop" / "medic" -- the same prefixes
// as the players table: civ_cash, cop_position, ...).
//
//   displayName -- shown in the spawn menu
//   description -- one line under the name
//   marker      -- a marker in mission.sqm; drag it in Eden to move the
//                  spawn point. Players land within `radius` metres of it,
//                  on a clear spot.
//   radius      -- spread so players don't stack on one spot
//
// The Kavala markers are the positions the 2025 build used in-game. The
// Athira, Pyrgos and Sofia markers are town centres -- move them onto the
// station/hospital you want in Eden.

class CfgSpawnPoints
{
    class civ
    {
        class kavala { displayName = "Kavala"; description = "The capital. Shops, jobs and the main square."; marker = "spawn_civ_kavala"; radius = 15; };
        class athira { displayName = "Athira"; description = "Central Altis."; marker = "spawn_civ_athira"; radius = 25; };
        class pyrgos { displayName = "Pyrgos"; description = "South-east coast."; marker = "spawn_civ_pyrgos"; radius = 25; };
        class sofia  { displayName = "Sofia"; description = "The far north-east."; marker = "spawn_civ_sofia"; radius = 25; };
    };

    class cop
    {
        class kavala_hq { displayName = "Kavala Police HQ"; description = "Headquarters."; marker = "spawn_cop_kavala"; radius = 8; };
        class athira    { displayName = "Athira Police Station"; description = "Central Altis."; marker = "spawn_cop_athira"; radius = 10; };
        class pyrgos    { displayName = "Pyrgos Police Station"; description = "South-east."; marker = "spawn_cop_pyrgos"; radius = 10; };
    };

    class medic
    {
        class kavala_hospital { displayName = "Kavala Hospital"; description = "Main hospital."; marker = "spawn_medic_kavala"; radius = 8; };
        class athira          { displayName = "Athira Hospital"; description = "Central Altis."; marker = "spawn_medic_athira"; radius = 10; };
        class pyrgos          { displayName = "Pyrgos Hospital"; description = "South-east."; marker = "spawn_medic_pyrgos"; radius = 10; };
    };

    class rebel
    {
        class kavala { displayName = "Kavala"; description = "The capital. Shops, jobs and the main square."; marker = "spawn_civ_kavala"; radius = 15; };
        class athira { displayName = "Athira"; description = "Central Altis."; marker = "spawn_civ_athira"; radius = 25; };
        class pyrgos { displayName = "Pyrgos"; description = "South-east coast."; marker = "spawn_civ_pyrgos"; radius = 25; };
        class sofia  { displayName = "Sofia"; description = "The far north-east."; marker = "spawn_civ_sofia"; radius = 25; };
    };

    class cartel
    {
        class kavala { displayName = "Kavala"; description = "The capital. Shops, jobs and the main square."; marker = "spawn_civ_kavala"; radius = 15; };
        class athira { displayName = "Athira"; description = "Central Altis."; marker = "spawn_civ_athira"; radius = 25; };
        class pyrgos { displayName = "Pyrgos"; description = "South-east coast."; marker = "spawn_civ_pyrgos"; radius = 25; };
        class sofia  { displayName = "Sofia"; description = "The far north-east."; marker = "spawn_civ_sofia"; radius = 25; };
    };
};
