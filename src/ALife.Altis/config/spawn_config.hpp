// TasDyn-ALife — spawn point definitions.
//
// This is the source of truth for where players can spawn — config-driven
// (git-diffable, reviewable in a PR), not a database table, since spawn
// placement is a mission-design decision made during development, not
// something staff need to hot-edit on a live server (contrast with
// arsenal_item_pools in database/schema.sql, which genuinely does need
// that per docs/ADMIN_TOOLS.md — different requirement, different answer).
//
// Each entry:
//   displayName — shown in the spawn menu UI
//   faction     — "civilian" / "police" / "medic"
//   marker      — preferred: a marker name placed in Eden (visual, no
//                 hand-typed coordinates). Position resolves from this if
//                 set.
//   position[]  — fallback ATL position [x,y,z], used only when `marker`
//                 is empty. For spawn points that don't need — or don't
//                 yet have — a mission.sqm marker.
//   gang        — "" (not gang-restricted) or a gang key. NOT enforced
//                 yet — gangs.gang_members isn't loaded as part of the
//                 player record (see docs/DATA_CONTRACT.md), so there's
//                 nothing to check membership against until that exists.
//                 The field is here now so the config shape doesn't need
//                 to change later; see the TODO in fn_spawnPlayer.sqf.
//
// At least one of `marker`/`position[]` must resolve to something real —
// fn_getSpawnPoints.sqf doesn't currently validate that at load time.

class CfgSpawnPoints
{
    class civilian_kavala
    {
        displayName = "Kavala";
        faction = "civilian";
        marker = "civ_kavala_spawn";  // place this marker in Eden once mission.sqm exists
        position[] = {};
        gang = "";
    };

    class police_kavala_hq
    {
        displayName = "Kavala Police HQ";
        faction = "police";
        marker = "police_kav_spawn";
        position[] = {};
        gang = "";
    };

    class medic_example
    {
        displayName = "Example Medic Spawn";
        faction = "medic";
        marker = "spawn_medic_0";
        position[] = {};
        gang = "";
    };
};
