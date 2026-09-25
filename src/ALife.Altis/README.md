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
├── initServer.sqf          # Mission-level init: the HandleDisconnect hook
├── initPlayerServer.sqf    # Per-player join hook: load, then open the spawn menu
├── config/
│   └── spawn_config.hpp    # Spawn point definitions — see below
├── dialog/
│   └── spawnMenu.hpp       # Faction + spawn point selection dialog
└── functions/
    ├── data/                # DB-facing — implements docs/DATA_CONTRACT.md
    │   ├── fn_load.sqf
    │   └── fn_save.sqf
    └── spawn/               # Faction/spawn-point selection
        ├── fn_getSpawnPoints.sqf          # Reads config/spawn_config.hpp
        ├── fn_spawnPlayer.sqf             # Authoritative spawn handling (server)
        ├── fn_spawnMenu.sqf               # Opens the dialog (client)
        ├── fn_spawnMenuFactionChanged.sqf # Repopulates spawn points on faction change (client)
        └── fn_confirmSpawn.sqf            # Sends the spawn request to the server (client)
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
whatever's actually in `mission.sqm` is what goes in the `marker` field). `mission.sqm` currently
has one real marker placed: `police_kav_spawn` (Kavala), matching `spawn_config.hpp`'s
`police_kavala_hq` entry. Civilian/medic markers aren't placed yet — `civilian_kavala`'s
`civ_kavala_spawn` marker doesn't exist in `mission.sqm` yet. Add/rename/remove `CfgSpawnPoints`
entries to match whatever markers actually exist as more get placed.

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
- The spawn menu dialog (`dialog/spawnMenu.hpp`) is a first pass — a simple list-based UI, not a
  final design. Reviewed a real three-panel (list + map preview + confirm) spawn dialog for the
  general shape while designing this, but didn't copy its layout or code.
