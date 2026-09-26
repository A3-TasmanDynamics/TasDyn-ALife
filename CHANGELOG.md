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
- **Follow-up, resolved**: re-investigated the "dedicated server stalls after Initializing Steam
  server" finding above. Two real things came out of it:
  - `description.ext` was missing a `class Header { ... }` block, which the engine logged as
    "Missing 'description.ext::Header'" -- a real (if minor) config gap. Added the minimum the
    engine expects (`gameType`, `minPlayers`, `maxPlayers`); gameplay-specific Header settings
    stay Phase 2 territory like the rest of this file.
  - The "stall" itself was a false alarm caused by how the test was being read, not a real hang:
    `tools/test_local_server.ps1` kills the server with `Stop-Process -Force` (`TerminateProcess`,
    no flush) after a fixed wait and then reads the RPT file from disk -- but confirmed via a live
    (not-yet-killed) test process that the server was fully up the whole time: it answered Steam
    A2S_INFO queries correctly (right hostname, region, BattlEye state) reporting the normal
    multiplayer-lobby "Waiting" status, its threads were parked in ordinary idle-wait states (no
    deadlock signature), and it was still answering queries after ~9 minutes of RPT silence. RPT
    writes are evidently buffered heavily enough that a forcibly-killed process's log can look
    frozen at a mid-init line when the server was actually fine -- the `A3_Characters_F`/
    `A3_Ui_F` warning was confirmed harmless (present in both the old and new runs, well before
    the apparent "stall" point, with no other effect). No mission-side fix needed beyond the
    Header addition above; `mission.sqm` was not touched.
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
- `src/website`: reworked the ticket UI toward a fully-featured ticketing system per explicit
  request -- requester identity, richer filtering, and a proper two-column ticket detail layout.
  - **Requester identity everywhere**: the queue, dashboard, and ticket detail page now show the
    requester's Steam64 UID next to their name; the ticket detail page additionally shows their
    linked Discord (username + ID, or "Not linked") in a dedicated Requester panel.
  - **Queue filters**: added category and a search box (subject / player name / Steam UID) to the
    existing status/priority/assigned filters -- the search uses one bound parameter reused across
    all three `ILIKE` clauses, not three separately-trusted inputs.
  - **Ticket detail page redesigned** into the two-column layout real ticketing systems (Zendesk,
    Freshdesk, Jira Service Desk) use: conversation + reply on the left, a metadata sidebar on the
    right (status/priority/category/assigned/timestamps, the requester identity block, and the
    claim/unassign/close/reopen/priority actions) -- ticket properties stay visible without
    scrolling away from the conversation. A player viewing their own ticket still gets the
    conversation only, no metadata panel.
  - Verified end-to-end against the real dev DB: search matches on subject and on a Steam UID
    fragment, a no-match search correctly shows the empty state, and the metadata/Requester panel
    renders the real ticket's actual Steam UID. Disposable test account cleaned up afterward.
- `database/schema.sql`: replaced `support_tickets.category` (a flat, free-text field) with a real
  category/sub-category taxonomy per explicit request -- a new self-referential `ticket_categories`
  table (`parent_id NULL` = top-level, set = a subcategory) rather than a hardcoded list, same
  DB-driven reasoning as `staff_ranks`/`arsenal_item_pools`. Seeded with Gameplay, Discord, Panel,
  TeamSpeak, and Other as top-level categories; Gameplay/Discord/Panel/TeamSpeak each get several
  subcategories (Gameplay: Bug Report, Player Report, Ban Appeal, Whitelist Application, Economy
  Issue, Vehicle/Property Issue; similar breakdowns for the other three). `support_tickets` now has
  `category_id`/`subcategory_id` FKs instead of the old text column -- `category_id` must be a
  top-level row and `subcategory_id`, if set, must be its child, both re-checked server-side on
  every ticket creation (`internal/handlers/categories.go`'s `validateCategoryPair`) rather than
  trusted from whatever the form's own `<select>` options happened to be.
  - Migrated the one real ticket already in the dev DB (verified in a rolled-back dry run first,
    then applied for real): its old free-text `category = 'other'` mapped correctly to the new
    `Other` top-level category with no subcategory.
  - The new-ticket form's category/sub-category selects cascade via a small inline script (no
    framework, no page reload) -- picking a top-level category filters which subcategories show.
  - Verified end-to-end: a valid category+subcategory pair creates correctly and displays
    correctly everywhere (My Tickets, the queue, the dashboard, the ticket detail panel); a
    subcategory-used-as-a-top-level-category submission is correctly rejected; a subcategory that
    doesn't belong to the chosen category is correctly rejected; the category filter on the queue
    correctly includes/excludes tickets by top-level category. Disposable test accounts cleaned up
    afterward.
