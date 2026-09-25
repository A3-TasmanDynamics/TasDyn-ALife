# Admin Tools: In-Game Staff Menu & Rank Management

Staff need to moderate and run the server from inside the game — kicking, banning, teleporting,
investigating a report — without reaching for RCON/BEC for routine work. This doc covers two
related but distinct systems:

1. An **in-game admin menu**, permission-gated by staff rank.
2. A **DB-backed staff rank system** — ranks are rows, not hardcoded constants, so adding a rank,
   removing one, or reassigning a player to one is an admin-menu action, not a code deploy.

## 1. Scope note

"Ranks" here means **staff/moderation ranks** (Trial Mod → Head Admin), which is what gates the
admin menu itself. This is a different system from **faction ranks** (Police/Medic career
progression — Constable → Sergeant, etc.), which is [Phase 2](ROADMAP.md#phase-2--core-gameplay-loop-2026-10-27--2026-11-23)
gameplay content, not an admin-tooling concern. §6 below notes how a faction-rank table could
reuse this same shape later, but it isn't built as part of this doc. If the intent was actually
the faction rank system, or both, flag it — this doc only covers staff ranks.

## 2. What's DB-driven vs. what's still code

Worth being precise here, because "add/remove ranks in the database" has two readings:

- **DB-driven — no code change needed:** which ranks exist, what they're named, their permission
  `level`, and which player currently holds which rank. All of it lives in Postgres and is
  editable live through the top-tier admin menu panel.
- **Still code — capability wiring:** *what a given permission level unlocks* (e.g. "kick requires
  level ≥ 10") is a constant in the SQF admin module, not a DB row. This is deliberate:
  level-based gating means a brand-new rank slotted in at, say, level 15 automatically inherits
  every capability gated at ≤ 15 with zero code changes — only a genuinely *new capability* (a new
  menu button) ever needs a code change, never a new *rank*.

## 3. Data model

**`staff_ranks`**

| Column | Type | Notes |
|---|---|---|
| id | serial PK | |
| key | text, unique | stable identifier, e.g. `trial_mod` |
| display_name | text | shown in the menu UI, e.g. "Trial Moderator" |
| level | int | permission threshold — every gated action checks `caller.level >= action.min_level` |
| created_at | timestamptz | |

**`players.staff_rank_id`** — nullable FK into `staff_ranks`. `null` = regular player, no admin
menu access at all.

Default seed ranks (edit/reorder freely once the table exists — a starting point, not a
requirement):

| Rank | Level | Intent |
|---|---|---|
| Trial Moderator | 10 | Kick, teleport, spectate, freeze — low-risk, reversible actions only |
| Moderator | 20 | + temp-ban, heal/revive, server announcements |
| Admin | 40 | + perma-ban, give item/cash (compensation), vehicle spawn/delete, anti-cheat flag review |
| Head Admin / Developer | 100 | + staff rank management panel (add/remove ranks, assign players) |

This directly replaces the prototype's loose `adminlevel` field — the one that got silently
overwritten by the old save/load field-mapping bug (the whole reason this project tracks its
save/load contract explicitly in the first place). That's exactly why staff rank lives in its own
table with its own FK instead of a positional integer squeezed into the player save array: a
dedicated table can't collide with an unrelated field the way a positional array index can.

## 4. Security model — every admin action is a Layer 1 action too

Every admin-menu button is, mechanically, just another `remoteExec`'d server function — so it goes
through the same [`CfgRemoteExec` allowlist](ANTI_CHEAT.md#layer-1--api-surface-remoteexec-allowlist)
as every other client request, with one addition: each allowlisted admin function declares a
`min_level`, and the server re-checks the caller's *current* `staff_rank_id` from the DB on every
call — never a client-supplied "I'm an admin" flag, and never a value cached from login. A demoted
staff member loses menu access on their very next action, not on their next reconnect.

This is also where the honeypot pattern from
[ANTI_CHEAT.md Layer 3](ANTI_CHEAT.md#layer-3--movement--behavior-heuristics) earns its keep
twice over: off-the-shelf cheat menus very often target generic `"isAdmin"`-style variables
specifically to unlock admin-only functionality. Because real admin status here is never
represented by a client-side variable at all, that attack has nothing to grab onto.

## 5. Menu sections

Grouped by the minimum staff level that unlocks them:

### Level 10 — Trial Moderator: low-risk, reversible

* Player list/search (online players, by name or UID)
* Teleport (to player, player-to-me, to coordinate/marker)
* Spectate
* Freeze / unfreeze
* Kick (reason required, logged)

### Level 20 — Moderator: adds standard moderation

* Temp-ban (duration + reason required)
* Heal / revive
* Server-wide announcement / broadcast message

### Level 40 — Admin: adds server-ops and higher-trust actions

* Perma-ban (reason required)
* Give/remove item, adjust cash or bank balance — for incident compensation; every use logged with
  before/after values, since this is mechanically adjacent to the exact duplication exploit
  [ANTI_CHEAT.md](ANTI_CHEAT.md) defends against and needs the same audit trail
* Vehicle spawn / delete / repair (event support)
* **Anti-cheat flag review panel** — see §7

### Level 100 — Head Admin / Developer: staff rank management

* Create/rename/delete a `staff_ranks` row, change its `level`
* Assign or clear a player's `staff_rank_id`

## 6. Reuse for faction ranks (not built here)

If/when Phase 2 wants data-driven faction ranks (Police Constable → Sergeant, etc.) instead of
hardcoded values, the same `key`/`level`/`display_name` shape applies — a `faction_ranks` table
plus `players.faction_rank_id`, gated by faction rather than by admin-menu level. Noted here so
the two systems stay consistent if that work happens, not because this doc builds it.

## 7. Closing the anti-cheat loop

The Admin tier's flag review panel lists flagged events from
[ANTI_CHEAT.md Layer 4](ANTI_CHEAT.md#layer-4--response--ops) — honeypot triggers, movement
threshold breaches, rejected idempotency tokens — each with a one-click kick/ban/dismiss action.
This is the actual human-review step that "flag + admin alert" depends on; without this panel,
graduated response has nowhere in-game to surface.

## 8. Audit logging

Every admin action writes one row: acting staff member, target player, action, reason (required
for kick/ban/balance changes), before/after values where relevant, timestamp. Same shape as
`Syslog` in [TasDyn-AI](https://github.com/A3-TasmanDynamics/TasDyn-AI) — one category, one
severity, one routed channel — rather than a second bespoke logging system. Feeds the same
interim-channel question already open in
[ANTI_CHEAT.md §6](ANTI_CHEAT.md#6-open-questions-to-settle-during-phase-13).

## 9. Out of scope for launch

* Faction rank progression — Phase 2 content, not this doc (§6).
* A full audit-log viewer inside the menu (beyond the Level 40 flag panel) — post-launch; RPT/DB
  query is the fallback until then.
* Fine-grained per-action permission overrides beyond the level threshold model — if a server
  ever needs "this specific Moderator can also spawn vehicles," that's a scope change to this
  model, not something it supports out of the box.
