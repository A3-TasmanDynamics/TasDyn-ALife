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

   Police slots 1–4 are **public**; slots 5–10 are **whitelisted** (`cop_level` 1+, set in
   `config/CfgALife.hpp`). Taking a whitelisted slot without the level shows a message and returns
   you to the lobby.

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
death screen: disconnecting while incapacitated is combat logging. The server records it (the
extension's `combatlog` command) as a **Combat logging** anti-cheat flag (website → Anti-Cheat
Flags) and a staff log entry (website → Staff Log, posted to Discord #staff-log), with who downed
them, the weapon, the distance and how long they'd been down.

- **Killed by** — the killer's name, weapon (or vehicle, for a roadkill) and distance in metres
  (`ALife_fnc_killInfo`; also written to the server log). Falls and drowning just say "You died."
- **Nearest medic** — live distance to the closest spawned-in medic, or "No medics on duty".
- **Respawn timer** — a bar from 100% to 0% over `CfgALife >> respawnTime` (120 s, in
  `config/CfgALife.hpp`). Respawn is locked until 0%; after that you can respawn or keep waiting
  for a medic. The game's own automatic respawn is held off the whole time.

Respawning takes you back through the spawn menu (`onPlayerRespawn.sqf`).

### Revive

Medics revive downed players with a **medkit** (a virtual item). Look at the body and use
**Revive** (Windows key on a downed player, then Revive): an 8-second action with a progress bar, cancelled if you move away. When
it finishes the server checks the medic has a medkit, uses one, and the downed player gets up where
they fell, with their gear, at 50% health (`reviveTime` / `reviveHealth` in `config/CfgALife.hpp`).
Flow: `ALife_fnc_revive` (medic) → `ALife_fnc_requestRevive` (server) → `ALife_fnc_revived`
(patient respawns) → `ALife_fnc_completeRevive` (server) → `ALife_fnc_reviveApplied`.

## Phone (Y)

A Galaxy-style phone (`ui/phone.hpp`, `ALife_fnc_phone`): a clock and a 3 x 3 grid of app icons
(`ui/icons/*.paa`). The icons and the layout are generated — edit and re-run
`tools/phone_icons/make_icons.py` (needs Arma 3 Tools' ImageToPAA), `make_frame.py` (the curved phone
and its wallpapers) or `make_layout.py`.

| App | What it does |
|---|---|
| Messages | Inbox (this session); send to Police, EMS or a player, optionally anonymous (the server log keeps the real sender) |
| Bank | Bank and cash balance; send money from your bank to any online player |
| Inventory | Your virtual items for the faction you're playing; give to a player within 5 m, or discard |
| Player ID | Your name, faction, police/EMS level, staff rank and licences (`config/CfgLicences.hpp` names them) |
| Settings | View distance, grass on/off, wallpaper (Dark by default, plus Dusk, Sunset, Ocean, Forest, Aurora; `config/CfgPhone.hpp`) — saved in your Arma profile |
| Perks, Gang, Keys, Admin | "Coming soon" (Perks will be a talent tree) |

Everything that changes something is checked by the server (`ALife_fnc_giveItem`,
`ALife_fnc_bankTransfer`, `ALife_fnc_sendMessage`), which answers with fresh phone data.

## Notifications

Our own notifications replace Arma's hint box (`ALife_fnc_toast`; the server sends them with
`ALife_fnc_notify`): cards stacking in the top-right with a coloured stripe — green done, amber
heads-up, red error, blue info, purple messages — fading after 5 s, drawn above any open menu.

## Inventory screen (Windows key)

One screen (`ui/transfer.hpp`, `ALife_fnc_transfer`) for dropped bags and vehicle trunks — the
bag/trunk on the left, you on the right, with weights.

- **Windows key** (`ALife_fnc_interact`): looking at a dropped bag (within 4 m) or a vehicle (within
  8 m) opens it; looking at nothing opens your inventory next to the **ground**, so you can drop
  things. Dropping leaves a bag at your feet (`CfgALife >> bagClass`, a placeholder backpack model —
  a simple object, so Arma's own inventory can't touch it). Bags vanish when emptied or after
  `bagLifetime` (15 min); they aren't saved over restarts.
- **Moving things:** select rows (Ctrl/Shift for several), enter a quantity, **Take** / **Store**
  (**Drop** on the ground); **Take all** / **Store all**; double-click a row to move all of it.
