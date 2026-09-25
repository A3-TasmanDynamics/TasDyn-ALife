# Roadmap

**Target: public launch by 2026-12-26 (13 weeks from project start on 2026-09-25).**

This is a solo-scoped plan. To hit a 3-month launch, the web dashboard and Discord bot
(both already marked "planned" in the [README](../README.md#2-technical-stack--architecture))
are **cut from the launch critical path** and pushed to post-launch fast-follow — the server has
to be playable end-to-end without them. Content scope within each faction is also deliberately
thin at launch (fewer jobs, fewer vehicle tiers) so the core loop ships on time; breadth gets
added after launch, not before.

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

## Phase 3 — Anti-Cheat & Hardening *(2026-11-24 → 2026-12-04)*

Full threat model, defense layers, and response policy: [ANTI_CHEAT.md](ANTI_CHEAT.md).

- [ ] Enable and configure **BattlEye** on the dedicated server (community filter set as a
      starting point) — zero client-install cost, catches generic memory/DLL cheat tooling that
      the heuristic layer below isn't designed to.
- [ ] Honeypot variables to trap generic cheat-menu variable manipulation.
- [ ] Server-side movement validation (distance-per-tick) to flag teleportation.
- [ ] Graduated flag/alert response wired up (see [ANTI_CHEAT.md §3 Layer 4](ANTI_CHEAT.md#layer-4--response--ops)) —
      not auto-ban on every signal, since the movement check especially needs tuning against real
      network conditions first.
- [ ] Load/soak test the C++ bridge under concurrent writes (simulate a full server's worth of
      saves).
- [ ] **In-game admin menu**, permission-gated by staff rank — full spec in
      [ADMIN_TOOLS.md](ADMIN_TOOLS.md):
      - Level 10 (Trial Mod): player list/search, teleport, spectate, freeze, kick.
      - Level 20 (Mod): + temp-ban, heal/revive, server announcements.
      - Level 40 (Admin): + perma-ban, give item/cash (audited), vehicle spawn/delete/repair,
        **anti-cheat flag review panel** — closes the loop on Phase 3's graduated response.
      - Level 100 (Head Admin/Dev): + staff rank management panel — create/rename/delete a
        `staff_ranks` row, assign/clear a player's rank. This is the "add/remove ranks in the
        database" requirement, done as an in-menu action rather than a manual DB edit.
- [ ] Admin action audit logging (who, target, action, reason, before/after values) — see
      [ADMIN_TOOLS.md §8](ADMIN_TOOLS.md#8-audit-logging).

**Milestone exit criteria:** BattlEye active, all four Layer 0–3 defenses from ANTI_CHEAT.md
implemented and passing a deliberate red-team pass (teleport, dupe/double-submit, honeypot
trigger), the bridge survives a concurrent-save soak test without corruption, and a Head
Admin-level staff member can create a new staff rank and assign a player to it entirely through
the in-game menu — no direct DB access required.

## Phase 4 — Content & Balance *(2026-12-05 → 2026-12-14)*

- [ ] Economy balance pass on whatever jobs/shops shipped in Phase 2.
- [ ] Rebel sub-faction gate (if time allows — first item to cut if the schedule slips).
- [ ] Second vehicle tier for Police/Medics (if time allows).
- [ ] Bug fixes surfaced by internal playtesting.

**Milestone exit criteria:** the team (or solo dev) can play a 2+ hour session across all three
factions without a save/load bug or an obvious exploit.

## Phase 5 — Closed Alpha *(2026-12-15 → 2026-12-21)*

- [ ] Invite a small closed group, real concurrent players against the real DB.
- [ ] Bug bash — triage everything found, fix save/load and duplication-class issues first.
- [ ] Performance pass under real player count (not just the Phase 3 synthetic soak test).

**Milestone exit criteria:** no known data-corrupting bug; server holds its target player count
for a full session without a restart.

## Phase 6 — Launch Prep & Public Launch *(2026-12-22 → 2026-12-26)*

- [ ] Production server hosting finalized, backup/restore drill run at least once.
- [ ] Whitelist/rules process (even a lightweight one) in place.
- [ ] Launch announcement.
- [ ] **Public launch — 2026-12-26.**

---

## Post-Launch / Fast-Follow (explicitly out of scope for the 3-month target)

- NuxtJS web dashboard (tickets, gang management, player stats, RCON graphs).
- Discord bot, two-way synced with Postgres.
- Remaining content breadth: uranium mining, fishing, full SOG vehicle tiers, speed cameras,
  physical jail beyond the MVP flow.
- Anti-cheat maturity beyond [ANTI_CHEAT.md](ANTI_CHEAT.md)'s Layer 0–4 launch scope — see its
  §3 Layer 5 for what that covers (tuned thresholds, expanded honeypot set, log-pattern review).

## Risk

Thirteen weeks for a from-scratch custom C++/Postgres bridge, a custom SQF framework, three
factions, and an economy is aggressive for a solo effort. Phase 2 (the gameplay loop) is the
most likely place a slip shows up first — if it does, cut content within Phase 2/4 (fewer jobs,
fewer vehicle tiers) before cutting time from Phase 3 (anti-cheat) or Phase 5 (alpha testing).
Shipping a smaller, stable launch beats shipping a bigger, exploitable one.
