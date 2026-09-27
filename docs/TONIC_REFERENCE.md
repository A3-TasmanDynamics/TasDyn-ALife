# Tonic's Framework — Architecture Reference & Comparison

A full read-through of Tonic's AsYetUntitled/Framework 5.0.0 (`C:\Users\Andrew\Documents\Framework-5.0.0`),
a real, currently-deployed Altis Life base, done specifically to check TasDyn-ALife's own decisions
against a working reference rather than guessing. Every section below states what Tonic actually
does, then how that lines up with [ROADMAP.md](ROADMAP.md) / [DATA_CONTRACT.md](DATA_CONTRACT.md) /
the current `src/ALife.Altis` code — marked **ALIGNED**, **DELIBERATE DIVERGENCE**, or **GAP**.

## 1. Directory layout

Tonic splits into three separate trees: the mission (`Altis_Life.Altis/`, client + shared code,
dialogs, configs) and **two separate addon-style PBOs** — `life_server/` (all server-authoritative
DB/game-state code, its own `config.cpp`/`PboPrefix.txt`, loaded via absolute `\life_server\...`
paths) and `life_hc/` (a mirror of the DB functions for a Headless Client). The split exists for a
real reason: the mission PBO is what every connecting client downloads, so keeping the actual SQL
query text out of it means a client can never read it, even with a PBO unpacker.

**GAP (minor, not urgent):** TasDyn-ALife's server-only logic (`fn_load.sqf`, `fn_save.sqf`,
`fn_playerJoin.sqf`, `fn_sync.sqf`) ships inside `src/ALife.Altis` itself, gated by a runtime
`isServer` check rather than being physically excluded from the client download. This is lower-risk
for us than for Tonic specifically because the real command construction lives in the compiled C++
extension, not in SQF — the mission PBO never contains SQL, only command names like `"load"`/`"save"`.
Worth a note, not worth restructuring into a separate addon PBO before launch.

## 2. Player join flow

**Tonic's exact chain**, confirmed by reading it end to end:
`initPlayerLocal.sqf` → `core/init.sqf` (client) → `SOCK_fnc_dataQuery` (remoteExec's to server,
target `RSERV`=2) → server's `DB_fnc_queryRequest` runs the `SELECT` → remoteExec's the result back
to the requesting client's owner ID → client's `SOCK_fnc_requestReceived` unpacks it into globals →
per-side init function (`life_fnc_initCiv`/`initCop`/`initMedic`) → **if the player's saved state was
alive, silently reposition them (`setVehiclePosition`, no dialog); only if dead/first-join does the
spawn dialog appear.**

**Tonic has no `initPlayerServer.sqf` at all** — confirmed it doesn't exist anywhere in the
framework. Every "player joined, load them" step is client-initiated via remoteExec to the server,
never server-pushed.

**ALIGNED:** this is exactly the shape `initPlayerLocal.sqf` → `fn_playerJoin.sqf` (server, via
`remoteExec` target 2) → `ALife_fnc_load` → spawn menu already takes in TasDyn-ALife, and it's the
same shape for the same reason — `initPlayerServer.sqf` was proven this session to never fire for a
real dedicated-server connection, and Tonic's own real, working framework independently confirms the
fix (client-initiated join) is the actual correct pattern, not a workaround.