- `src/website`: made ticket data directly editable from the detail page, NinjaOne-style (an
  inline-editable properties panel, not a separate edit mode) -- per explicit request.
  - **Subject** and **Category/Sub-category** are now editable (text field + Save; the same
    cascading selects as ticket creation, pre-filled and re-validated through the same
    `validateCategoryPair`).
  - Replaced the separate Claim/Unassign buttons with a single **Assign to** dropdown listing
    every currently Support-Panel-eligible player (resolved fresh via the same rank+override logic
    `internal/auth/session.go` uses at login, not cached) -- covers claiming, reassigning to
    someone else, and releasing back to the queue in one control. Assigning bumps `open` ->
    `pending`; reassigning an already-`pending` ticket leaves its status alone.
  - Every new endpoint re-validates server-side rather than trusting the form: an assignee must
    currently resolve as support-eligible or the request is rejected with a specific error, not
    silently accepted.
  - Caught and fixed a real routing bug before it shipped: the new Subject/Category edit forms
    initially posted to `/tickets/{id}/subject`\`/category` (404 -- those routes were registered
    under `/support/tickets/{id}/...`, matching every other staff-only ticket action). Found by
    actually submitting the forms and checking the real HTTP response, not by inspection.
  - Verified end-to-end against the real dev DB: subject edit, category+subcategory edit, assign,
    reassign to a second staff member (status correctly stays `pending`, not reset to `open`),
    unassign, and a rejected assignment attempt to a player without current Support Panel access.
    Disposable test accounts (including two real-rank test staff, to exercise reassignment
    properly) cleaned up afterward.
- `src/website`: gated the ticket detail page's editable properties (Subject, Priority,
  Assigned to, Category/Sub-category) behind an explicit **Edit** button instead of leaving them
  live at all times -- per explicit request, so a staff member just reading a ticket can't change
  its data with a stray click or keystroke.
  - A read-only view (plain text/badges) now shows by default; clicking **Edit** swaps in the same
    forms/selects that already existed, unchanged; **Done editing** swaps back. Pure client-side
    visibility toggle (`hidden` attribute) -- no new backend routes, no change to how any of the
    four existing save endpoints validate or persist.
  - Status stayed as one-click Close/Reopen actions rather than being pulled behind the edit
    gate -- treated as a workflow transition, not a field edit.
  - Verified end-to-end against the real dev DB: the properties card renders in read-only mode by
    default (no inputs/selects present in the initial HTML), and the edit form markup is present
    but marked `hidden` until toggled. Disposable test session cleaned up afterward.
