# src/ALife.altis/

SQF mission source for the ALife gamemode. Client requests, server decides:
gameplay code sends an intent to the server; the server is the only thing
that reads or writes persistent state via `src/cpp_extension`.

Named `ALife.altis` to follow Arma 3's own `<mission name>.<world>` mission-folder convention —
`altis` because that's the terrain this is built against right now. If the terrain ever changes,
this folder gets renamed to match; the two are meant to stay in lockstep, not drift.

## What's here

```text
src/ALife.altis/
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

Each `CfgSpawnPoints` entry prefers a marker (`marker = "spawn_<faction>_<n>";`) placed visually in
Eden over hand-typed coordinates. **Once `mission.sqm` exists, place a marker in Eden for every
entry in `spawn_config.hpp`, named exactly what that entry's `marker` field says** — the three
`*_example` entries currently there expect `spawn_civilian_0`, `spawn_police_0`, and
`spawn_medic_0`. Add/rename/remove entries in `spawn_config.hpp` to match whatever markers you
actually place — the example entries are placeholders, not a fixed set.

## What's *not* here yet — `mission.sqm`

`mission.sqm` (the actual map layout — markers, spawn points, triggers) isn't generated here;
it's Eden editor output and has to come from actually building the mission in-game. Because this
folder is already named to match Arma's own convention, the natural path is to make it the mission
folder directly rather than building one elsewhere and copying files in afterward:

1. Symlink (or copy) this folder into your Arma 3 install's `MPMissions/` (or into your dev
   workspace's missions folder, if you use one) as `ALife.altis` — same name, so Eden recognizes
   it as a mission for the Altis terrain.
2. Open it in Eden (or start a new mission on Altis named `ALife` — Eden will create/use a folder
   by that same name) and save. This produces `mission.sqm` **directly alongside the files already
   here** — no copy step needed once the folder is in the right place under the right name.
3. Place the spawn point markers described above.
4. Point the mission's `config.ini` (see the repo root's `config.ini.example`) at a real Postgres
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