**GAP:** Tonic skips the spawn dialog entirely for an already-alive reconnect — straight reposition,
no menu. `fn_playerJoin.sqf` currently always does `["open"] remoteExec ["ALife_fnc_spawnMenu", _player]`
regardless of saved alive-state. [ROADMAP.md Phase 2](ROADMAP.md#phase-2--core-gameplay-loop-2026-10-27--2026-11-23)
already scopes `ALife_fnc_spawnPlayer` to ignore a spawn *request* when the player should resume at
`<faction>_position` instead — but that's the **spawn button's** target selection, not "should the
menu appear at all." Concretely: `fn_playerJoin.sqf` should branch on the loaded record's
`<faction>_alive` the same way Tonic's `fn_initCiv.sqf` does, and skip straight to positioning +
`ALife_fnc_spawnPlayer`-equivalent logic instead of opening the dialog, when the player was already
alive at disconnect.

## 3. Faction/side selection — the actual root cause of this session's role-conflict bug

This is the one worth reading carefully, because it explains a bug already chased and "fixed" earlier
this session without fully understanding why it happened.

**Tonic's model:** faction (west/civilian/independent) is chosen via **Arma's own native multiplayer
role-selection screen**, backed by *multiple Editor-placed playable slots per side*. Their custom
dialog (`life_spawn_selection`) never touches faction at all — it only picks a **spawn location**
within the side the engine already assigned. Cop/medic slots additionally get an in-script whitelist
kick (`life_coplevel==0 && life_adminlevel==0` → `BIS_fnc_endMission`) *after* the engine already let
them into that slot, since the engine's role screen has no concept of a DB-backed whitelist.

**TasDyn-ALife's plan matches Tonic's exactly: engine-native role selection (west/civilian/
independent), not a custom in-dialog faction picker.** The dialog's `SideCivilian`/`SideCop`/
`SideMedic` buttons (`spawnMenu.hpp`) and the single-generic-slot `mission.sqm` fix from earlier this
session were a wrong turn, not the intended design — corrected below, not just documented as a
"valid alternative."

**This is the real root cause of the "16 roles" RPT warning and the no-spawn-menu bug**: the original
`mission.sqm` already had the *correct* topology for the intended design — 16 pre-placed per-faction
playable roles (`police_1-4`/`civilian_1-4`/`medic_1-8`, one group per real Arma side) — but the
SQF/dialog code was accidentally built for a *different* pattern (one generic slot, side chosen
in-dialog), which nothing had actually decided on. Having the mission built for one pattern and the
code built for the other is what broke role selection, not "too many slots," and not a defect in the
per-side-slots topology itself.

**Fix applied:** `mission.sqm`'s 16 roles are back to `isPlayable=1` (police=west, civilian, medic=
independent — matching Tonic's own side mapping), `spawnMenu.hpp`'s side buttons are removed, and a
new `ALife_fnc_sideToFaction` (`core/functions/spawn/fn_sideToFaction.sqf`) maps `side player`/`side _unit`
to `"civ"`/`"cop"`/`"medic"` — the one place that mapping lives, used both client-side
(`fn_spawnMenu.sqf`'s `onLoad`, to populate the right spawn-point list) and, critically,
**server-side** in `fn_spawnPlayer.sqf`, which now derives faction from the unit's actual `side`
rather than trusting whatever a client claims. That closes a real gap the old design had: a client
could previously `remoteExec` `ALife_fnc_spawnPlayer` directly with an arbitrary faction string,
since the only server-side check was "is this one of the three known strings," not "does this match
who you actually are." The dialog itself (`spawnMenu.hpp`) is now spawn-point-only, matching Tonic's
`life_spawn_selection` shape.

**Still open, matching Tonic's own pattern:** cop/medic whitelisting. Tonic lets the engine's role
screen put anyone into a west/independent slot, then kicks non-whitelisted players via a post-spawn
in-script check (`life_coplevel==0 && life_adminlevel==0` → `BIS_fnc_endMission`) rather than
restricting the slot itself. TasDyn-ALife doesn't have this check yet — anyone can currently pick a
Police/Medic slot in the role screen with no whitelist gate at all. Worth scoping into
[ANTI_CHEAT.md](ANTI_CHEAT.md)/[ROADMAP.md Phase 3](ROADMAP.md#phase-3--anti-cheat--admin-tooling-2026-11-24--2026-12-29-revised--was-2026-11-24--2026-12-04)
rather than left implicit.

## 4. Dialog base classes — direct validation of this session's revert

Tonic's `dialog/common.hpp` defines every base control class (`Life_RscButton`, `Life_RscListBox`,
`Life_RscStructuredText`, `Life_RscMapControl`, etc.) as **fully self-contained**, explicit `type = N`
constants, full hand-written property sets. **There is not a single `class RscText;`-style
forward-declare-and-inherit anywhere in the framework.** This is a real, live, currently-deployed
mission — proof the self-contained pattern works correctly in production.

**ALIGNED, with a bug already caught and fixed this session as a direct result of this finding:**
common_ui.hpp was briefly switched to forward-declare + inherit from the engine's real `Rsc*` classes,
on the theory it would provide every property automatically. Live RPT proved that wrong in this
project's compile context (every control lost its `type` entirely). Tonic's own working code
independently confirms the self-contained approach — already reverted back to in
[PR #100](https://github.com/A3-TasmanDynamics/TasDyn-ALife/pull/100) — was correct, not a
workaround to abandon later.

## 5. Save/load architecture

**Tonic:** no periodic full-autosave loop anywhere (confirmed via search — zero hits for
"autosave"). Persistence is 100% event-driven: `SOCK_fnc_updatePartial`, a narrow single-field save
keyed by an integer mode, fires immediately after ~40 different gameplay actions across
shops/housing/gangs/medical/cop. A full save (`SOCK_fnc_updateRequest`) only fires from a few explicit
points (manual `/sync` command, revive, respawn, the ESC-menu Abort button). Server-side
`HandleDisconnect` only saves civilian position/alive-state, not cash/gear/licenses — the assumption
being those were already persisted via the per-transaction partial saves.

**DELIBERATE DIVERGENCE:** TasDyn-ALife's `fn_sync.sqf` runs a 60s interval loop that autosaves
*every* connected player unconditionally, in addition to per-field saves already happening via
`ALife_fnc_save`/`fn_savePlayerState.sqf` on disconnect. This is a genuine extra safety net Tonic
doesn't have (guards against a save silently never happening between transactions), at the cost of
periodic write load scaling with player count regardless of activity. Not a bug — just worth being
aware it's a deliberate addition beyond what the reference framework does, worth revisiting if it
ever shows up as a DB load concern at higher player counts.

**ALIGNED, already better than Tonic in one specific way:** [DATA_CONTRACT.md](DATA_CONTRACT.md)
explicitly chose key-value pairs over positional arrays specifically to avoid field-order drift —
Tonic's entire wire format (DB rows, remoteExec payloads, `_this select N` unpacking) is flat
positional arrays end to end, which the framework's own code shows is fragile (e.g. licenses/gear
stored as stringified-SQF-literal TEXT columns, decoded via `call compile format [...]`). TasDyn's
contract doc already identified and avoided this exact failure mode before ever hitting it.

**Notable mechanism worth remembering for later (routing async DB results):** Tonic routes an async
DB response back to exactly the right client via `_ownerID = owner _playerObject` then
`remoteExec [..., _ownerID]` — the load-bearing trick that makes "server does an async call, then
gets the result back to the one client who asked" work in plain SQF. Relevant once TasDyn-ALife's C++
extension needs to push a result back mid-session rather than always being called synchronously
request/response style (e.g. Phase 3's anti-cheat flag alerts).

## 6. Faction / spawn point config

Tonic's `CfgSpawnPoints` nests `<Map> > <Side> > <NamedPoint>`, each point carrying a raw SQF
`conditions` string (e.g. `"call life_coplevel >= 3"`) evaluated at runtime via `call compile` —
fully data-driven gating on rank/licenses without per-point code, but a live `call compile` on
config-authored text.

**ALIGNED in spirit:** `config/spawn_config.hpp` + `ALife_fnc_getSpawnPoints` already does
config-driven, per-faction spawn point filtering per [ROADMAP.md Phase 2](ROADMAP.md#phase-2--core-gameplay-loop-2026-10-27--2026-11-23).
Worth checking `fn_getSpawnPoints.sqf` doesn't lean on a raw `call compile` of config text the way
Tonic does — a simple side/rank comparison is safer than an arbitrary-string eval, and there's no
sign yet that TasDyn needs the flexibility that justifies the risk Tonic accepted there.

## 7. Anti-cheat — one pattern worth adopting, not yet in ANTI_CHEAT.md

Two mechanisms in Tonic aren't currently mentioned in [ANTI_CHEAT.md](ANTI_CHEAT.md) and are worth
considering additions rather than a gap in what's already planned:

- **`CONST`/`CONSTVAR` macros** (`compileFinal(str(value))`) wrap every sensitive server-delivered
  value (`life_adminlevel`, `life_coplevel`, `life_donorlevel`) so they're immutable compiled code
  objects instead of plain reassignable globals — a client script can't just do
  `life_adminlevel = 999`, it has to be read via a call. Cheap, load-bearing, and directly applicable
  to however TasDyn-ALife delivers rank/admin-level to the client after `ALife_fnc_load`.
- **Debug console usage treated as a cheat signal.** `SpyGlass/fn_variableCheck.sqf` checks
  `parsingNamespace` is empty except for the two known debug-console result variables — i.e. a player
  opening Arma's built-in debug console at all triggers a kick. Simple, and something ANTI_CHEAT.md's
  honeypot-variable layer doesn't currently call out.

Also confirmed: `CfgRemoteExec.hpp`'s `mode = 1` whitelist-only pattern with explicit
`allowedTargets` per function is exactly what TasDyn-ALife's own `CfgRemoteExec.hpp` already does
(the roadmap already cites Tonic's file as the reference for this) — no gap here, just confirmation.

## 8. Other engine-gotcha-driven decisions worth knowing about

- **No engine respawn system used at all** (`description.ext` has zero `respawn=`). Death is fully
  custom: a `Killed` event handler + spectator cam + dialog, then the *same* unit object is
  repositioned (`setPos`/`setVehiclePosition`), never destroyed and recreated. Sidesteps Arma's
  respawn-template gear-reset quirks entirely. Worth confirming TasDyn-ALife's eventual death/revive
  flow (not yet built — Phase 2) follows the same "reposition, don't recreate" approach.
- **Serial, blocking gear application**: Tonic's `fn_loadGear.sqf` applies each clothing/inventory
  slot one at a time via `spawn` + `waitUntil {scriptDone _handle}` — a real, confirmed gotcha
  (Arma's uniform/vest/backpack functions aren't safely fire-and-forget in a tight loop). Relevant
  once gear/loadout restoration is actually wired up (currently explicitly deferred, per
  `src/ALife.Altis/README.md`).
- **`unsafeCVL = 1;`** in `description.ext` (enables `createVehicleLocal` in MP, normally
  restricted) — only relevant if TasDyn-ALife ends up needing local-only vehicle effects.

## Summary — concrete follow-ups

1. **Redeploy and retest now** — the "black screen, no spawn menu, no errors" symptom just reported
   matches exactly what the (already-fixed, already-pushed) forward-declare regression in PR #100
   would produce; likely already resolved.
2. **Done (§3):** `mission.sqm` restored to 16 per-side playable slots (west/civilian/independent),
   `spawnMenu.hpp`'s side-selection buttons removed, and a new `ALife_fnc_sideToFaction` makes
   `fn_spawnPlayer.sqf` derive faction from `side _unit` server-side instead of trusting a
   client-supplied value.
3. `fn_playerJoin.sqf`: branch on the loaded record's `<faction>_alive` and skip the spawn dialog
   entirely (silent reposition) for an already-alive reconnect, matching Tonic's `fn_initCiv.sqf`
   (§2).
4. Consider a `CONST`/`compileFinal`-style wrapper for `life_adminlevel`-equivalent values once
   rank/admin-level start reaching the client (§7) — flag for Phase 3's anti-cheat work.
5. Confirmed: `fn_getSpawnPoints.sqf` doesn't use a raw `call compile` on config text (§6) — no
   action needed.
6. **New, from §3:** cop/medic whitelisting doesn't exist yet — anyone can currently pick a
   Police/Medic slot in the engine's role screen. Tonic gates this with a post-spawn in-script kick,
   not a slot restriction; scope the equivalent into Phase 3.
