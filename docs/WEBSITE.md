# Website: Public Site, Member Portal, Admin Panel, Support Panel

A single Go web application serving four audiences from one codebase:

1. A **public landing page** — no login required.
2. A **member portal** — players log in and manage their account outside the game (view stats,
   manage their gang, send money).
3. An **Admin Panel** — staff moderation/administration, web-equivalent of the in-game admin menu
   ([ADMIN_TOOLS.md](ADMIN_TOOLS.md)).
4. A **Support Panel** — ticket system, bridged to Discord.

Admin and Support are **separate panels with separate access**, not one panel with every button
visible to every staff member — a support volunteer who only triages tickets should never see ban
tools, and vice versa. Staff holding both grants get a panel switcher; staff holding one see only
that one. §4 covers how access is granted.

**Scope target: built out fully at launch, not a stub** — same "don't defer, build it real"
instruction that expanded [ADMIN_TOOLS.md](ADMIN_TOOLS.md) beyond its original scope. §11 is
honest about what that costs the schedule.

## 1. Why one Go app, not four

The four audiences share almost everything that matters: the same Postgres database
([schema.sql](../database/schema.sql)), the same identity (a `players` row), the same session/auth
system, and — for staff — the same rank/permission model already built for the in-game admin menu.
Splitting them into separate services would mean re-solving auth and DB access four times for no
real isolation benefit; a single Go binary with route groups and per-group middleware gets the same
separation (a support volunteer's session simply doesn't pass the admin-panel middleware) with one
codebase, one deploy, one set of dependencies to keep patched.

This mirrors [src/server_manager](../src/server_manager) — Go was already the project's choice for
tooling outside the C++ extension and SQF mission, for the same reasons: a single static binary,
no runtime to install on a host, straightforward Postgres access via `pgx`.

## 2. Tech stack

