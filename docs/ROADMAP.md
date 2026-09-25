# Roadmap

**Revised target: public launch by 2027-01-20 (~17 weeks from project start on 2026-09-25).**

> **This date moved from the original 2026-12-26 target.** The admin-tooling scope grew from
> "basic tooling" (kick/ban/teleport/spectate) to feature parity with
> [infiSTAR](https://infistar.de/product/infistar-arma3) and
> [Fini Anti-Hack & Admin Tools](https://bytex.market/products/item/7iclegb5zmytw3d22q3l/Fini%20Anti-Hack%20%26%20Admin%20Tools)
> plus arsenal editing — see [ADMIN_TOOLS.md §11](ADMIN_TOOLS.md#11-timeline-impact--the-honest-part) —
> and that scope was made launch-critical rather than deferred. **~4 added weeks is this project's
> estimate, not a confirmed decision** — treat Phase 3's new window below as a proposal to confirm
> or adjust, not a settled fact.

This is a solo-scoped plan. The web dashboard and Discord bot (both already marked "planned" in
the [README](../README.md#2-technical-stack--architecture)) are **cut from the launch critical
path** and pushed to post-launch fast-follow — the server has to be playable end-to-end without
them. Content scope within each faction is also deliberately thin at launch (fewer jobs, fewer
vehicle tiers) so the core loop ships on time; breadth gets added after launch, not before.

Every task below is tracked as a GitHub issue on the
[Delivery Board](https://github.com/orgs/A3-TasmanDynamics/projects/1), grouped into a milestone
per phase. This doc is the map; the board is the source of truth for what's actually done.

---

## Phase 0 — Foundations *(2026-09-25 → 2026-10-05)*

Mostly the scaffold work already merged in PR #1/#2. What's left before any gameplay code starts:

- [ ] Write the save/load **data contract** doc (`docs/DATA_CONTRACT.md`) — the exact field
      order/types shared by the Postgres schema, the C++ extension's `RVExtensionArgs` return
      format, and the SQF `select`/array-building code that parses it. This is the single most
      important doc in the repo — the prototype's save/load bug happened because this contract
      only ever existed implicitly, split across three files that drifted out of sync.
- [ ] Canonical Postgres schema (`database/schema.sql`) for: player record (uid, name, cash,
      bank, faction, rank), inventory (JSONB), licenses (JSONB), and staff ranks
      (`staff_ranks` + `players.staff_rank_id` — see [ADMIN_TOOLS.md §3](ADMIN_TOOLS.md#3-data-model)).
- [ ] Local dev Postgres instance + `config.ini` set up and connecting.
- [ ] HEMTT (or equivalent) build tooling decided for the C++ extension and the mission.

**Milestone exit criteria:** data contract doc merged, schema applied to a local DB, dev
environment reproducible from a clean clone.

## Phase 1 — Core Bridge & Persistence *(2026-10-06 → 2026-10-26)*

- [ ] C++ extension skeleton: `RVExtension`/`RVExtensionArgs` entry points, `libpqxx` connection
      pool, config loading from `config.ini`.
- [ ] `cmd_save` / `cmd_load` implemented against the Phase 0 schema, **prepared statements
      only** — no hand-built SQL strings.
- [ ] SQF request/response framework skeleton: the "Client Requests, Server Decides" pattern —
      one documented event name convention, one server-side dispatcher, one client-side
      acknowledgement path.
- [ ] `CfgRemoteExec` allowlist: only named, reviewed functions are network-callable at all — the
      enforced version of "Server Decides," per [ANTI_CHEAT.md §3 Layer 1](ANTI_CHEAT.md#layer-1--api-surface-remoteexec-allowlist).
      Admin actions (Phase 3) reuse this same allowlist with an added `min_level` check per
      function — see [ADMIN_TOOLS.md §4](ADMIN_TOOLS.md#4-security-model--every-admin-action-is-a-layer-1-action-too).
- [ ] End-to-end smoke test: a player joins, a blank record is created, cash changes on the
      server, disconnect, rejoin — balance persisted correctly.
- [ ] Transaction locking **and idempotent request tokens** on economy-affecting writes (prevents
      both the concurrent-write and the double-submit duplication classes — see
      [ANTI_CHEAT.md §3 Layer 2](ANTI_CHEAT.md#layer-2--economic-integrity)).

**Milestone exit criteria:** a player's cash/bank/rank survives a disconnect/reconnect cycle
against a real Postgres instance, with no untested code path in the save/load contract, and no
server-side function is network-callable unless it's on the `CfgRemoteExec` allowlist.

## Phase 2 — Core Gameplay Loop *(2026-10-27 → 2026-11-23)*

The biggest phase — this is what makes it a *Life* server rather than a database demo.

- [ ] Faction spawn/selection: West (Police/APF), Independent (Medics/AMS), Civilian.
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

## Phase 4 — Content & Balance *(2026-12-30 → 2027-01-08, shifted)*

- [ ] Economy balance pass on whatever jobs/shops shipped in Phase 2.
- [ ] Rebel sub-faction gate (if time allows — first item to cut if the schedule slips).
- [ ] Second vehicle tier for Police/Medics (if time allows).
- [ ] Bug fixes surfaced by internal playtesting.

**Milestone exit criteria:** the team (or solo dev) can play a 2+ hour session across all three
factions without a save/load bug or an obvious exploit.

## Phase 5 — Closed Alpha *(2027-01-09 → 2027-01-15, shifted)*

- [ ] Invite a small closed group, real concurrent players against the real DB.
- [ ] Bug bash — triage everything found, fix save/load and duplication-class issues first.
- [ ] Performance pass under real player count (not just the Phase 3 synthetic soak test).
- [ ] Dedicated pass on the admin menu itself: every tier's actions exercised by a real staff
      member, not just the developer — this surface is large enough now to need its own
      verification, not just gameplay testing.

**Milestone exit criteria:** no known data-corrupting bug; server holds its target player count
for a full session without a restart; every admin-menu action has been used at least once by
someone other than whoever built it.

## Phase 6 — Launch Prep & Public Launch *(2027-01-16 → 2027-01-20, shifted)*

- [ ] Production server hosting finalized, backup/restore drill run at least once.
- [ ] Whitelist/rules process (even a lightweight one) in place.
- [ ] Launch announcement.
- [ ] **Public launch — 2027-01-20.**

---

## Post-Launch / Fast-Follow (explicitly out of scope for launch)

- NuxtJS web dashboard (tickets, gang management, player stats, RCON graphs).
- Discord bot, two-way synced with Postgres (an admin-menu report *webhook* is in-scope per
  [ADMIN_TOOLS.md §8](ADMIN_TOOLS.md#8-access-control-banlist-and-reporting) — the full two-way bot
  is the separate, still-deferred piece).
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

**~17 weeks** for a from-scratch custom C++/Postgres bridge, a custom SQF framework, three
factions, an economy, *and* an admin/anti-hack suite built to match two commercial products, is
aggressive for a solo effort — more aggressive than the original 13-week estimate, and that
estimate was already called aggressive. Two places a slip is most likely to show up, in order:

1. **Phase 3** (now the largest single phase by scope, not just Phase 2) — if the admin-tooling
   feature list in [ADMIN_TOOLS.md §6](ADMIN_TOOLS.md#6-menu-sections) is running long, the metrics
   dashboard and in-menu log viewer (§9) are the most cuttable pieces — RPT/direct DB query is a
   real fallback for both, unlike the security-relevant parts of this phase.
2. **Phase 2** (the gameplay loop) — if it slips, cut content within Phase 2/4 (fewer jobs, fewer
   vehicle tiers) before cutting time from Phase 3's security-relevant work or Phase 5 (alpha
   testing).

Shipping a smaller, stable launch beats shipping a bigger, exploitable one — and a later launch
with the security/moderation surface actually solid beats an on-time launch that isn't.