- `src/website`: fixed a real bug in the edit-mode toggle above, caught from a screenshot showing
  the read-only view and the edit form both visible at once after clicking Edit. Root cause: the
  browser's default `[hidden] { display: none }` rule is *author*-origin-losing against any of our
  own CSS with a competing `display` (e.g. `.meta-list { display: grid }`), since author rules
  always outrank user-agent rules regardless of matching specificity -- so setting `.hidden = true`
  on the properties `<dl>` silently did nothing. Added a global `[hidden] { display: none
  !important; }` rule so `hidden` reliably hides an element site-wide, no matter what else targets
  it.
  - Also consolidated the two separate Edit/"Done editing" controls into one header button that
    swaps its own label, and gave it a compact button style instead of a plain text link.
  - While investigating, found and fixed a second real bug the same screenshot exposed: the ticket
    detail page's "Assigned to" read-out used bare `assignee.name` (no blank-name fallback), so a
    support staff member who signed up via the website first (empty `players.name`, the same gap
    fixed elsewhere for requester names) showed as "Unassigned" everywhere -- ticket detail,
    dashboard, and queue -- despite genuinely being assigned. Applied the same
    `COALESCE(NULLIF(name, ''), 'Player #' || id)` pattern used for requester names, with an outer
    `COALESCE(..., '')` fallback preserved for tickets with no assignee at all.
- `src/ALife.Altis`: fixed two real bugs reproduced directly in Eden Editor preview.
  - `dialog/spawnMenu.hpp`'s controls inherited from `RscText`/`RscButton`/`RscListbox` with no
    forward declaration -- "Undefined base class 'RscText'". Standard Arma dialog requirement:
    the mission config compiler needs these declared (`class RscText;`) before use as a base
    class, even though the real classes exist in the engine at runtime.
  - `initPlayerServer.sqf` called `ALife_fnc_load` before the `CfgFunctions` library was
    guaranteed to have finished compiling -- "Undefined variable: alife_fnc_load". A real
    dedicated server's lobby wait hides this; a fast-starting session (Eden preview,
    locally-hosted MP) doesn't guarantee that gap. Added a `waitUntil { !isNil "..." }` guard.
