# src/ALife.Altis/

SQF mission source for the ALife gamemode. Client requests, server decides:
gameplay code sends an intent to the server; the server is the only thing
that reads or writes persistent state via `src/cpp_extension`.

Named `ALife.Altis` to follow Arma 3's own `<mission name>.<world>` mission-folder convention —
`Altis` because that's the terrain this is built against right now. If the terrain ever changes,
this folder gets renamed to match; the two are meant to stay in lockstep, not drift.

## What's here

```text
src/ALife.Altis/
├── description.ext        # Mission config — includes everything below
├── CfgFunctions.hpp        # Registers ALife_fnc_* from functions/
├── CfgRemoteExec.hpp       # The remoteExec allowlist — see docs/ANTI_CHEAT.md Layer 1
├── initServer.sqf          # Mission-level init: HandleDisconnect + starts fn_sync.sqf
├── initPlayerServer.sqf    # Per-player join hook: load, then open the spawn menu
├── config/
│   └── spawn_config.hpp    # Spawn point definitions — see below
├── dialog/
│   ├── common_ui.hpp       # Shared ALife_Rsc* dialog base classes
│   └── spawnMenu.hpp       # Faction + spawn point selection dialog
└── functions/
    ├── data/                          # DB-facing — implements docs/DATA_CONTRACT.md
    │   ├── fn_callExtension.sqf         # The only place that calls "tasdyn_alife" directly
    │   ├── fn_load.sqf
    │   ├── fn_save.sqf
    │   ├── fn_parseStoredPosition.sqf  # Safely parses a loaded <faction>_position
    │   ├── fn_savePlayerState.sqf      # Saves alive/position -- shared by disconnect + sync
    │   └── fn_sync.sqf                 # Periodic pulse: DB keep-alive + autosave -- see below
    └── spawn/                # Faction/spawn-point selection
        ├── fn_getSpawnPoints.sqf  # Reads config/spawn_config.hpp
        ├── fn_spawnPlayer.sqf     # Authoritative spawn handling (server)
        └── fn_spawnMenu.sqf       # Everything about the dialog (client), mode-dispatched:
                                   # open / onLoad / selectSide / selectLocation / spawn / onUnload
```

One subfolder per area under `functions/` (`data/`, `spawn/`, more to come — `player/`, `admin/`,
etc. as those systems get built) — an idea taken from how Tonic's framework splits its `core/`
folder by area, not a copy of its actual structure. `CfgFunctions.hpp`'s `class` nesting mirrors
this 1:1 — add a new subfolder and a matching `CfgFunctions.hpp` category together, not one
without the other.

## Spawn points

Defined in `config/spawn_config.hpp`, not a database table — see the comment at the top of that
file for why (short version: it's a mission-design decision made during development, not
something staff need to hot-edit on a live server, unlike `arsenal_item_pools`).

Each `CfgSpawnPoints` entry references a marker by name (no fixed naming pattern required —
whatever's actually in `mission.sqm` is what goes in the `marker` field). Three real markers are
placed in Kavala: `civilian_kavala_spawn`, `police_kavala_spawn`, `medic_kavala_spawn`, matching
`spawn_config.hpp`'s `civilian_kavala` / `police_kavala_hq` / `medic_kavala_hospital` entries.
Each entry's `faction` value is `"civ"` / `"cop"` / `"medic"` — matching
`docs/DATA_CONTRACT.md`'s `players.<faction>_*` field prefix directly (see "Faction naming"
below) — not the `displayName`, which is free text. Add/rename/remove `CfgSpawnPoints` entries to
match whatever markers actually exist as more get placed.

## Faction naming

