# Admin Tools: In-Game Staff Menu & Rank Management

Staff need to moderate and run the server from inside the game — kicking, banning, teleporting,
investigating a report, editing the arsenal — without reaching for RCON/BEC for routine work.

**Scope target: feature parity with [infiSTAR](https://infistar.de/product/infistar-arma3) and
[Fini Anti-Hack & Admin Tools](https://bytex.market/products/item/7iclegb5zmytw3d22q3l/Fini%20Anti-Hack%20%26%20Admin%20Tools),
plus arsenal editing, built launch-critical rather than deferred.** §10 maps every feature from
both products to where it lands here — matched, exceeded, or deliberately scoped differently (and
why). This is a large scope increase over the original 4-bullet "basic admin tooling" line in
Phase 3 — see [§11](#11-timeline-impact--the-honest-part) for what that actually does to the
schedule.

This doc covers three systems:

1. An **in-game admin menu** — player, vehicle, object, arsenal, and server tooling.
2. A **DB-backed staff rank + permission system** — ranks and per-command grants are rows, not
   hardcoded constants.
3. **Anti-hack-adjacent admin features** — the review/response surface for
   [ANTI_CHEAT.md](ANTI_CHEAT.md)'s detections. Detection logic itself still lives in that doc;
   this doc covers what staff *do* with a detection once it fires.

## 1. Scope note

"Ranks" here means **staff/moderation ranks** (what gates the admin menu), not faction career
ranks (Police Constable → Sergeant). §9 notes how a `faction_ranks` table could reuse the same
shape later; it isn't built here.

## 2. What's DB-driven vs. what's still code

- **DB-driven — no code change needed:** which staff ranks exist, their `level`, which player
  holds which rank, per-command permission overrides (§7), arsenal item pools and loadout presets
  (§5), the banlist/whitelist (§8).
- **Still code — capability wiring:** *what a given permission level unlocks by default* (e.g.
  "kick requires level ≥ 10") is a constant in the SQF admin module. A new rank at level 15
  inherits everything gated ≤ 15 automatically; a genuinely new *capability* (a new menu button)
  still needs code. Overrides (§7) are the escape hatch when a specific person needs an exception
  to that default without waiting on a deploy.

## 3. Data model

**`staff_ranks`**

| Column | Type | Notes |
|---|---|---|
| id | serial PK | |
| key | text, unique | e.g. `trial_mod` |
| display_name | text | e.g. "Trial Moderator" |
| level | int | default permission threshold |
| created_at | timestamptz | |

**`players.staff_rank_id`** — nullable FK. `null` = no admin menu access.

**`staff_permission_overrides`** *(new — see §7)*

| Column | Type | Notes |
|---|---|---|
| id | serial PK | |
| player_id | FK → players | the specific staff member this override applies to |
| command_key | text | e.g. `vehicle.spawn`, `debug.console` |
| allow | boolean | `true` grants below their rank's default gate, `false` explicitly revokes |
| granted_by | FK → players | audit trail for who set the override |
| created_at | timestamptz | |

**`arsenal_item_pools`** *(new — see §5)*

| Column | Type | Notes |
|---|---|---|
| id | serial PK | |
| faction_key | text | which faction this pool applies to, or `staff` for the admin-only test pool |
| item_classname | text | Arma class name |
| category | text | weapon / attachment / uniform / vest / backpack / item, for menu grouping |

**`arsenal_loadout_presets`** — named, saveable loadouts (JSONB of classnames), taggable to a
faction or rank, editable through the menu.

**`banlist`** *(new — see §8)* — uid, reason, banned_by, expires_at (`null` = permanent),
created_at. This project's own persistent, Postgres-backed banlist — see §8 for why this is
scoped as "our own DB table" rather than a literal shared cloud service.

Default seed ranks (same as before, unchanged):

| Rank | Level | Intent |
|---|---|---|
| Trial Moderator | 10 | Low-risk, reversible actions |
| Moderator | 20 | + standard moderation |
| Admin | 40 | + server-ops, arsenal editing, anti-cheat review |
| Head Admin / Developer | 100 | + rank/permission management, debug console |

Replaces the prototype's loose `adminlevel` field — the one silently overwritten by the save/load
field-mapping bug this project exists to not repeat.

## 4. Security model

Unchanged from the previous version of this doc: every admin action is a
[`CfgRemoteExec`](ANTI_CHEAT.md#layer-1--api-surface-remoteexec-allowlist) function with a
server-side `min_level` (or override, §7) check re-evaluated from the DB on every call — never a
client-supplied or cached flag. The honeypot pattern in
[ANTI_CHEAT.md Layer 3](ANTI_CHEAT.md#layer-3--movement--behavior-heuristics) exists precisely
because generic cheat menus target client-side `"isAdmin"`-style variables; this design gives
that attack nothing to grab.

**The debug console (§6) is the one exception that needs its own paragraph**: it is, by
definition, arbitrary code execution — the highest-privilege action in the entire system. It gets
its own hard-coded level-100-only gate (never eligible for a §7 override down to a lower tier),
a mandatory typed-confirmation step client-side (not just a click), and the full command text is
logged verbatim before execution, not after.

## 5. Arsenal editing

The explicit new requirement, and one of the two Fini/infiSTAR-inspired additions with no direct
precedent in the original doc:

- **Open the Virtual Arsenal** for self or a target player (Admin tier, level 40) — the base
  infiSTAR-equivalent "edit arsenal" action.
- **Per-faction item pools** (`arsenal_item_pools`): what shows up in a Civilian's arsenal is not
  the same list as a Police or Medic arsenal — gated by faction, not just by whether the arsenal
  is open at all. Prevents the trivial "everyone can see every weapon in the game via the vanilla
  arsenal UI" problem that an unfiltered Virtual Arsenal has out of the box.
- **Named loadout presets** (`arsenal_loadout_presets`), taggable to a faction or a staff/faction
  rank, editable in-menu — "give this player the standard Sergeant loadout" as one action instead
  of manually re-picking every item.
- Item pool and preset edits are DB writes, same DB-driven philosophy as staff ranks — a new
  weapon can be added to the Police pool without a mission redeploy.

## 6. Menu sections

### Level 10 — Trial Moderator

Player list/search, teleport (to player / to me / to coordinate/marker), spectate, freeze/unfreeze,
kick (reason required).

### Level 20 — Moderator

Adds: temp-ban, heal/revive, server-wide announcement, mute/gag chat, **player report queue**
(§8), votekick oversight (accept/veto an in-progress vote).

### Level 40 — Admin

Adds: perma-ban, give/remove item or adjust cash/bank (audited, before/after logged), **arsenal
editing (§5)**, vehicle tools (spawn/delete/repair/refuel/rearm/lock/flip/teleport-to-me), object
tools (spawn/delete single or mass-nearby-radius/move/rotate), **live player map / ESP for staff
only** (positions sent via targeted `remoteExec` to staff clients specifically — never a global
broadcast, per [ANTI_CHEAT.md's information-leakage mitigation](ANTI_CHEAT.md#5-explicitly-out-of-scope)),
**anti-cheat flag review panel** (unchanged from before), server lockdown toggle (whitelist-only
join mode), restart scheduler + countdown warnings, mission/time/weather/date control.

### Level 100 — Head Admin / Developer

Adds: staff rank management (create/rename/delete a rank, change its level), **per-command
permission overrides (§7)**, **debug console (§6)**, whitelist/banlist management (§8), metrics
dashboard (§9).

## 7. Per-command permission overrides

The original version of this doc scoped granular per-person overrides as out-of-scope; the
infiSTAR/Fini feature bar makes that call wrong now, so it's revised in. `staff_permission_overrides`
lets a Head Admin grant (or explicitly revoke) a single `command_key` for a single staff member,
independent of their rank's default level threshold — "this specific Moderator can also spawn
vehicles" without inventing a whole new rank tier for one person. Every gated command checks, in
order: (1) is there an override row for this player + command? use it. (2) otherwise, fall back to
the rank-level default. The debug console (§6) is hard-excluded from ever being grantable this way.

## 8. Access control, banlist, and reporting

- **Whitelist**: server-join allowlist by UID, managed at level 100.
- **Banlist**: our own persistent Postgres table (`banlist`), not a literal third-party shared
  cloud-ban service — see [§10](#10-feature-parity-with-infistar--fini) for why "cloud banlist" is
  scoped this way rather than as a literal hosted cross-server network. It's still
  server-authoritative, survives restarts, and is trivially extensible to a second Tasman Dynamics
  server later if one exists — it's just not a multi-org shared service on day one.
- **Player reports**: an in-game `/report` command any player can use, landing in the Moderator+
  queue (§6) and mirrored to a Discord webhook for offline visibility.
- **First-time player screening**: a new UID's first session gets a shorter movement-heuristic
  window and a lower duplication-idempotency debounce tolerance — i.e., new accounts are watched
  more closely by default, tightening back to normal after a clean first session. Directly answers
  Fini's "first-time player screening" feature.

## 9. Monitoring, logging, and metrics

- Every admin action logs: staff member, target, action, reason (required for kick/ban/balance
  changes), before/after values, timestamp — unchanged from before, same shape as
  [TasDyn-AI's `Syslog`](https://github.com/A3-TasmanDynamics/TasDyn-AI).
- **In-menu log viewer** (level 100): categorized, filterable, without needing RPT/DB access.
- **Metrics dashboard** (level 100): rolling 24h/3d/7d player-count, flagged-event, and
  economy-throughput figures — the equivalent of infiSTAR's metrics tab, backed by the same audit
  log rather than a separate telemetry pipeline.

Faction ranks (§1 scope note) could reuse the `key`/`level` table shape later if built — noted,
not built here.

## 10. Feature parity with infiSTAR & Fini

| Feature (source) | Status here | Notes |
|---|---|---|
| Spawn vehicles/buildings (infiSTAR) | ✅ §6 Admin tier | |
| Player control: kick/ban/freeze/teleport/spectate/heal (infiSTAR) | ✅ §6, all tiers | |
| ESP/player-name visibility, livemaps (infiSTAR) | ✅ §6 Admin tier | Staff-only targeted remoteExec, not broadcast |
| Duper detection (infiSTAR), anti-dupe/"toolless dupe methods" (Fini) | ✅ [ANTI_CHEAT.md Layer 2](ANTI_CHEAT.md#layer-2--economic-integrity) | Transaction locking + idempotency; specific dupe patterns enumerated there |
| Toxic player identification, continuous AC tests (infiSTAR) | ✅ [ANTI_CHEAT.md](ANTI_CHEAT.md), expanded | Rate-limiting + repeat-offense tracking |
| Cloud-based banlist / "Vision Cloudban Network" (infiSTAR) | 🔶 Scoped as our own `banlist` table, §8 | Not a literal multi-org hosted service — see §8 rationale |
| RCE via Zeus / developer debugging (infiSTAR, Fini) | ✅ §6 Debug console, level 100 only | Highest-privilege action, non-overridable, verbatim-logged |
| Server lockdown, extended firewall (infiSTAR) | 🔶 Lockdown ✅ §6; firewall → Phase 6 hosting hardening, not an in-game menu feature | Network-level firewalling is an infra/ops concern, not SQF |
| Scheduler / restart warnings (infiSTAR) | ✅ §6 Admin tier | |
| Report players, player votes (infiSTAR) | ✅ §8 reporting, §6 votekick oversight | |
| Role-based server members (infiSTAR) | ✅ `staff_ranks`, this whole doc | |
| Advanced logging, RCon history, online logs, metrics, audit logs (infiSTAR) | ✅ §9 | |
| Whitelist/banlist, join rules (infiSTAR) | ✅ §8 | |
| Discord integration (infiSTAR) | 🔶 Report webhook now (§8); full two-way sync is [TasDyn-AI's Discord bot](ROADMAP.md#post-launch--fast-follow-explicitly-out-of-scope-for-launch), still post-launch | The bot itself is a separate, already-deferred piece of work — a webhook is not |
| Complex/granular permission system (Fini) | ✅ §7 per-command overrides | |
| First-time player screening (Fini) | ✅ §8 | |
| Economy/rare-item dupe protection (Fini) | ✅ ANTI_CHEAT.md Layer 2 | |
| Memory-based cheat detection, e.g. Aurora (Fini explicitly *doesn't* do this either) | ❌ Out of scope, [ANTI_CHEAT.md §5](ANTI_CHEAT.md#5-explicitly-out-of-scope) | Fini's own product page admits the same limit — this isn't us falling short of a bar competitors clear; nobody clears it purely server-side |
| Framework-level exploits, e.g. jailing exploits (Fini explicitly *doesn't* fix these either) | ❌ Framework-specific, not a generic AC tool's job | Same reasoning — this is on our own SQF framework's correctness, covered by the data-contract discipline elsewhere in the roadmap, not by an anti-cheat layer |
| **Arsenal editing (this project's explicit new ask, not from either product)** | ✅ §5 | |

## 11. Timeline impact — the honest part

The original Phase 3 scope was four bullets: honeypots, movement validation, soak test, "basic
admin tooling." This doc now specifies roughly the admin-and-anti-hack feature surface of two
commercial products that have each had years of dedicated development, built launch-critical
rather than deferred, by a solo effort, inside a project that also still needs the from-scratch
C++/Postgres bridge, three factions, and an economy.

That is not a small increase — it is closer to a second major workstream running alongside
gameplay. [ROADMAP.md](ROADMAP.md) has been updated with a **revised target of 2027-01-20** (was
2026-12-26, ~4 added weeks, all absorbed into Phase 3's window) and the full expanded Phase 3 task
list. [ROADMAP.md's Risk section](ROADMAP.md#risk) already says to cut content before cutting
anti-cheat/alpha time — this doc's scope is now large enough that "cut content" alone (fewer jobs,
fewer vehicle tiers) was unlikely to fully absorb it, which is why the date moved instead of the
scope shrinking. That revised date is itself an estimate flagged for confirmation, not a
guarantee — see ROADMAP.md's header note.
