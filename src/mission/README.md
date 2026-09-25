# src/mission/

SQF mission source for the ALife gamemode. Client requests, server decides:
gameplay code sends an intent to the server; the server is the only thing
that reads or writes persistent state via `src/cpp_extension`.

## What's here

```text
src/mission/
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
it's Eden editor output and has to come from actually building the mission in-game. To wire this
scaffold into a real mission:

1. Open Eden, create/open the mission, save it — this produces `mission.sqm` in the mission's
   working folder.
2. Place the spawn point markers described above.
3. Copy (or symlink) everything else in this folder — `description.ext`, `CfgFunctions.hpp`,
   `CfgRemoteExec.hpp`, `initServer.sqf`, `initPlayerServer.sqf`, `config/`, `dialog/`, and
   `functions/` — into that same folder, alongside the `mission.sqm` Eden created.
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
