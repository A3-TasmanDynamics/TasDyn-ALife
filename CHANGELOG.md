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
- `src/website/`: the Go website scaffold from `docs/WEBSITE.md`, built out with real working
  functionality, not just routing stubs:
  - "Sign in with Steam" (OpenID 2.0, verified against Steam's own docs before implementing, not
    assumed) creates/resolves a `players` row; DB-backed sessions (`web_sessions`) so a ban or
    rank change can revoke a session immediately by deleting its row, unlike a JWT.
  - Admin Panel and Support Panel access resolved at login from
    `staff_ranks.default_admin_panel`/`default_support_panel` plus per-player
    `staff_permission_overrides` (`panel.admin`/`panel.support`) -- re-checked on every
    panel-gated request, not just at login.
  - Discord account linking, built both directions: OAuth2 "Connect Discord" from the website, and
    a `/link <code>` slash command from Discord itself (`discordgo` bot), redeeming a short-lived
    code the website generates (`discord_link_codes`). Either path sets the same
    `players.discord_id`. A linked Discord account also becomes a valid *additional* login method
    -- never the first one, since only Steam creates a `players` row.
  - Full support-ticket lifecycle (create, claim, reply, close/reopen) shared between the member
    portal's "My Tickets" and the Support Panel's queue -- one `support_tickets` row, two access
    levels. New tickets post to Discord via webhook.
  - Member dashboard (read-only faction stats, gang roster/balance) and an Admin Panel staff-log
    viewer; both explicitly flagged in-code and in `docs/ROADMAP.md` as read-only for now --
    write actions (bank transfers, gang management, bans, rank edits) are designed but not yet
    built.
  - Verified end-to-end against a real local Postgres instance: curl-driven checks of every
    login-gate redirect, 403 on missing panel access, and the full ticket lifecycle; then
    confirmed visually in a real browser (landing page, the login-required banner, panel
    rendering) and alongside `server_manager`/pgAdmin4 open at the same time. Schema additions
    applied for real to the local dev DB (not just a rolled-back dry run this time), with the two
    smoke-test player rows deleted afterward.
  - Additional schema: `players.discord_id`/`discord_username`, `discord_link_codes`.
