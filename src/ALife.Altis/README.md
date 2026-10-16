# src/ALife.Altis/

The Altis Life mission, rebuilt from scratch (October 2026) around one goal: **join → load your
character from the database → pick a spawn point → play**, with your state kept in sync. Every
other system (shops, jobs, gear, whitelisting, ...) builds on top of this.

## The join flow

1. **Pick a slot** in the lobby. The slot's side is your faction:

   | Lobby side | Faction | Database prefix |
   |---|---|---|
   | BLUFOR | Police | `cop_*` |
   | Independent | Medic | `medic_*` |
   | Civilian | Civilian | `civ_*` |

2. **Loading screen.** `initPlayerLocal.sqf` → `ALife_fnc_startSession` freezes you behind a black
   overlay, waits until the server says its database is connected, then asks the server for your
   character (`ALife_fnc_requestSession`). The overlay always says what it's waiting for, and shows
   any problem in red instead of hanging.
3. **The server loads you** (`ALife_fnc_loadRecord`, 3 tries). A first-time player gets a blank
   record. Banned players are refused with the reason. Your in-game name is saved to the database.
4. **Spawn menu.** A list on the left, the map on the right, and your faction, cash and bank in the
   header (straight from the database). If you logged off alive, **Last position** is the first
   option.
5. **Spawn.** The client sends only the spawn point's key (`ALife_fnc_requestSpawn`). The server
   checks it's one of your faction's points, finds a clear spot nearby, saves you as alive there,
   and tells your client to move in (`ALife_fnc_spawnApproved`).

## Dying

Dying sets `<faction>_alive = false` and opens the **death screen** (`ALife_fnc_deathScreen`,
`ui/deathScreen.hpp`) titled **YOU ARE INCAPACITATED**. The view switches to a camera 5 m above
your body looking straight down, following it until you respawn. There's no way to leave from the
death screen: disconnecting while incapacitated is combat logging, and the server logs it as
`[ALife][COMBAT LOG]`.

- **Killed by** — the killer's name, weapon (or vehicle, for a roadkill) and distance in metres
  (`ALife_fnc_killInfo`; also written to the server log). Falls and drowning just say "You died."
- **Nearest medic** — live distance to the closest spawned-in medic, or "No medics on duty".
- **Respawn timer** — a bar from 100% to 0% over `CfgALife >> respawnTime` (120 s, in
  `config/CfgALife.hpp`). Respawn is locked until 0%; after that you can respawn or keep waiting
  for a medic. The game's own automatic respawn is held off the whole time.

Respawning takes you back through the spawn menu (`onPlayerRespawn.sqf`). Medics can't revive
yet — that's the next step for the medic side.

## What's saved, and when

| When | What |
|---|---|
| Join | `name` (if it changed) |
| Spawn | `<faction>_alive = true`, `<faction>_position` |
| Every 60 s | alive + position for everyone spawned (also pings the database) |
| Death | `<faction>_alive = false` |
| Disconnect | alive + position |

All database access goes through `ALife_fnc_extension`, which calls the C++ extension
(`src/cpp_extension`, commands `ping` / `load` / `save`; see `docs/DATA_CONTRACT.md`).

## Files

```text
description.ext          mission settings (respawn, no AI in empty slots), includes the rest
mission.sqm              slots (10 police, 8 medic, 30 civilian) + spawn/respawn markers
initServer.sqf           -> ALife_fnc_serverInit
initPlayerLocal.sqf      -> ALife_fnc_startSession
onPlayerRespawn.sqf      -> ALife_fnc_startSession again
config/
  CfgFunctions.hpp       function list
  CfgRemoteExec.hpp      the only functions callable over the network
  CfgSpawnPoints.hpp     spawn points per faction
  CfgALife.hpp           gameplay settings (respawn timer)
ui/
  base.hpp               shared control classes (kept from the previous mission: each one
                         was fixed against a real in-game error, the map especially)
  loading.hpp            the black loading overlay
  spawnMenu.hpp          the spawn picker
  deathScreen.hpp        the death screen
functions/
  server/                extension, serverInit, loadRecord, requestSession, spawnOptions,
                         requestSpawn, savePlayer, syncLoop
  client/                startSession, loading, sessionReceived, spawnMenu, spawnApproved,
                         deathScreen
  common/                sideToFaction, parsePosition, killInfo
```

## Spawn points

`config/CfgSpawnPoints.hpp` lists them per faction; each points at a marker in `mission.sqm`.
**To move one, drag its marker in Eden** (`spawn_civ_kavala`, `spawn_cop_athira`, ...). To add one,
place a new empty marker and add an entry. The Kavala markers are the positions the 2025 build used
in-game; Athira, Pyrgos and Sofia are town centres — move them onto the station or hospital.

## Server requirements

- `tasdyn_alife_x64.dll` and its `config.ini` next to `arma3server_x64.exe` (Server Manager
  deploys both when it starts the server).

## Not built yet

- Cop/medic whitelisting (anyone can take a Police or Medic slot).
- Gear/loadouts (slot weapons are removed on spawn; uniforms stay).
- Cash/bank use in game (they're loaded and shown, not yet spent).
