# src/mission/

SQF mission source for the ALife gamemode. Client requests, server decides:
gameplay code sends an intent to the server; the server is the only thing
that reads or writes persistent state via `src/cpp_extension`.

## What's here

```text
src/mission/
├── description.ext        # Mission config — includes the two files below
├── CfgFunctions.hpp        # Registers ALife_fnc_* from functions/
├── CfgRemoteExec.hpp       # The remoteExec allowlist — see docs/ANTI_CHEAT.md Layer 1
├── initServer.sqf          # Mission-level init: the HandleDisconnect hook
├── initPlayerServer.sqf    # Per-player join hook: calls ALife_fnc_load
└── functions/
    ├── fn_load.sqf         # Implements docs/DATA_CONTRACT.md "load"
    └── fn_save.sqf         # Implements docs/DATA_CONTRACT.md "save"
```

## What's *not* here yet — `mission.sqm`

`mission.sqm` (the actual map layout — markers, spawn points, triggers) isn't generated here;
it's Eden editor output and has to come from actually building the mission in-game. To wire this
scaffold into a real mission:

1. Open Eden, create/open the mission, save it — this produces `mission.sqm` in the mission's
   working folder.
2. Copy (or symlink) `description.ext`, `CfgFunctions.hpp`, `CfgRemoteExec.hpp`, `initServer.sqf`,
   `initPlayerServer.sqf`, and `functions/` from here into that same folder, alongside the
   `mission.sqm` Eden created.
3. Point the mission's `config.ini` (see the repo root's `config.ini.example`) at a real Postgres
   instance — the C++ extension side of this (Phases 0–1) is already built and verified; see
   `src/cpp_extension/README.md`.

## Open items

- `initPlayerServer.sqf` doesn't yet send the loaded record back to the owning client — there's no
  client-side UI to send it to until Phase 2. It's intentionally kept server-side only
  (`setVariable` without the public flag) rather than broadcast, per
  [`ANTI_CHEAT.md`'s information-leakage guidance](../../docs/ANTI_CHEAT.md#5-explicitly-out-of-scope).
- The failure policy for a `load` returning `ERROR` (DB down, extension not connected) isn't
  decided — see the `TODO` in `initPlayerServer.sqf`. That's a product call (kick vs. retry vs.
  let them in flagged), not a technical one.
- `initServer.sqf`'s `HandleDisconnect` hook depends on an `alife_activeFaction` variable that
  doesn't exist yet — it's set when a player spawns into a faction, which is Phase 2 gameplay work
  (faction selection). The hook exits cleanly until then rather than guessing at that system.