- `src/website`: member-portal bank transfers (`internal/bank`) -- both between a player's own
  faction accounts and to another player by exact name, idempotency-tokened (a fresh token per
  rendered form, so a double-click or back-button resubmit can't double-spend) and row-locked
  (`SELECT ... FOR UPDATE`, fixed ascending-account-ID order to avoid deadlocking an opposite-
  direction concurrent transfer) against races. Unblocked by actually checking the open question
  `docs/WEBSITE.md` §5 flagged rather than leaving it flagged: `fn_save.sqf`'s field allowlist has
  no `*_bank` entries at all (only `*_cash` is saveable), and the C++ extension's own `db.cpp`
  confirms `bank_accounts` is only ever seeded and read, never written by `save` -- there is no
  live in-game write path this could race against. Fixed a real gap the investigation surfaced
  along the way: a player who signs up on the website before ever connecting in-game had no
  `bank_accounts` rows at all (only the C++ extension's first-load path seeded them) --
  `FindOrCreatePlayerBySteamUID` now seeds the same three rows the game does.
- `src/website`: CSRF protection (`internal/csrf`, double-submit cookie pattern) wired into every
  state-changing route before the money-moving feature above shipped, not after.
- Verified the whole set end-to-end against the real local dev DB: own-account transfer, player-
  to-player transfer, insufficient-funds rejection, unknown/ambiguous-recipient rejection, a
  resubmitted-token replay (confirmed no double-apply), and a rejected request missing its CSRF
  token -- ledger rows (signed amounts, `balance_after`, shared per-pair request tokens) checked
  by hand against what the code should have produced. Smoke-test player rows deleted afterward.
- `src/website`: member-portal gang management (`internal/gang`) -- invite, remove, and rank
  change (`member`/`officer`) for an existing gang's membership, gated to the gang's leader
  (`gangs.leader_player_id` -- the only permission tier that exists for this, since
  `gang_members.rank` has no CHECK constraint in the schema and so is organizational, not a grant).
  Every action logs to `gang_log` the same way the eventual in-game gang system will. Invite
  reuses a new shared `internal/playerlookup` package (exact case-insensitive name resolution,
  rejecting ambiguous/unknown names) factored out of the bank-transfer feature rather than
  duplicated a second time. Verified end-to-end: leader invite/promote/remove all working, a
  non-leader correctly blocked from managing a gang they don't lead, a duplicate invite (player
  already in a gang) and an ambiguous name both correctly rejected with specific messages, and
  `gang_log` rows checked by hand for all three action types. Smoke-test gang/player rows deleted
  afterward (in FK-safe order: the gang row before its former leader, since `leader_player_id` is
  `ON DELETE RESTRICT`).
- `src/server_manager`: fixed a real bug reported after actually trying to launch the server
  through the app -- `LaunchServer` wrote `server.cfg` naming a mission template, but nothing had
  ever copied a mission folder by that name into the Arma 3 Server install's `mpmissions/`, so
  Arma had nothing to load. `deploy.go` now deploys `src/ALife.Altis` there fresh on every launch
  (same thing `tools/test_local_server.ps1` already did for its own smoke test, now wired into the
  real launch path too), and best-effort deploys the built C++ extension DLL + runtime deps +
  `config.ini` (a missing extension warns rather than blocking launch, since the mission still
  loads without it). Caught the "warnings pushed as `server:log` events get wiped by the frontend's
  post-launch `clearConsole()`" race before shipping it -- `LaunchServer` now returns warnings
  directly instead. Verified against the real local Arma 3 Server install via a new gated
  integration test (`deploy_test.go`, `ALIFE_TEST_ARMA_PATH`), not just compiled -- confirmed the
  mission's files actually land in `mpmissions/ALife.Altis` and the extension's DLLs/config.ini
  actually land in the server root, then checked the real directory by hand.
- `src/server_manager`: rebranded the Dashboard tab into **Logs** -- a log-type selector (Staff
  Actions / Anti-Cheat Flags / Kicks), each backed by its own row-level Go query
  (`GetStaffLog`/`GetAntiCheatLog`/`GetKickLog`) rather than one aggregated "everything" call.
  Added a new **Performance** tab: live CPU%/memory line graphs for the running dedicated server
  process, sampled every 2s via `gopsutil` and pushed as `server:performance` events -- same
  push-based pattern the Console tab's log tailing already used. No DB table backs it on purpose
  (ephemeral process telemetry, not durable game state); history clears on stop/relaunch. Verified
  the `gopsutil` CPU%/memory sampling against a real child process in isolation before wiring it to
  the real server process.
- `src/server_manager`: Performance tab now also graphs network I/O and server FPS.
  - Network is system-wide (`gopsutil`'s `net.IOCounters`, all interfaces) rather than isolated to
    `arma3server_x64.exe` -- Windows has no reliable per-process network byte counter the way it
    does for CPU/memory short of ETW, labeled honestly as a proxy rather than presented as
    process-specific. Delta computed against the previous 2s sample; guarded against an
    interface-reset counter wraparound reading as a nonsensical spike (`uint64` subtraction
    doesn't panic on underflow, it silently wraps).
  - FPS comes from `diag_fps` -- Arma exposes this only to script running inside the sim, so
    `src/ALife.Altis/initServer.sqf` now logs `[ALife][FPS] <value>` via `diag_log` every 2s, and
    `server_manager`'s existing RPT tail (already reading every new line for the Console tab)
    greps for that tag and feeds it into the same `PerformanceSample` the CPU/memory/network
    fields ride in. Reads `-1` (rendered as "waiting...", not `0`) until the first line arrives,
    so "no data yet" is never confused with a genuine 0 FPS.
  - Both verified against the local Arma 3 Server install via `tools/test_local_server.ps1` --
    which surfaced a separate, pre-existing issue while doing so (see below), not caused by this
    change.
- **Found, not yet fixed**: `tools/test_local_server.ps1` (run to verify the FPS logging above)
  shows the dedicated server stalling immediately after "Initializing Steam server failed" --
  RPT logging stops dead at that point even after a 90-second wait, meaning `initServer.sqf` (and
  therefore the new FPS logging, and everything else mission-side) never actually executes in this
  local test environment. `mission.sqm` lists `A3_Characters_F`/`A3_Ui_F` as addon dependencies
  (auto-added by Eden for the placed playable units), and the RPT logs a
  "downloadable content that has been deleted" warning for `A3_Characters_F` moments before the
  stall -- a commonly-reported, often-harmless warning on dedicated servers for that specific
  classname, so it's not yet confirmed as the actual cause rather than a coincidence. Flagged for
  the user rather than guessed at further; not something to unilaterally change in a
  user-authored `mission.sqm` without discussing it first.
- Fixed a real login bug found while actually testing Steam sign-in end-to-end: `clientIP()`
  (`src/website/internal/auth/session.go`) hand-rolled stripping the port off `r.RemoteAddr` by
  finding the last colon, which breaks on IPv6 (`r.RemoteAddr` is `"[::1]:PORT"` for a local
  browser hitting `localhost` -- exactly the common case) by leaving the brackets in, which
  Postgres's `inet` column then rejects outright. Every local Steam login was failing with
  "Something went wrong signing you in." until this was fixed -- replaced with `net.SplitHostPort`,
  which handles IPv6 correctly. Verified against a real Steam login end-to-end after the fix.
- `src/website`: reworked the Support Panel into a proper IT-support-portal, not a bare table --
  priority (`support_tickets.priority`: `low`/`normal`/`high`/`urgent`, picked by the submitter
  when opening a ticket, re-triageable by staff afterward; the queue sorts by priority first, then
  age), queue stats (Open/Unassigned/Assigned to Me/Urgent counts), filters (status/priority/
  assignment via plain query-string GETs, no client-side filtering), Claim/Unassign, and
  staff-only **internal notes** (`support_ticket_messages.internal`) -- never shown to the
  requester, never mirrored to Discord, filtered out in the SQL query itself for a non-staff
  viewer rather than just hidden by the template, so the content never reaches the page as hidden
  markup a curious player could inspect. A raw POST from a player's own session can't set the
  internal flag either, since the handler re-derives "is this submitter staff" from the session,
  not from the submitted form.
  - Verified end-to-end with disposable test accounts against the real dev DB: priority-sorted
    queue ordering (urgent → high → normal → low), all three filters (status/priority/assigned),
    claim/unassign, priority re-triage, and internal-note visibility -- confirmed staff sees the
    note and the ticket's own owner gets zero matches for it even in the raw HTTP response, not
    just "the button to see it is missing" from their view.
  - Also caught (during testing, not by inspection) that the real dev DB already had a ticket from
    live use this session -- left untouched, only the disposable test accounts and their tickets
    were cleaned up afterward.
- `src/website`: split the Support Panel into a two-page structure with a persistent side nav --
  `/support` is now a **Dashboard** (stats bar + the 5 most recently opened tickets), and
  `/support/tickets` is the full filterable **Tickets** queue -- matching how real IT ticketing
  systems separate an at-a-glance overview from the working queue instead of cramming both into
  one page. The side nav (`partial_support_sidebar.html`) also follows a staff viewer into the
  ticket detail page, so workspace navigation stays visible while drilling into a specific ticket;
  a player viewing their own ticket never sees it at all.
  - `internal/render`'s template loader now also parses `partial_*.html` fragments alongside every
    page (previously it only ever parsed layout + one page at a time, since pages define a
    same-named "content" block that would collide if every page were parsed together) -- partials
    define their own uniquely-named block instead, so this is the first shared fragment reused
    across pages rather than duplicated in each one.
  - Verified end-to-end: both pages render with correct active-tab highlighting, the sidebar
    appears on the ticket page for a staff session and is confirmed absent (zero matches, not just
    "not visible") for a plain player session viewing their own ticket. Disposable test accounts
    cleaned up afterward; the real ticket already in the dev DB was untouched.
- Fixed a real bug reported right after the sidebar shipped: the sidebar looked completely absent
  in a real browser despite the server correctly rendering it (confirmed by fetching the exact
  same route directly and getting the full, correct HTML back). Root cause: `/static/style.css`
  had been updated across several PRs today, but nothing told the browser to re-fetch it --
  `http.FileServer` sets `ETag`/`Last-Modified` but no `Cache-Control`, so the browser was free to
  keep serving a CSS snapshot from hours earlier with no visible error, just a page that silently
  looked wrong. Fixed by setting `Cache-Control: no-cache` on `/static/*` -- the browser still
  caches the file, but must revalidate with the server on every request (a cheap 304 when nothing
  changed) instead of serving a stale copy on its own heuristic. This affects every future
  `style.css`/JS change too, not just this one.
- Also fixed, found while verifying the above: the ticket queue's "Requester" column showed
  blank for a real ticket -- `players.name` defaults to `''` and is only ever set by the game's own
  save path, so a website-first signup (Steam login before ever connecting in-game) has an
  honestly-empty name. `COALESCE(NULLIF(name, ''), 'Player #' || id)` now backstops this
  everywhere a player's display name is read for session/ticket display (`internal/auth/session.go`,
  both ticket queries in `internal/handlers/support.go`, `internal/handlers/tickets.go`'s ticket-detail
  query) -- the top nav's own name display had the identical gap.
- `src/website`: `.site-main` no longer caps content at 960px centered -- pages now use the full
  browser width (padding only, no `max-width`). Matters most for the Support Panel's sidebar+table
  layout and the ticket queue, which had noticeably wasted space on anything wider than a laptop
  screen.