- **Dropping only cash** leaves a stack of money (`CfgALife >> cashClass`, `Land_Money_F`) instead of a
  bag. Windows on a stack that's only cash picks it straight up (no menu).
- **Cash** is a row like any item (also in the phone's Inventory, where you can give it to someone
  within 5 m or drop it): physical money you carry (`<faction>_cash`), moved with the same
  idempotent delta save as purchases.
- **Limits:** what you carry is capped at `carryWeight` (24), trunks by vehicle type
  (CfgVehicles `trunk`, or `CfgVehicles >> TrunkDefaults` by type for unlisted vehicles), using each item's `weight`.
- The server does all the moving (`ALife_fnc_containerTransfer`, run with `remoteExecCall` so two
  players can't race each other) and checks distance, what each side has, and space.
  Trunk contents live on the server only (not saved yet).

## ATMs (Windows key)

Every ATM on the map works — no markers: `ALife_fnc_isAtm` recognises them by model or class
(`CfgALife >> atmModels` / `atmClasses`). Look at one and press **Windows** for the bank menu
(`ui/atm.hpp`, `ALife_fnc_atm`): balances for the faction you're playing, an amount with quick
buttons ($100 / $500 / $1,000 / $5,000), **Deposit**, **Withdraw**, **Deposit all cash**,
**Withdraw all**, and **Send** to an online player. Deposit/withdraw go through
`ALife_fnc_atmAction`, which checks you're really at an ATM, then the extension's `bank`
`deposit` / `withdraw` (cash and bank change in one transaction, logged in `bank_transactions`).

## Shops and items

- Items: `config/CfgItems.hpp` (just **Medkit** so far). Icons in `ui/items/`, drawn by
  `tools/phone_icons/make_items.py`. Saved per faction in
  `players.<faction>_items`.
- Shops: `config/CfgShops.hpp`. **Medical Supplies** (medics only) at the three hospital spawn
  markers: Medkit $250, paid by cash or bank. Press **Windows** there and pick the shop.
- Money: bank payments and transfers go through the extension's `bank` command (the
  `bank_transactions` ledger); cash uses the `*_cash` delta save. Nobody starts with money yet —
  staff can add some on the website (player profile → bank adjustment).

## Gathering, processing and selling (Windows key)

Set in `config\CfgResources.hpp`; items in `CfgItems`, prices in `CfgMarket`.

- **Gather**: press Windows inside a gathering area (needs a Pickaxe). The server times it, checks
  you're still there, and gives what fits your carry weight.
- **Process**: look at the worker at a processor and press Windows. The server puts a worker
  (`CfgProcessing >> npcClass`) at each `process_*` point marker, facing the marker's direction.
  Everything you carry for it is processed in one go. Free with the processing licence; without it, a
  fee per batch from cash.
- **Sell**: look at a Resource Trader NPC (`trader_resources_1` / `_2`, placed in Eden; CfgShops
  `npcs[]`) and press Windows. It buys raw and processed resources at the market's sell price, which
  drops as people sell and recovers over time.

Shop NPCs and workers stand still and can't be hurt. Markers in Eden (gathering: area markers --
ellipse or rectangle; processors: point markers):

| Resource | Gather | Process into | Processor |
|---|---|---|---|
| Oil | `mine_oil` | Refined Oil | `process_oil` |
| Copper | `mine_copper` | Copper Bar | `process_copper` |
| Iron | `mine_iron` | Iron Bar | `process_iron` |
| Salt | `mine_salt` | Refined Salt | `process_salt` |
| Sand | `mine_sand` | Glass | `process_glass` |
| Diamond | `mine_diamond` | Cut Diamond | `process_diamond` |
| Rock | `mine_rock` | Cement | `process_cement` |

More spots for one resource: add markers to its `zones[]` / `markers[]`; more traders: add units to
`npcs[]`.

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

Tracked as issues on the Delivery Board; the main gaps:

- Phone apps Gang and Keys; Perks.
- Drugs: processing at objects, drug dealers, cash-only licences (#208).
- Garage / impound lot to get vehicles back (#209); a licence shop for the non-processing licences (#210).
- Police: escort, put in vehicle, jail (#211, #16).
- Paychecks (staff add money on the website for now; withdraw it at an ATM).
- Medic whitelisting (all Medic slots are open).
- Saving vehicle trunk contents (#213); a server-side kick (#212).
