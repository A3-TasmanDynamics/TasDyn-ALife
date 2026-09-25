# Roadmap

**Revised target: public launch by 2027-03-03 (~23 weeks from project start on 2026-09-25).**

> **This date has moved twice.** First from the original 2026-12-26 to 2027-01-20, when the
> admin-tooling scope grew from "basic tooling" (kick/ban/teleport/spectate) to feature parity with
> [infiSTAR](https://infistar.de/product/infistar-arma3) and
> [Fini Anti-Hack & Admin Tools](https://bytex.market/products/item/7iclegb5zmytw3d22q3l/Fini%20Anti-Hack%20%26%20Admin%20Tools)
> plus arsenal editing (see [ADMIN_TOOLS.md §11](ADMIN_TOOLS.md#11-timeline-impact--the-honest-part)).
> Now again from 2027-01-20 to 2027-03-03, adding the full website — public site, member portal,
> Admin Panel, Support Panel, Discord integration — as launch-critical rather than post-launch
> fast-follow (see [WEBSITE.md §11](WEBSITE.md#11-timeline-impact--the-honest-part)). **Both
> revisions follow the same pattern: a piece originally scoped as "basic" or "deferred" was made
> launch-critical instead, on purpose, at the cost of the date.** ~6 added weeks for the website is
> this project's estimate, not a confirmed decision — treat Phase W's window below as a proposal to
> confirm or adjust, same as Phase 3's was.

This is a solo-scoped plan, which matters for how "parallel track" (Phase W below) should actually
be read: the website shares almost no *code* with the SQF/C++ side, so it doesn't block Phase 1–4
gameplay work technically — but one person still only has one set of hours, so it is not free
calendar time. The schedule below treats Phase W as genuinely additive, not hidden inside the
existing window, for the same reason Phase 3's scope increase wasn't hand-waved as absorbable.
Content scope within each faction is still deliberately thin at launch (fewer jobs, fewer vehicle
tiers) so the core gameplay loop isn't what's absorbing this cost.

Every task below is tracked as a GitHub issue on the
[Delivery Board](https://github.com/orgs/A3-TasmanDynamics/projects/1), grouped into a milestone
per phase. This doc is the map; the board is the source of truth for what's actually done.

---

## Phase 0 — Foundations *(2026-09-25 → 2026-10-05)*

Mostly the scaffold work already merged in PR #1/#2. What's left before any gameplay code starts:

- [x] Write the save/load **data contract** doc (`docs/DATA_CONTRACT.md`) — the exact field
      order/types shared by the Postgres schema, the C++ extension's `RVExtensionArgs` return
      format, and the SQF `select`/array-building code that parses it. This is the single most
      important doc in the repo — the prototype's save/load bug happened because this contract
      only ever existed implicitly, split across three files that drifted out of sync.
- [x] Canonical Postgres schema (`database/schema.sql`) for: player record (uid, name, cash,
      bank, faction, rank), inventory (JSONB), licenses (JSONB), and staff ranks
      (`staff_ranks` + `players.staff_rank_id` — see [ADMIN_TOOLS.md §3](ADMIN_TOOLS.md#3-data-model)).
      Reviewed against [AsYetUntitled/Framework](https://github.com/AsYetUntitled/Framework)
      (Tonic's widely-deployed Altis Life base) before finalizing — added persisted per-faction
      alive/position (prevents a known disconnect-to-escape-death/arrest exploit in this genre),
      a `wanted_crimes` table, name history, and per-hitpoint vehicle damage as a direct result.
- [x] Local dev Postgres instance + `config.ini` set up and connecting.
- [x] Build tooling decided for the C++ extension: MSVC (`cl.exe`) directly via `build.ps1`, not
      HEMTT — HEMTT builds Arma content/PBOs, not an arbitrary C++ DLL; no cmake/vcpkg needed
      either, since `libpq` ships with the local PostgreSQL server install already.

**Milestone exit criteria:** data contract doc merged, schema applied to a local DB, dev
environment reproducible from a clean clone. ✅ **Phase 0 complete.**

## Phase 1 — Core Bridge & Persistence *(2026-10-06 → 2026-10-26)*

- [x] C++ extension skeleton: `RVExtension`/`RVExtensionArgs` entry points, `libpq` (the C client
      library — ships with the Postgres server install, no vcpkg needed) connection, config
      loading from `config.ini`. Verified end-to-end with a native `LoadLibrary`/`GetProcAddress`
      test harness (no Arma install needed to prove it): `ping` round-trips through the DLL,
      libpq, and a local Postgres instance and returns `OK`.
- [x] `cmd_save` / `cmd_load` implemented against the Phase 0 schema, **prepared statements
      only** — no hand-built SQL strings (`PQexecParams` against fixed, allowlist-selected SQL
      literals). `load` creates a blank record + one `bank_accounts` row per faction on first
      join; `save`'s cash fields are a delta with an idempotency-token check
      (`applied_request_tokens`), everything else is absolute-set. See `docs/DATA_CONTRACT.md`.
- [x] SQF request/response framework skeleton: `src/ALife.Altis/` scaffolded — `ALife_fnc_load`/
      `ALife_fnc_save` implementing `docs/DATA_CONTRACT.md` exactly, `initPlayerServer.sqf` as the
      join hook, `initServer.sqf`'s `HandleDisconnect` hook for the alive/position persistence.
      `mission.sqm` now exists and the whole config stack (`description.ext`, `CfgFunctions.hpp`,
      `CfgRemoteExec.hpp`, `config/spawn_config.hpp`, `dialog/spawnMenu.hpp`) was verified to load
      cleanly on a real local Arma 3 dedicated server — zero errors, server reaches a stable,
      listening, ready state — via `tools/test_local_server.ps1`. **Still not tested with a real
      player connecting** (needs an interactive client, which that script can't drive), so
      `load`/`save`/spawn triggered by an actual join remain unverified end-to-end — the C++/DB
      side they call into is separately verified (PR #57/#60).
- [x] `CfgRemoteExec` allowlist: `CfgRemoteExec.hpp` — `mode = 1` (whitelist-only), the two
      Phase 1 functions listed, `allowedTargets = 2` (server-only). Grows with every new server
      function, same as any allowlist — not a one-time task. Reviewed against
      [Tonic's own `CfgRemoteExec.hpp`](https://github.com/AsYetUntitled/Framework/blob/master/Altis_Life.Altis/CfgRemoteExec.hpp)
      for the real-world pattern (per-function `allowedTargets`/`jip`, separate client/server/HC
      sections) before writing this one. Per [ANTI_CHEAT.md §3 Layer 1](ANTI_CHEAT.md#layer-1--api-surface-remoteexec-allowlist).
      Admin actions (Phase 3) reuse this same allowlist with an added `min_level` check per
      function — see [ADMIN_TOOLS.md §4](ADMIN_TOOLS.md#4-security-model--every-admin-action-is-a-layer-1-action-too).
- [x] End-to-end smoke test (C++/DB side): a blank record is created on first `load`, `civ_cash`
      changes persist across separate calls, a replayed idempotency token is rejected without
      double-applying, and an overspend is rejected without going negative — all verified against
      a real local Postgres instance via the native test harness. **Not yet tested from inside a
      running Arma mission** — that's the SQF-side half of this bullet, still open.
- [x] Idempotent request tokens on economy-affecting writes: `applied_request_tokens`, verified
      against replay (no double-apply) and overspend (no negative balance) — see PR #60. Full
      transaction locking coverage for the four named dupe patterns
      ([ANTI_CHEAT.md §3 Layer 2](ANTI_CHEAT.md#layer-2--economic-integrity)) still needs the
      actual inventory/trading interactions those patterns describe, which don't exist as
      gameplay features yet (Phase 2) — the mechanism they'll reuse is proven, the coverage isn't
      complete until there's something to cover.

**Milestone exit criteria:** a player's cash/bank/rank survives a disconnect/reconnect cycle
against a real Postgres instance, with no untested code path in the save/load contract, and no
server-side function is network-callable unless it's on the `CfgRemoteExec` allowlist. **The
C++/DB half is done and verified (PR #57/#60); the mission config is verified to load cleanly on
a real dedicated server; the actual disconnect/reconnect cycle with a real player still hasn't
been run** — that needs an interactive client connecting, not just the server hosting the mission,
so this phase isn't fully closed out despite every individual task being checked off above.

## Phase 2 — Core Gameplay Loop *(2026-10-27 → 2026-11-23)*

The biggest phase — this is what makes it a *Life* server rather than a database demo.

- [x] Faction spawn/selection scaffolded ahead of schedule, during Phase 1's mission-scaffold work:
      `spawnMenu.hpp` dialog, config-driven spawn points (`config/spawn_config.hpp`), and
      `ALife_fnc_spawnPlayer` (server-authoritative — a player whose stored `<faction>_alive` is
      `false` has their spawn-point request ignored and resumes at `<faction>_position` instead,
      closing the loop on the disconnect-to-escape protection `database/schema.sql` was built
      around). Config verified clean on a real dedicated server alongside the rest of Phase 1 (see
      above); **the spawn flow itself still needs a real player to actually test** — the
      civilian/medic markers (`civ_kavala_spawn`, etc.) also aren't all placed yet. Gear/loadout
      equipping is explicitly not wired in yet, see `src/ALife.Altis/README.md`.
- [ ] Civilian: 2–3 legal jobs at launch (pick the simplest to implement well — e.g. mining,
      trucking; defer fishing/uranium to post-launch).
- [ ] Economy core: physical cash vs. digital bank, a basic buy/sell shop system, one dynamic
      supply/demand market loop (even a simple version).
- [ ] Police: arrest/jail flow, one vehicle tier, basic placeables (cones/barriers).
- [ ] Medic: revive flow (defib + timer), one ambulance vehicle.
- [ ] Visual identity: `setObjectTextureGlobal` faction liveries/uniforms (vanilla-compatible).

**Milestone exit criteria:** a player can spawn into any of the 3 factions, do at least one
job/duty loop specific to that faction, and see money move as a result — all server-authoritative.

## Phase 3 — Anti-Cheat & Admin Tooling *(2026-11-24 → 2026-12-29, revised — was 2026-11-24 → 2026-12-04)*

Full threat model, defense layers, response policy: [ANTI_CHEAT.md](ANTI_CHEAT.md). Full admin
menu, permission, and arsenal-editing spec (including the infiSTAR/Fini feature-parity mapping
that's the reason this phase's window nearly quadrupled): [ADMIN_TOOLS.md](ADMIN_TOOLS.md).

**Anti-cheat:**
- [ ] Enable and configure **BattlEye** (community filter set as a starting point).
- [ ] Honeypot variables, server-side movement validation, rate-limiting of suspicious bursts.
- [ ] Continuous re-checks for the full session (not just on-join) + per-UID repeat-offense
      tracking + first-time player screening (tighter thresholds on a new UID's first session).
- [ ] The specific dupe patterns in [ANTI_CHEAT.md Layer 2](ANTI_CHEAT.md#layer-2--economic-integrity)
      (death-race, vehicle-exit, trade-race, container-desync) — each needs its own re-read of
      current DB state immediately before mutation, not just a generic transaction lock.
- [ ] Graduated flag/alert response wired to the admin menu's flag review panel (below).
- [ ] Load/soak test the C++ bridge under concurrent writes.

**Admin menu — Trial Mod/Mod tiers (levels 10/20):**
- [ ] Player list/search, teleport, spectate, freeze, kick, temp-ban, heal/revive, announcements,
      mute/gag, player report queue, votekick oversight.

**Admin menu — Admin tier (level 40):**
- [ ] Perma-ban, give item/cash (audited before/after), vehicle tools (spawn/delete/repair/
      refuel/rearm/lock/flip), object tools (spawn/delete/move/mass-delete-by-radius).
- [ ] **Arsenal editing**: open arsenal for self/target, per-faction item pools
      (`arsenal_item_pools`), named loadout presets (`arsenal_loadout_presets`) — the explicit
      new requirement this phase was expanded for.
- [ ] Live player map / ESP for staff only (targeted `remoteExec`, never a global broadcast).
- [ ] Anti-cheat flag review panel (closes the loop on graduated response above).
- [ ] Server lockdown toggle, restart scheduler + countdown warnings, mission/time/weather/date
      control.

**Admin menu — Head Admin/Dev tier (level 100):**
- [ ] Staff rank management panel — create/rename/delete a `staff_ranks` row, assign/clear a
      player's rank. This is the "add/remove ranks in the database" requirement, done in-menu.
- [ ] Per-command permission overrides (`staff_permission_overrides`) — grant or revoke a single
      command for a single staff member independent of their rank's default.
- [ ] **Debug console** (arbitrary SQF execVM/compile) — hard-gated to level 100 only, never
      overridable down, typed-confirmation required, full command text logged before execution.
- [ ] Whitelist/banlist management, in-menu categorized log viewer, 24h/3d/7d metrics dashboard.

**Cross-cutting:**
- [ ] Admin action audit logging (who, target, action, reason, before/after values) for every
      item above — see [ADMIN_TOOLS.md §9](ADMIN_TOOLS.md#9-monitoring-logging-and-metrics).

**Milestone exit criteria:** BattlEye active, all ANTI_CHEAT.md Layer 0–3 defenses (including the
four named dupe patterns) pass a deliberate red-team pass, the bridge survives a concurrent-save
soak test, and — the concrete end-to-end test — a Head Admin can create a new staff rank, grant a
Trial Moderator a one-off vehicle-spawn override, edit the Police arsenal's item pool, and review
a flagged anti-cheat event, entirely through the in-game menu with no direct DB access.

## Phase W — Website, Member Portal & Discord Integration *(2026-12-30 → 2027-02-09, new)*

Full design: [WEBSITE.md](WEBSITE.md). Sequenced after Phase 3 rather than fully overlapping it —
the Admin Panel is a *port* of Phase 3's rank/permission model onto a second surface, so that model
needs to exist first. Shares no SQF/C++ code with Phase 1–4, which is what makes it schedulable as
its own phase instead of embedded piecemeal inside them.

- [x] Design doc + schema additions (`web_sessions`, `support_tickets`,
      `support_ticket_messages`, `staff_ranks.default_admin_panel`/`default_support_panel`) —
      this PR.
- [ ] Go project scaffold (`src/website`), Steam OpenID login, DB-backed sessions.
- [ ] Public landing page + status strip (reuses `server_manager`'s dashboard queries).
- [ ] Member portal: stats view, gang management (invite/remove/rank), bank transfers — the last
      one through the *same* `bank_accounts`/`bank_transactions` idempotency path the C++
      extension uses, per [WEBSITE.md §5](WEBSITE.md#5-public-site--member-portal)'s open
      question about `fn_save.sqf` and the `*_bank` cache, which must be resolved before this
      ships, not after.
- [ ] Admin Panel: player lookup, ban/unban, rank + permission-override management (the actual web
      UI for the DB-driven system Phase 3 built the data model for), anti-cheat flag review queue,
      arsenal editor, staff log viewer.
- [ ] Support Panel: ticket queue, claim, thread view; player-facing "My Tickets" in the member
      portal.
- [ ] Discord: one-way webhook logs (staff actions, bans, high-confidence anti-cheat flags, new
      tickets) — the piece [ADMIN_TOOLS.md §8](ADMIN_TOOLS.md#8-access-control-banlist-and-reporting)
      already assumed.
- [ ] Discord: two-way ticket bot (`discordgo`, thread-per-ticket, mirrors replies both directions).
- [ ] Security pass: CSRF on every state-changing form, rate limiting on login and transfers,
      server-side re-check of panel access on every write (not just at login).

**Milestone exit criteria:** a player can log in with Steam, view their stats, invite someone to
their gang, and send money to another player, entirely from the website; a staff member holding
both grants can act in the Admin Panel and Support Panel and switch between them without
re-authenticating; a new support ticket appears in Discord and a staff reply from Discord appears
on the website ticket.

## Phase 4 — Content & Balance *(2027-02-10 → 2027-02-19, shifted)*

- [ ] Economy balance pass on whatever jobs/shops shipped in Phase 2.
- [ ] Rebel sub-faction gate (if time allows — first item to cut if the schedule slips).
- [ ] Second vehicle tier for Police/Medics (if time allows).
- [ ] Bug fixes surfaced by internal playtesting.

**Milestone exit criteria:** the team (or solo dev) can play a 2+ hour session across all three
factions without a save/load bug or an obvious exploit.

## Phase 5 — Closed Alpha *(2027-02-20 → 2027-02-26, shifted)*

- [ ] Invite a small closed group, real concurrent players against the real DB.
- [ ] Bug bash — triage everything found, fix save/load and duplication-class issues first.
- [ ] Performance pass under real player count (not just the Phase 3 synthetic soak test).
- [ ] Dedicated pass on the admin menu itself: every tier's actions exercised by a real staff
      member, not just the developer — this surface is large enough now to need its own
      verification, not just gameplay testing.
- [ ] Same dedicated pass on the website: Admin Panel and Support Panel exercised by a real staff
      member, member portal (stats/gang/transfers) exercised by a real alpha player, not just the
      developer — same reasoning as the admin-menu pass above, same surface-size threshold.

**Milestone exit criteria:** no known data-corrupting bug; server holds its target player count
for a full session without a restart; every admin-menu action and every website Admin/Support
Panel action has been used at least once by someone other than whoever built it.

## Phase 6 — Launch Prep & Public Launch *(2027-02-27 → 2027-03-03, shifted)*

- [ ] Production server hosting finalized, backup/restore drill run at least once (covers the
      website's Postgres usage too — it's the same database, same drill).
- [ ] Website deployed to production hosting, HTTPS/TLS in place (required for `Secure` session
      cookies per [WEBSITE.md §10](WEBSITE.md#10-security-notes) — not optional at launch).
- [ ] Whitelist/rules process (even a lightweight one) in place.
- [ ] Launch announcement.
- [ ] **Public launch — 2027-03-03.**

---

## Post-Launch / Fast-Follow (explicitly out of scope for launch)

- Full [WEBSITE.md](WEBSITE.md) is now launch-critical (Phase W) — **not** deferred; see the
  roadmap header for why this line changed from the original plan.
- Remaining content breadth: uranium mining, fishing, full SOG vehicle tiers, speed cameras,
  physical jail beyond the MVP flow.
- Anti-cheat maturity beyond [ANTI_CHEAT.md](ANTI_CHEAT.md)'s Layer 0–4 launch scope — see its
  §3 Layer 5 for what that covers (tuned thresholds, expanded honeypot set, log-pattern review).
- A literal multi-org shared/cloud ban network (this project's own persistent `banlist` table is
  in-scope; a hosted cross-server service is not — see
  [ADMIN_TOOLS.md §10](ADMIN_TOOLS.md#10-feature-parity-with-infistar--fini)).
- In-menu full audit-log viewer beyond the Admin-tier flag panel; fine-grained permission grants
  beyond the level + override model.

## Risk

**~23 weeks** for a from-scratch custom C++/Postgres bridge, a custom SQF framework, three
factions, an economy, an admin/anti-hack suite built to match two commercial products, *and* a
full website with its own auth/money-moving/Discord-bot surface, is aggressive for a solo effort —
more aggressive than the original 13-week estimate, and that estimate was already called
aggressive. Three places a slip is most likely to show up, in order:

1. **Phase W** (the website) — the newest and least-proven estimate of the three; a Steam OpenID
   integration issue or the `fn_save.sqf`/`*_bank` cache question in
   [WEBSITE.md §5](WEBSITE.md#5-public-site--member-portal) turning out to need real SQF-side
   changes (not just a website-side check) is the likeliest way this phase's window is
   optimistic. The two-way Discord bot and the admin-panel port are next-most cuttable if it runs
   long — a one-way webhook log and a solid member portal alone still deliver most of the value.
2. **Phase 3** — if the admin-tooling feature list in
   [ADMIN_TOOLS.md §6](ADMIN_TOOLS.md#6-menu-sections) is running long, the metrics dashboard and
   in-menu log viewer (§9) are the most cuttable pieces — RPT/direct DB query is a real fallback
   for both, unlike the security-relevant parts of this phase.
3. **Phase 2** (the gameplay loop) — if it slips, cut content within Phase 2/4 (fewer jobs, fewer
   vehicle tiers) before cutting time from Phase 3's security-relevant work, Phase W's
   security-relevant work (§10), or Phase 5 (alpha testing).

Shipping a smaller, stable launch beats shipping a bigger, exploitable one — and a later launch
with the security/moderation surface actually solid beats an on-time launch that isn't.
