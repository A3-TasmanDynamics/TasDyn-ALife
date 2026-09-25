# Changelog

## [Unreleased]
### Added
- Initial project scaffold: README, directory structure, `.gitignore`, `config.ini.example`.
- Org-standard branding and repo structure: banner (`docs/assets/banner.svg`), `CONTRIBUTING.md`,
  and delivery-board sync workflow (`.github/workflows/sync-to-project.yml`), matching the other
  Tasman Dynamics repos.
- `docs/ROADMAP.md`: phased plan and timeline toward a public launch target, tracked alongside
  GitHub issues/milestones on the Delivery Board.
- `docs/ANTI_CHEAT.md`: full threat model and layered defense design (BattlEye, `CfgRemoteExec`
  allowlist, economic integrity, movement/behavior heuristics, graduated response) — expands the
  README's anti-cheat section and Phase 1/3 of the roadmap beyond the original two heuristics.
- `docs/ADMIN_TOOLS.md`: in-game staff menu spec, expanded to feature parity with infiSTAR and
  Fini Anti-Hack & Admin Tools plus arsenal editing (per-faction item pools, loadout presets),
  backed by a DB-driven `staff_ranks` table + per-command permission overrides, a debug console,
  and in-menu logging/metrics — extends Phase 0's schema and Phase 3's scope substantially.
- Roadmap revised: public launch target moved from 2026-12-26 to **2027-01-20** (~4 added weeks,
  entirely inside Phase 3's window) once the admin-tooling scope above was made launch-critical
  rather than deferred — flagged explicitly in ROADMAP.md as an estimate pending confirmation.
- `docs/arma/`: an offline, indexed SQF command reference — every command used in this project
  gets verified against it (syntax, params, return type, `since` version) before it's written.
- **Phase 0 complete**: `docs/DATA_CONTRACT.md` (the save/load field contract) and
  `database/schema.sql` (23 tables — players/banking/staff-admin core, vehicles/houses/gangs,
  anti-cheat/whitelist backing tables). Reviewed against Tonic's AsYetUntitled/Framework Altis
  Life base before finalizing — see below.
- **Phase 1 started**: the C++ extension skeleton (`src/cpp_extension/`) — `RVExtension`/
  `RVExtensionArgs` exports, a `libpq` connection (config loaded from `config.ini`), and a `ping`
  command proving the full chain (Arma-equivalent call → DLL → libpq → Postgres) actually works.
  Verified with a native test harness (`test_harness.cpp`) rather than assumed.
- Bank balances (`bank_accounts.balance`) and civilian bounty (`wanted_crimes`, summed) are each
  authoritative sources with a DB-trigger-synced cache column on `players` — never two
  independently-updated numbers for the same value.
- Persisted per-faction alive/death state and last position on `players` — without this, a player
  who disconnects while dead/unconscious respawns fresh instead of resuming dead, a known
  disconnect-to-escape exploit in this genre (confirmed via Tonic's framework, which has the same
  fields for the same reason).
- `player_aliases` (name history, for spotting alt-account/evasion patterns) and `wanted_crimes`
  (Police faction wanted list/bounty ledger) — both gaps found by reviewing Tonic's schema.
- Vehicle damage stored as a per-hitpoint JSONB map (matching `getAllHitPointsDamage`), not a
  single float — a single-number damage column can't actually represent Arma's damage model.
- **`cmd_load`/`cmd_save` implemented and verified end-to-end** against a real local Postgres
  instance (blank-record creation, field persistence, idempotency-token replay rejection, and the
  cash overspend/negative-balance guard all tested via the native harness, not just written).
- JSONB shape rule established and applied schema-wide: any JSONB column crossing the
  `callExtension` boundary (`gear`, `position`, vehicle `damage`/`inventory`, house `storage`) is
  an array of `[key, value]` pairs, never a bare JSON object — `parseSimpleArray`'s grammar has no
  object literal, so Postgres's raw JSONB text is now directly valid SQF input with zero
  conversion code needed in the extension.
- `civ_cash`/`cop_cash`/`medic_cash` treated as a signed delta with its own idempotency ledger
  (`applied_request_tokens`), not an absolute-set field — an absolute `save` on physical cash had
  the same concurrent-double-apply risk `bank_accounts` was split out to avoid; caught before any
  save code was written against the original design.
- `src/ALife.Altis/` scaffolded: `description.ext`, `CfgFunctions.hpp`, `CfgRemoteExec.hpp`
  (reviewed against Tonic's own `CfgRemoteExec.hpp` for the real-world pattern before writing
  ours), `initPlayerServer.sqf`/`initServer.sqf` join and disconnect hooks, and
  `ALife_fnc_load`/`ALife_fnc_save` implementing `docs/DATA_CONTRACT.md` exactly. `mission.sqm`
  itself isn't generated here — it's Eden editor output — see `src/ALife.Altis/README.md` for how to
  wire this scaffold into an actual mission.
- Faction spawn/selection (Phase 2 work, done ahead of schedule alongside the mission scaffold):
  `config/spawn_config.hpp` defines spawn points config-side (git-diffable), each resolving its
  position from an Eden marker or a raw fallback; `dialog/spawnMenu.hpp` + three client functions
  drive the faction/spawn-point picker; `ALife_fnc_spawnPlayer` is server-authoritative — a
  player's spawn-point choice is a request, not a fact, and is overridden entirely (resume at
  `<faction>_position`, ignore the request) when their stored `<faction>_alive` is `false`. This
  is the actual enforcement of the disconnect-to-escape protection reviewed in from Tonic's schema
  during Phase 1 — the columns existed before, now something reads and acts on them.