| Layer | Choice | Why |
|---|---|---|
| Language | Go | Consistent with [server_manager](../src/server_manager); single static binary to deploy. |
| Router | [chi](https://github.com/go-chi/chi) | Thin stdlib-compatible router with clean middleware groups — exactly what per-panel access control needs. |
| Rendering | `html/template`, server-rendered | No SPA build pipeline for content that's mostly forms and tables; matches "don't add abstraction the task doesn't need." Auto-escapes by default (`text/template` does not — this distinction matters, this app touches user-supplied ticket text and player names). |
| DB access | `pgx/v5`, same driver as server_manager's dashboard queries | Already proven against this schema in `src/server_manager/dashboard.go`. |
| Sessions | DB-backed (`web_sessions` table, §6), opaque token in an `HttpOnly`, `Secure`, `SameSite=Lax` cookie | Not JWT — a ban or staff demotion must invalidate a session **immediately**, not wait for token expiry. A DB row can be deleted on the spot; a signed JWT can't be revoked without a blocklist, which is a DB table anyway — so just use the DB table directly. |
| Styling | Hand-written CSS, navy/amber tokens ported from `server_manager/frontend/src/style.css` | Same brand, no CDN dependency for an internet-facing production site (a CDN outage shouldn't take login styling down). |
| Discord | [discordgo](https://github.com/bwmarrin/discordgo) for the ticket bot; plain webhook POSTs for one-way logs | See §9 — two different integration shapes for two different needs. |

Project lives at `src/website/`, its own Go module (like `src/server_manager`) — different
deployment target (long-running server vs. desktop app), no reason to couple their dependency
graphs.

## 3. Authentication — Steam, not a new identity system

Every player already has exactly one durable identity: `players.uid`, which is their Steam64 ID
(what `getPlayerUID` returns in-game — see [DATA_CONTRACT.md](DATA_CONTRACT.md)). The website
reuses it instead of inventing a separate login/registration system:

- Login is **"Sign in with Steam"** via Steam's OpenID 2.0 endpoint (Steam does not offer OAuth2
  for this — it's OpenID 2.0 specifically; **verify the exact endpoint/response shape against
  Steam's own Web API docs at implementation time**, not assumed here, same verify-don't-assume
  rule this project applies to Arma commands).
- On first login, if no `players` row exists for that Steam64 ID yet, one is created (`status =
  'whitelisted_pending'` or `'active'` depending on whether the server runs whitelist-gated —
  matches the existing `players.status` check constraint, no schema change needed).
- Staff use the **exact same login**. There is no separate staff auth system — a staff member is a
  player whose `players.staff_rank_id` is set. Panel access is resolved *after* login, not via a
  different login path (see §4).

No passwords are ever stored by this app — Steam owns the credential.

**Discord is a second, linked identity, not a second way to create an account.** A player can
connect their Discord account two ways — from the website (`/auth/discord/connect`, real OAuth2
authorization-code flow, since Discord actually offers OAuth2 unlike Steam) or from Discord itself
(a `/link <code>` bot command redeeming a short-lived code the website generated,
`discord_link_codes` in §6) — whichever is more convenient for that player. Both paths converge on
the same `players.discord_id` column; once linked, "Sign in with Discord" becomes a valid
*additional* login method for that account (`internal/auth.FindPlayerByDiscordID`). It can never be
the *first* login for a brand-new visitor, though — only Steam creates a `players` row, since that
row's identity is Steam64-keyed. A Discord login attempt with no matching `players.discord_id`
tells the visitor to sign in with Steam first, not silently create a Discord-only account that
would have no `uid` to ever attach in-game data to.

## 4. Panel access model

Extends the rank system from [ADMIN_TOOLS.md §3](ADMIN_TOOLS.md#3-data-model) rather than
building a parallel one.

**`staff_ranks`** gains two columns (schema addition, §6):

| Column | Type | Notes |
|---|---|---|
| default_admin_panel | boolean, default false | Does this rank get Admin Panel access by default? |
| default_support_panel | boolean, default false | Does this rank get Support Panel access by default? |

Per-player exceptions reuse the **existing** `staff_permission_overrides` table
([ADMIN_TOOLS.md §7](ADMIN_TOOLS.md#7-per-command-overrides)) rather than a new override table —
`command_key = 'panel.admin'` or `'panel.support'`, `allow = true/false` grants or revokes past the
rank default. A moderator who only does support work gets `('panel.support', true)`; a head admin
who should stay out of the ticket queue gets `('panel.support', false)`.

Resolved access = `(rank default) XOR-overridden-by (player override, if one exists)`, computed once
at login and cached on the session row (§6) — not re-derived on every request, but re-derived on
every new login, so a rank/override change takes effect next sign-in, same latency as the in-game
admin menu already accepts for rank changes.

**UI**: a player with only Admin access lands in `/admin`; only Support access lands in `/support`;
both lands on a small chooser, with a switcher persistently available in that panel's nav (not
buried in settings) — this is the concrete answer to "switch between without problems."

## 5. Public site & member portal

**Public (no login):**
- Landing page — server pitch, rules summary, connect info, Discord invite link.
- Status strip — reuses `server_manager`'s dashboard queries (player count, uptime) so the public
  page and the operator's desktop dashboard never show different numbers from two code paths.

**Member portal (login required, plain player):**
- **Stats** — per-faction (civilian/police/medic) level, cash, bank balance, licences, playtime.
  Read-only, straight from `players`.
- **Gang** — **built** (`internal/gang/gang.go`). If `gang_members` has a row for this player: view
  roster, gang bank balance (`gang_accounts`), gang log (`gang_log`). The gang's leader
  (`gangs.leader_player_id` — the only permission tier this checks; `gang_members.rank` has no
  CHECK constraint in the schema, so a rank label like "officer" is organizational, not a grant of
  these actions) additionally gets:
  - **Invite a member** — by exact (case-insensitive) name, via the shared
    `internal/playerlookup` resolver also used by bank transfers (§ below). Writes `gang_members`,
    logs to `gang_log` (`action = 'member_added'`), same table the in-game gang system will use.
    `gang_members.player_id` is `UNIQUE` (a player belongs to at most one gang at a time), so
    inviting someone already in a gang is rejected with a specific message, not a raw constraint
    error.
  - **Remove a member** / **change rank** (`member`/`officer`) — same pattern, same `gang_log`
    action trail. A leader can't remove or re-rank themselves this way — leaving/disbanding a gang
    is a different, not-yet-built action.
  - Gang *creation* is not built — these actions manage an existing gang's membership only.
- **Send money** — **built** (`internal/bank/transfer.go`). Bank transfer between the player's own
  faction accounts, or to another player identified by exact (case-insensitive) name — ambiguous or
  unknown names are rejected rather than guessed at, since routing real money to the wrong account
  on a bad guess is exactly the mistake worth an extra rejection to prevent. Writes through
  `bank_accounts`/`bank_transactions` **exactly like the C++ extension is designed to** — same
  idempotency-token pattern from [DATA_CONTRACT.md](DATA_CONTRACT.md) (a request token per
  transfer, unique-indexed on `(account_id, request_token)`, generated fresh each time the transfer
  form is rendered so a double-click or back-button resubmit can't double-spend), plus row-level
  locking (`SELECT ... FOR UPDATE`, both accounts, fixed ascending-ID order to avoid deadlocking a
  concurrent transfer touching the same two accounts in the opposite direction) so a concurrent
  transfer against the same account can't read-then-overwrite a stale balance. The website is
  *another writer* against the same authoritative ledger, not a parallel money system — this is why
  `bank_accounts.balance` being authoritative (not `players.*_bank`) mattered in the original schema
  design; it's what makes a second writer safe at all. Verified end-to-end (own-account transfer,
  player-to-player transfer, insufficient-funds rejection, unknown-recipient rejection, and a
  resubmitted-token replay all producing the correct ledger with no double-apply) against a real
  local Postgres instance.
- **Wanted status** — outstanding `wanted_crimes` for this player, read-only.

**Resolved (was an open question):** does `fn_save.sqf` ever write `players.*_bank` directly, which
would let a web-initiated transfer get overwritten by a later in-game autosave? **No** —
`fn_save.sqf`'s field allowlist has no `civ_bank`/`cop_bank`/`medic_bank` entries at all (only
`*_cash` is saveable), and `src/cpp_extension/src/db.cpp` confirms `bank_accounts` is only ever
seeded to 0 at player creation and read at load, never written by "save". There is currently no
live in-game write path this feature could race against. One thing this did surface and fix: a
player who signs up on the website *before* ever connecting in-game had no `bank_accounts` rows at
all (only the C++ extension's first-load path seeded them) — `FindOrCreatePlayerBySteamUID` now
seeds the same three rows a new player gets in-game, so website-first signup works too.

## 6. New / changed schema

All additions to `database/schema.sql`, in the same file (no migration framework in this project
yet — see [database/README.md](../database/README.md)).

**`staff_ranks`** — add `default_admin_panel boolean not null default false`,
`default_support_panel boolean not null default false` (§4).

**`players`** — add `discord_id text unique` (nullable) and `discord_username text` (nullable,
display cache only — `discord_id` is the durable key). Set by either Discord-linking path in §3.

**`discord_link_codes`** *(new)*

| Column | Type | Notes |
|---|---|---|
| code | text PK | short, human-typeable (no ambiguous characters) |
| player_id | FK → players | who generated it, from the website |
| created_at | timestamptz | |
| expires_at | timestamptz | 10 minutes, matching the round-trip this is meant for |
| used_at | timestamptz, nullable | null = unredeemed; set atomically on redemption so a code can't be replayed |

**`web_sessions`** *(new)*

| Column | Type | Notes |
|---|---|---|
| id | bigserial PK | |
| player_id | FK → players | |
| token_hash | text, unique | SHA-256 of the cookie token — the raw token is never stored, same reasoning as a password reset token |
| admin_panel_access | boolean | resolved at login per §4, cached for the session's lifetime |
| support_panel_access | boolean | ditto |
| ip_address | inet | |
| user_agent | text | |
| created_at | timestamptz | |
| last_seen_at | timestamptz | |
| expires_at | timestamptz | |

Deleting a row logs that session out immediately — a ban handler or a rank change can delete a
player's `web_sessions` rows as part of the same transaction, same "revoke now, not eventually"
property staff already expect from in-game kicks.

**`ticket_categories`** *(new)*

| Column | Type | Notes |
|---|---|---|
| id | serial PK | |
| key | text, unique | globally unique by construction — a subcategory's key is prefixed with its parent's (e.g. `gameplay_bug`), not scoped per-parent, since `UNIQUE(parent_id, key)` wouldn't stop two top-level rows colliding (Postgres treats every `NULL` `parent_id` as distinct) |
| label | text | display name |
| parent_id | FK → ticket_categories, nullable | `NULL` = top-level category; set = a subcategory of that row |
| sort_order | integer | display order within the same parent |

A self-referential lookup table, not a hardcoded list — a support team realistically wants to
add/rename categories over time without a code deploy, same reasoning `staff_ranks`/
`arsenal_item_pools` were made DB-driven. Seeded with Gameplay, Discord, Panel, TeamSpeak, and
Other as top-level categories, each (except Other) with a handful of subcategories — see
`database/schema.sql`'s seed `INSERT`s for the exact list.

**`support_tickets`** *(new)*

| Column | Type | Notes |
|---|---|---|
| id | bigserial PK | |
| player_id | FK → players | who opened it |
| subject | text | |
| status | text, check | `open` / `pending` / `closed` |
| priority | text, check, default `normal` | `low` / `normal` / `high` / `urgent` — §8 |
| category_id | FK → ticket_categories | must reference a **top-level** row (`parent_id IS NULL`) — enforced in application code (`internal/handlers/categories.go`'s `validateCategoryPair`), not a DB constraint, since a `CHECK` can't reference another row |
| subcategory_id | FK → ticket_categories, nullable | if set, must be a child of `category_id` — same validation function |
| assigned_staff_id | FK → players, nullable | claimed by, null = unclaimed |
| discord_thread_id | text, nullable | Discord thread this ticket mirrors to, §9 |
| created_at | timestamptz | |
| updated_at | timestamptz | |
| closed_at | timestamptz, nullable | |

**`support_ticket_messages`** *(new)*

| Column | Type | Notes |
|---|---|---|
| id | bigserial PK | |
| ticket_id | FK → support_tickets | |
| author_player_id | FK → players, nullable | null = system message (e.g. "ticket closed") |
| body | text | |
| source | text, check | `web` / `discord` — which side this message originated on |
| internal | boolean, default false | staff-only note, never shown to the requester or mirrored to Discord — §8 |
| discord_message_id | text, nullable | for edit/delete mirroring, if implemented |
| created_at | timestamptz | |

No changes needed to `players`, `gangs`, `gang_members`, `bank_accounts`, or `bank_transactions` —
the member portal and admin panel are additional *writers* against those tables using the exact
column shapes already defined, not a reason to reshape them.

## 7. Admin Panel

Web-equivalent of the in-game admin menu ([ADMIN_TOOLS.md](ADMIN_TOOLS.md)) for the things that are
awkward to do from inside the game — bulk lookups, long-form investigation, typing reasons longer
than a dialog field comfortably allows:

- Player lookup (by UID or name, via `player_aliases`) — profile view: full stats, `player_log`,
  `player_sessions`, `staff_log` entries where they're the target, `kick_log`, `anti_cheat_flags`.
- Ban / unban — writes `banlist`, exactly the table the game's BE/whitelist check already reads.
- Rank management — assign/change `staff_ranks`, grant/revoke `staff_permission_overrides` —
  this is the **web UI for the DB-driven system ADMIN_TOOLS.md §2 already assumes exists**; today
  that system is edited by hand via `psql`, which doesn't scale past the first few staff.
- Anti-cheat flag review queue — `anti_cheat_flags` where `reviewed_at IS NULL`, same triage
  surface as [ANTI_CHEAT.md](ANTI_CHEAT.md) Layer 4, easier to work through as a filterable table
  than one at a time in-game.
- Arsenal pool / loadout preset editor — `arsenal_item_pools`, `arsenal_loadout_presets`.
- Staff log viewer — full `staff_log`, filterable by staff member or target, the audit trail for
  everything above.

Every write from this panel inserts into `staff_log` the same way the in-game admin menu does
(`staff_player_id`, `action`, `before_value`/`after_value`) — one unified audit trail regardless of
whether the action happened in-game or on the web, not two logs staff have to cross-reference.

## 8. Support Panel

Built out as a proper IT-support-portal, not a bare table — its own two-page structure with a
persistent side nav (`/support` Dashboard, `/support/tickets` Tickets), matching how real IT
ticketing systems separate an at-a-glance overview from the full working queue rather than
cramming both into one page:

- **Dashboard** (`/support`) — the stats bar plus the 5 most recently opened tickets. An overview
  to land on, not the working queue.
- **Tickets** (`/support/tickets`) — the full filterable/sortable queue (below).
- The side nav (`partial_support_sidebar.html`, a shared template partial — see
  `internal/render`'s partial-loading support) also appears on the ticket detail page for a staff
  viewer, so the workspace navigation stays visible while drilling into a specific ticket; a
  player viewing their own ticket never sees it at all, since that partial is only rendered inside
  the `{{if .IsStaff}}` branch of `ticket_thread.html`.

The concrete features that distinguish this from a plain forum thread:

- **Priority** (`support_tickets.priority`: `low`/`normal`/`high`/`urgent`) — the submitter picks
  one when opening a ticket (their own sense of urgency, not a locked-in SLA commitment); staff can
  re-triage it from the ticket page once they've actually looked. The queue sorts by priority first
  (urgent → high → normal → low), then age within each tier.
- **Queue stats** — Open, Unassigned, Assigned to Me, Urgent counts at the top of the panel, so
  triage priorities are visible before scrolling any list.
- **Categories and sub-categories** (`ticket_categories`, above) — top-level: Gameplay, Discord,
  Panel, TeamSpeak, Other; Gameplay/Discord/Panel/TeamSpeak each break down further (Gameplay, for
  instance, into Bug Report / Player Report / Ban Appeal / Whitelist Application / Economy Issue /
  Vehicle-Property Issue). The new-ticket form's sub-category `<select>` is populated by a small
  inline script filtering a JSON array embedded in the page by the chosen category — no page
  reload, no framework, consistent with this app's "plain server-rendered HTML plus a sprinkle of
  vanilla JS where it genuinely helps" approach elsewhere (e.g. the Performance tab's charts).
- **Filters** — status (open+pending / open only / pending only / closed / all), priority,
  category (top-level only), and assignment (everyone / assigned to me / unassigned), plus a
  **search** box (subject, player name, or Steam UID — one bound parameter reused across all three
  `ILIKE` clauses, not three separately-trusted inputs), via plain query-string GETs — no
  client-side filtering, consistent with this app having no SPA framework anywhere else.
- **Requester identity** — the queue, dashboard, and ticket detail page all show the requester's
  Steam64 UID (`players.uid`) alongside their name; the ticket detail page additionally shows
  their linked Discord (username + ID, or "Not linked") in a dedicated Requester panel — staff
  need this to cross-reference bans, Discord reports, and in-game identity without leaving the
  ticket.
- **Ticket detail page** — a two-column layout for staff: conversation + reply on the left, a
  metadata sidebar on the right — a read-only properties view by default, an explicit **Edit**
  button to reveal the editable form controls (see below), the Requester identity block, and
  Close/Reopen. A player viewing their own ticket gets the conversation only, no metadata panel
  (staff-only information and staff-only edits stay staff-only).
- **Editable ticket data** — staff can correct/re-triage a ticket directly from the detail page,
  each field its own small form re-validated server-side (never trusting that the page's own
  `<select>` options or JS were the ones a request actually came from). The properties panel opens
  in a read-only view (plain text/badges, matching what a requester sees); clicking **Edit** swaps
  in the live controls below, and **Done editing** swaps back — a pure client-side visibility
  toggle (`hidden` attribute, no page reload), so a staff member just reading a ticket can't change
  its data with a stray click or keystroke. Close/Reopen stay outside the edit gate as one-click
  workflow actions rather than field edits.
  - **Subject** — a text field + Save button, for when a player's own title doesn't actually
    describe the issue.
  - **Category / Sub-category** — the same cascading selects as ticket creation, pre-filled with
    the current values, re-checked through the identical `validateCategoryPair` the create path
    uses.
  - **Assigned to** (`assigned_staff_id`) — a single dropdown listing every player currently
    resolved as Support-Panel-eligible (rank default + per-player override, the exact same
    resolution `internal/auth/session.go` uses at login — re-derived fresh here, not cached),
    replacing separate "claim for myself"/"unassign" actions with one control that covers
    claiming, reassigning to someone else, and releasing back to the queue. Auto-submits on
    change. Assigning bumps `open` → `pending` (an assigned ticket nobody's looked at yet reads
    oddly as "open"); reassigning an already-`pending` ticket leaves its status alone.
  - **Priority** — unchanged from before, a select that auto-submits.
- **Internal notes** (`support_ticket_messages.internal`) — a staff-only comment on a ticket, never
  shown to the requester and never mirrored to Discord. Filtered out **in the SQL query itself**
  for a non-staff viewer (`WHERE ... AND (NOT internal OR $isStaff)`), not just hidden by the
  template — the content never reaches the page as hidden markup a curious player could inspect.
  Can only be created by a request that already has Support Panel access; a raw POST from a
  player's own session can't set the flag no matter what it submits, since the handler re-derives
  "is this submitter staff" from the session, not from the form.
- **Thread view** — priority/status badges, requester name (staff only), reply box, close/reopen,
  the internal-note toggle above.
- A player's own **My Tickets** view lives in the member portal (§5), not here — a ticket is one
  row viewed from two different access levels (owner vs. staff), not two separate objects.

## 9. Discord integration

Two different integrations, deliberately built differently because they solve different problems:

**One-way logs (webhooks — simple, no bot process needed):**
Staff actions (`staff_log` inserts), bans, anti-cheat flags crossing `high` confidence, and new
support tickets each fire a webhook POST to a configured Discord channel webhook URL. Fire-and-
forget from the Go handler — a webhook failure must never block or fail the underlying action (a
ban still applies even if Discord is down); log the webhook error, don't propagate it.

**A real bot (discordgo — genuinely needs a persistent connection), for two things:**
A webhook can only post *into* Discord, never read anything back out — a bot (gateway websocket
connection) is unavoidable both for account linking and for two-way ticket sync:
- **Account linking (`/link <code>`) — built.** The website generates a short-lived code
  (`discord_link_codes`, §6); the bot's slash-command handler redeems it and sets
  `players.discord_id` for whichever player generated it, replying ephemerally with the result. See
  §3 for why this exists alongside the OAuth2 "Connect Discord" path rather than instead of it.
- **Ticket-thread sync — designed, not yet built.** New ticket → bot creates a thread in a
  configured support category, posts the opening message, stores the thread ID in
  `support_tickets.discord_thread_id`. New reply on the web → bot posts it into the thread. New
  message in that thread on Discord → bot writes a `support_ticket_messages` row
  (`source = 'discord'`). This is the next piece to layer onto the same bot process, not a second
  bot — see `internal/discord/bot.go`'s trailing TODO.

The bot runs as a goroutine inside the same website binary (one process to deploy, matching §1's
reasoning) rather than a separate service — `discordgo`'s gateway client is designed to run
alongside a normal Go program, not as a standalone daemon. It is optional at runtime: an unset
`DISCORD_BOT_TOKEN` disables it (logged, not fatal) and the site runs fine without it — Steam login
and the website-generated link code still work; only the `/link` command's redemption side and any
future ticket sync are unavailable.

Config (bot token, guild ID, ticket category ID, per-log-type webhook URLs) is external
configuration, not hardcoded — same `.ini`-or-env pattern as `config.ini` for the C++ extension and
`settings.json` for server_manager. A bot token is a credential; it must never be committed, same
rule as the Postgres credentials already `.gitignore`d.

## 10. Security notes

- CSRF token on every state-changing form (transfers, gang actions, admin/support writes) — **built**
  (`internal/csrf`, double-submit cookie pattern: a random token in an `HttpOnly` cookie, echoed into
  every rendered form, required to match on every POST). This app has real financial actions (bank
  transfers) reachable from a browser session, unlike the in-game admin menu which has no
  cross-site attack surface at all.
- Rate limiting on login and on money-moving endpoints — **not yet built**. Separate from
  [ANTI_CHEAT.md](ANTI_CHEAT.md)'s in-game rate limiting (different attack surface — this is HTTP
  requests, not `callExtension` calls) but the same *principle*: repeated identical requests in a
  short window are suspicious before they're proven legitimate.
- Session cookies: `HttpOnly` (no JS access — mitigates XSS token theft), `Secure` (HTTPS only),
  `SameSite=Lax`.
- All admin/support writes require the CSRF token **and** re-check panel access server-side on
  every request, not just at login — a revoked override must take effect on the *next request*,
  not the next login, for anything actually destructive (ban, rank change). This is stricter than
  §4's "resolved at login" for read access; §4 covers *which panel loads*, this covers *whether a
  specific write is still allowed right now*.
- User-supplied text (ticket bodies, player-chosen display text) is never trusted into `html/template`
  as pre-escaped — no `template.HTML()` wrapping of anything that touched user input.

## 11. Timeline impact — the honest part

This is a genuinely large addition — a full auth system, four distinct UI surfaces, a money-moving
write path against the live economy ledger, and a persistent Discord bot process — not a small
bolt-on. Built to the scope above rather than deferred, this doesn't fit inside the existing Phase
4–6 window without pushing it.

Treat this as a **parallel track**, not sequential with Phase 3 (Anti-Cheat & Admin Tooling): it
shares almost no code with the SQF/C++ side, so it can be built concurrently rather than after. But
the realistic engineering time — auth, four panels, the ledger-safe transfer path, the bot — is
measured in weeks, not days, and **[ROADMAP.md](ROADMAP.md)'s dates will need another honest revision**
once this work is underway and better understood, the same way ADMIN_TOOLS.md's scope increase
pushed the date from `2026-12-26` to `2027-01-20`. Not doing that revision now, in this doc — doing
it in ROADMAP.md itself once Phase 3 work clarifies how much runs in parallel versus how much
contends for the same reviewer/tester time.

## 12. Explicitly out of scope (for now)

- Real-money payments (donations/store) — no payment processor integration here; if this project
  ever wants that, it's its own security-reviewed doc, not folded into this one.
- Mobile app — responsive web only.
- Public API for third-party tools — the member portal is the only sanctioned external surface for
  now.