- `src/ALife.Altis`: fixed civilian/police death-position persistence being silently dropped.
  This mission uses `"civilian"`/`"police"`/`"medic"` as its faction identifier everywhere (spawn
  menu, `config/spawn_config.hpp`, matching `bank_accounts.faction`'s `CHECK` constraint) but
  `docs/DATA_CONTRACT.md`'s `players.<faction>_alive` / `players.<faction>_position` fields use
  the abbreviated `civ`/`cop` prefix -- two different, both-intentional conventions that
  `fn_spawnPlayer.sqf` and `initServer.sqf`'s `HandleDisconnect` were conflating by building the
  save/load field name directly from the faction identifier. `civilian_alive`/`police_position`
  never matched any real `players` column, so `ALife_fnc_save` silently returned `"ERROR"` for
  every civilian/police death-position save (`HandleDisconnect` never checks the return value) --
  those two factions never actually got their alive/position state persisted on disconnect; only
  medic worked, since it's spelled the same both ways. Added `functions/data/fn_factionDbPrefix.sqf`
  as the one place that translates between the two conventions.
- `src/ALife.Altis`: placed the first real mission content in Eden -- `police_kavala_spawn`,
  `civilian_kavala_spawn`, `medic_kavala_spawn` marker objects, and 16 placeholder playable units
  (`police_1..4`, `civilian_1..4`, `medic_1..8`; real per-faction models/uniforms are Phase 2
  work). `config/spawn_config.hpp`'s marker names updated to match.
- **Found via Eden preview, not a code bug**: `callExtension` resolves the extension DLL from the
  *running executable's own directory* -- for `arma3server_x64.exe` that's the dedicated server
  install (what `tools/test_local_server.ps1` / `server_manager`'s `deploy.go` already deploy the
  extension into), but Eden Editor's "Play Scenario" preview runs `arma3_x64.exe` from the
  regular **Arma 3 client** install -- a separate directory the extension had never been deployed
  to. Missing this doesn't produce an obvious error; `callExtension` silently returns nothing
  usable, which cascades into confusing SQF-side errors (`ALife_fnc_load`'s `_record` coming back
  undefined, then garbage propagating into later checks) that look like mission-code bugs.
  Confirmed via `test_harness.exe` run from both directories -- fails from the client install
  without the extension present, succeeds identically once deployed there too. Documented in
  `src/cpp_extension/README.md` under "Deploying for Eden Editor / local preview testing" so this
  doesn't get rediscovered the hard way again.
- `src/ALife.Altis`: rebuilt the spawn system after the "Invalid number in expression" crash kept
  recurring at the same line through two narrower fixes. Reviewed a real framework's spawn dialog
  and an old local prototype's `core/spawn/` structure for reference (not copied wholesale):
  - **Faction naming unified to `"civ"`/`"cop"`/`"medic"`** everywhere in this mission (spawn menu,
    `config/spawn_config.hpp`, `fn_spawnPlayer.sqf`, `initServer.sqf`'s `HandleDisconnect`) --
    matching `docs/DATA_CONTRACT.md`'s `players.<faction>_*` field prefix directly, so a faction
    string can be concatenated straight into a field name with no translation step. Removed
    `fn_factionDbPrefix.sqf` (no longer needed) and its `"civilian"/"police"/"medic"` counterpart
    naming, which was the source of the earlier civ/police persistence bug in the first place.
  - **`fn_parseStoredPosition.sqf`** (new): validates a loaded `<faction>_position` value's shape
    (real array, non-empty, all three coordinates actually numbers) before ever calling
    `createHashMapFromArray` on it, returning `nil` instead of crashing on anything malformed --
    `fn_spawnPlayer.sqf` had called that directly on untrusted data with zero validation, which is
    exactly what "Invalid number in expression" turned out to mean once fed anything unexpected.
    Falls back to the faction's first configured spawn point rather than leaving the player stuck.
  - **`dialog/common_ui.hpp`** (new): self-contained `ALife_Rsc*` dialog base classes (explicit
    `type = N` control-type constants, full property sets) instead of `class X : RscText`
    inheriting from the engine's own UI config -- the exact thing that broke the dialog earlier
    ("Undefined base class 'RscText'", needing a forward declaration easy to forget on every new
    dialog). Sidesteps that whole class of bug permanently.
  - **Spawn menu UI**: rebuilt as a proper multi-step flow matching the reference's shape -- side
    buttons (civ/cop/medic) instead of a faction listbox, a spawn point list, and a live map
    preview (marker + pan) of the selected point, replacing the original bare-bones two-listbox
    dialog. Split into one function per responsibility (`fn_spawnMenuOpen`/`SelectSide`/
    `SelectLocation`/`Spawn`/`Close`.sqf), wired via the dialog's own `onLoad`/`onUnload` rather
    than being driven externally -- removed `fn_confirmSpawn.sqf` and
    `fn_spawnMenuFactionChanged.sqf`, superseded by this split.
  - Verified via `tools/test_local_server.ps1` against the real local Arma 3 Server install: the
    entire config stack (`description.ext`, `CfgFunctions.hpp`, `CfgRemoteExec.hpp`,
    `config/spawn_config.hpp`, `dialog/common_ui.hpp`, `dialog/spawnMenu.hpp`) and every new/changed
    `.sqf` function compiles with zero errors -- no "Undefined base class" or "Missing Header"
    warnings, no function-compile errors anywhere in the RPT. Still needs a real Eden Editor
    playtest (actual player join, faction pick, spawn) to confirm end-to-end -- this environment
    can't drive an interactive client.
- `src/ALife.Altis`: reviewed `initServer.sqf`/`initPlayerServer.sqf` against Bohemia's documented
  Mission Event Handler / Initialization Order signatures rather than assuming.
  `HandleDisconnect`'s `params ["_unit", "_id", "_uid", "_name"]` already matched the documented
  `[unit, id, uid, name]` exactly. `initPlayerServer.sqf` only declared `params ["_player"]`, but
  the engine actually calls it with `_this = [player, didJIP]` -- not a functional bug (load-then-
  open-spawn-menu is correct for a JIP reconnect too), but the declaration now reflects the real
  argument shape instead of silently dropping it.
- `src/ALife.Altis`: consolidated the spawn menu's five one-function files
  (`fn_spawnMenuOpen/SelectSide/SelectLocation/Spawn/Close.sqf`) into one mode-dispatched
  `fn_spawnMenu.sqf` (`"open"`/`"onLoad"`/`"selectSide"`/`"selectLocation"`/`"spawn"`/`"onUnload"`)
  -- all five were tightly coupled to the same dialog and its selection state, never meaningfully
  called independently. `dialog/spawnMenu.hpp`'s actions/`onLoad`/`onUnload`/`onLBSelChanged`
  updated to pass the mode; no behavior change. Verified via `tools/test_local_server.ps1`.
- `src/cpp_extension` + `src/ALife.Altis`: added DB connection resilience and a periodic
  autosave/keep-alive pulse.
  - **`Database::EnsureConnected()`** (new, `db.cpp`): the extension had no reconnect logic at all
    -- a Postgres restart or network blip would silently fail every load/save for the rest of the
    server's uptime. Every public `Database` method now calls this first. Caught a real bug while
    building it: trusting `PQstatus() == CONNECTION_OK` alone isn't enough, since that only
    reflects libpq's cached belief and doesn't proactively notice a server-killed connection --
    confirmed with a live `pg_terminate_backend` test where a naive status check let a dead
    connection straight through to a failed query. Fixed by having `EnsureConnected` do a real
    `SELECT 1` probe before trusting the cached status, only falling back to `PQreset()` when that
    probe actually fails. Re-verified the same live-kill test afterward -- the connection now
    self-heals within one call, no server restart needed.
  - **`fn_sync.sqf`** (new): a periodic pulse spawned once from `initServer.sqf` (same
    function-compile-race guard as `initPlayerServer.sqf`'s `ALife_fnc_load` call). Every 60s:
    pings the DB (triggering `EnsureConnected`'s recovery on a schedule rather than only ever
    discovering a dead connection the next time a player happens to load/save) and autosaves every
    connected, actively-playing player's alive/position state.
  - **`fn_savePlayerState.sqf`** (new): the alive/position-saving logic factored out of
    `initServer.sqf`'s `HandleDisconnect` so `fn_sync.sqf`'s autosave can share it --
    `HandleDisconnect` alone never covers an ungraceful server crash, exactly the gap autosave
    exists to close.
  - Reviewed an old local prototype's `fn_autoSave.sqf`/`fn_serverPulse.sqf` for the general "spawn
    one loop, sleep, act" shape -- not copied wholesale. That prototype's DB event-polling loop
    (website/Discord commands reaching the live server) is real, useful, separate future work, not
    something this pass builds.
- `src/ALife.Altis`: closed a real gap flagged by the user -- almost everything in the connect/
  disconnect/spawn/sync lifecycle was **failure-only** logged, meaning a normal, working session
  produced zero RPT output and there was no way to positively confirm any of it was actually
  happening. Added the missing success-path lines:
  - `initPlayerServer.sqf`: logs "player connecting" on join (noting `[JIP]` if applicable) and
    "player connected ... load OK" once the record load succeeds -- previously only the failure
    case logged anything at all.
  - `initServer.sqf`'s `HandleDisconnect`: logs "player disconnecting" before the save now runs.
  - `fn_savePlayerState.sqf`: previously logged nothing, success or failure. Now logs the actual
    `ALife_fnc_save` result for both the alive and position writes -- a silent DB failure during a
    disconnect-time save used to be completely invisible.
  - `fn_spawnPlayer.sqf`: logs the actual outcome (resumed at a stored position / spawned fresh at
    a location), not just rejections -- previously a *successful* spawn was silent.
  - `fn_sync.sqf`: logs its initial DB connection state unconditionally on start (previously
    assumed connected and only logged on a later state *change*, so a DB that was already dead at
    boot never got an explicit line), and now logs a count when it actually autosaves someone.
  - Caught and fixed a real bug surfaced while writing these: used `[falseVal, trueVal] select
    someBoolean` (a common community idiom) for a couple of the new log lines, but
    `docs/arma/arma3.db`'s own `select` entry documents its index as `Number`, not `Boolean`, and a
    web search couldn't confirm the coercion is actually guaranteed either. Rather than ship an
    unverified assumption, replaced every instance (including one already merged in `fn_sync.sqf`)
    with plain `if/then/else`, which needs no such assumption.
- `src/server_manager` + `tools/test_local_server.ps1`: added `-autoInit` to the dedicated server's
  launch arguments. `persistent = 1;` alone (already set, both places) only keeps a mission running
  after every player has left -- it doesn't make it start "playing" on its own; without a real
  client to trigger that transition, the mission sits in the pre-play lobby/briefing state
  indefinitely. `-autoInit` initializes the mission at boot the same as the first client connecting
  would (silently ignored unless `persistent=1`, which both call sites already set unconditionally).
  Verified via a live A2S query before/after: the server's reported status changed from a bare
  "Waiting" to the mission's actual briefing name and gametype once `-autoInit` was added --
  confirmed it's genuinely running, not just guessed from the docs.
- `src/ALife.Altis`: `-autoInit` above had an immediate side effect -- it's what finally made the
  server actually run `fn_load.sqf` for the first time in this whole project's history through
  *real SQF*, rather than only ever through `test_harness.exe`'s direct `LoadLibrary`/
  `GetProcAddress` calls (which never touch `callExtension`, `parseSimpleArray`, or
  `createHashMapFromArray` at all). It immediately surfaced two real, previously-invisible syntax
  bugs that had apparently existed since Phase 1:
  - **`callExtension`'s array-args form returns an `Array`, not a `String`.** `"ext" callExtension
    [command, args]` returns `[resultString, returnCode]` in this Arma version -- every direct call
    site (`fn_load.sqf`, `fn_save.sqf`, `fn_sync.sqf`) had been comparing/parsing that whole array
    as if it were the response string, producing a live `Error ==: Type Array, expected ... String`
    the moment real SQF finally exercised it. Added `fn_callExtension.sqf` as the one place that
    unwraps it (`select 0`), so no new call site can make the same mistake again -- every other
    file now goes through `ALife_fnc_callExtension` instead of calling `"tasdyn_alife"
    callExtension` directly.
  - **`parseSimpleArray`/`createHashMapFromArray` were being called postfix
    (`value call parseSimpleArray`, `value createHashMapFromArray`) instead of their actual unary
    prefix syntax (`parseSimpleArray value`, `createHashMapFromArray value`)** -- confirmed against
    real BIS wiki examples, since `docs/arma/arma3.db`'s own syntax field for both commands was
    empty. This produced a parse-time "unexpected )" / "Invalid number in expression" the instant
    they actually ran. Fixed in `fn_load.sqf` and `fn_parseStoredPosition.sqf`.
  - Both bugs retroactively explain a chunk of the "undefined `_record`" mysteries chased earlier
    in this project's history that were never fully root-caused at the time (only the extension-
    deployment and function-compile-race issues found alongside them were) -- `fn_load.sqf` failing
    silently on either bug leaves `_record` exactly that kind of undefined.
  - Take-away for later: `test_harness.exe` proves the C++ extension itself works, but it can never
    catch an SQF-side calling-convention mistake, since it bypasses `callExtension` entirely. A real
    server actually running the mission's own SQF (which `-autoInit` finally forced) is the only
    thing that exercises this path -- worth remembering before trusting "the extension test passes"
    as proof the mission code that calls it is also correct.