The spawn menu, `config/spawn_config.hpp`, and `fn_spawnPlayer.sqf`/`initServer.sqf`'s
`HandleDisconnect` all use `"civ"` / `"cop"` / `"medic"` as the faction identifier — matching
`docs/DATA_CONTRACT.md`'s `players.<faction>_*` field prefix exactly, so a faction string can be
concatenated straight into a field name (`_faction + "_alive"`) with no translation step. This
used to be `"civilian"`/`"police"`/`"medic"` here (matching `bank_accounts.faction`'s naming
elsewhere in `database/schema.sql`), and that mismatch was a real, silent bug: `"police_position"`
never matched the real `cop_position` column, so police/civilian death-position persistence
silently no-opped. If gang/bank features ever get wired into this mission, they'll need to
translate at that boundary instead — not here.

## Sync / autosave

`fn_sync.sqf` is spawned once from `initServer.sqf` and runs for the mission's whole lifetime
(guarded against the same function-compile race `initPlayerServer.sqf`'s `ALife_fnc_load` call
needed — see the comment there). Every 60 seconds it:

- **Pings the DB** (`["ping", []] call ALife_fnc_callExtension`) and logs its connection state on
  start unconditionally, then again on any later state *change* (lost/restored), not every tick.
  This exists because the C++ extension didn't used to reconnect
  on its own at all — a Postgres restart or a network blip would silently fail every load/save for
  the rest of the server's uptime. `src/cpp_extension/src/db.cpp`'s `EnsureConnected` now recovers
  from that automatically (see that repo's README) — the ping here is what actually *triggers*
  that recovery on a schedule, rather than only ever discovering a dead connection the next time a
  player happens to load/save.
- **Autosaves** every connected, actively-playing player's alive/position state via
  `fn_savePlayerState.sqf` — the same logic `initServer.sqf`'s `HandleDisconnect` uses for a clean
  quit, factored out since `HandleDisconnect` never fires on an ungraceful server crash at all, and
  that's exactly the case autosave exists to cover.

Reviewed an old local prototype's `fn_autoSave.sqf`/`fn_serverPulse.sqf` for the general shape
(spawn one loop, sleep, act) — not copied wholesale; nothing here needs a generic external-event
poll the way that prototype's did, since this mission doesn't have a cross-system command queue
(website/Discord → live server) yet. That's a real, separate feature for later, not something this
pass builds.

## `mission.sqm`

Exists — built in Eden and living directly in this folder (Arma's own `MPMissions/` naming
convention meant no copy step was needed once the folder had the right name). Deploying it to an
actual local dedicated server for testing:

- **`tools/test_local_server.ps1`** automates the deploy-run-check-cleanup cycle — see
  `tools/README.md`. Verified clean against a real Arma 3 Server install: the entire config stack
  here (`description.ext`, `CfgFunctions.hpp`, `CfgRemoteExec.hpp`, `config/spawn_config.hpp`,
  `dialog/spawnMenu.hpp`) loads with zero errors and the server reaches a stable, ready state.
  That script can't drive an actual player connecting, though — `load`/`save`/spawn triggered by a
  real join are still unverified end-to-end.
- Point the mission's `config.ini` (see the repo root's `config.ini.example`) at a real Postgres
  instance — the C++ extension side of this (Phases 0–1) is already built and verified; see
  `src/cpp_extension/README.md`.

## Open items

- `initPlayerServer.sqf`'s failure policy for a `load` returning `ERROR` (DB down, extension not
  connected) isn't decided — see the `TODO` there. That's a product call (kick vs. retry vs. let
  them in flagged), not a technical one.
- Gear/loadout equipping isn't wired into `fn_spawnPlayer.sqf` yet — `civ_gear`/`cop_gear`/
  `medic_gear` don't have a documented item-key contract (`docs/DATA_CONTRACT.md` only says "full
  loadout" generically). Write that contract before wiring gear application.
- `spawn_config.hpp`'s `gang` field isn't enforced yet — gang membership isn't part of the loaded
  record (`docs/DATA_CONTRACT.md`), so `fn_spawnPlayer.sqf` has nothing to check it against until
  that exists.
- Spawn point rank/department gating (a real framework's spawn config supports per-point
  `minRank`/`departments`/arbitrary `conditions`) isn't implemented — there's no rank/department
  data in the loaded record yet for it to check against. `config/spawn_config.hpp`'s shape doesn't
  preclude adding it later.
