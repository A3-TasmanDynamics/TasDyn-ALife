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