- Reviewed Tonic's spawn-selection dialog and mission folder layout for structural ideas (a
  list-based spawn-point picker, config-driven definitions) — not used directly; built fresh
  against this project's own schema/contract.
- `src/server_manager/`: a Go + Wails desktop app for actually running the dedicated server --
  configure name/passwords/slots, launch/stop `arma3server_x64.exe`, watch its live log, and view
  Postgres-backed graphs (players, economy, anti-cheat flags, staff actions). Distinct from
  `tools/test_local_server.ps1` (a one-shot smoke test) -- this is for a real host, day to day.
  Verified the dashboard SQL against a real local Postgres instance via an integration test
  (`dashboard_test.go`), not just compiled. Caught and fixed a real gap while testing the built
  app: every panel's Go call is now wrapped in try/catch with a visible error banner on failure --
  previously a rejected promise failed silently, leaving a panel stuck on "Loading..." forever
  with no indication anything had gone wrong.
- `src/server_manager/` UI revamp: sidebar shell with icon nav and a persistent live server-status
  pill (visible on every tab, not just Launch), plus a glassmorphism treatment across cards, stat
  tiles, and inputs -- kept the existing navy/amber palette.
- `docs/WEBSITE.md`: full design for a Go website -- public landing page, a member portal (Steam
  login, faction stats, gang management, bank transfers), and two separately-gated staff surfaces,
  an Admin Panel and a Support Panel, switchable without re-authenticating for staff holding both.
  Deliberately not a parallel identity/money system: logins resolve to the existing `players` row
  (`uid` = Steam64 ID) and member-portal writes (transfers, gang management) go through the same
  `bank_accounts`/`bank_transactions`/`gangs`/`gang_members` tables the game already treats as
  authoritative. Support tickets sync two-way with Discord via a bot (`discordgo`); staff actions,
  bans, and high-confidence anti-cheat flags post to Discord one-way via webhook.
- Schema additions backing the website: `web_sessions` (DB-backed, revocable sessions -- not JWT,
  so a ban can log a session out immediately), `support_tickets`, `support_ticket_messages`, and
  `staff_ranks.default_admin_panel`/`default_support_panel` (Admin and Support are separate grants,
  not implied by rank level alone). Applied against the real local dev DB in a rolled-back
  transaction to verify syntax before committing.
- Roadmap revised again: public launch target moved from 2027-01-20 to **2027-03-03** (~6 added
  weeks, a new Phase W between Phase 3 and Phase 4) once the website above was made launch-critical
  rather than post-launch fast-follow -- same pattern as the admin-tooling revision, flagged
  explicitly in ROADMAP.md as an estimate pending confirmation. README's Technical Stack and
  project-structure sections updated to match (the old "NuxtJS + Node.js bot, planned" placeholder
  is replaced by the actual Go decision).
