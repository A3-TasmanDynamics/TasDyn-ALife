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
- **Gang** — if `gang_members` has a row for this player: view roster, gang bank balance
  (`gang_accounts`), gang log (`gang_log`). Leader/officer ranks additionally get:
  - **Invite a member** — writes `gang_members`, logs to `gang_log` (`action = 'member_added'`),
    same table the in-game gang system already writes.
  - **Remove a member** / **change rank** — same pattern.
- **Send money** — bank transfer between the player's own faction accounts, or to another player.
  Writes through `bank_accounts`/`bank_transactions` **exactly like the C++ extension does** — same
  idempotency-token pattern from [DATA_CONTRACT.md](DATA_CONTRACT.md) (a request token per
  transfer, unique-indexed on `(account_id, request_token)`), so a double-click or a retried request
  can't double-spend. The website is *another writer* against the same authoritative ledger, not a
  parallel money system — this is why `bank_accounts.balance` being authoritative (not
  `players.*_bank`) mattered in the original schema design; it's what makes a second writer safe at
  all.
- **Wanted status** — outstanding `wanted_crimes` for this player, read-only.

**Open question, flag before this ships (§11):** while a player is actively connected in-game, does
`fn_save.sqf` ever write `players.*_bank` directly? If it does, a web-initiated transfer landing
between two in-game autosaves could get its effect overwritten on the next save, since `*_bank` is
documented as a *cache* synced by trigger from `bank_accounts` (schema.sql line ~49), not something
`save` should be setting absolutely. **Verify `fn_save.sqf` never treats `*_bank` as an
absolute-set field before enabling web-initiated transfers against a live session** — this is a
correctness check against the existing DATA_CONTRACT, not new design.

## 6. New / changed schema

All additions to `database/schema.sql`, in the same file (no migration framework in this project
yet — see [database/README.md](../database/README.md)).

**`staff_ranks`** — add `default_admin_panel boolean not null default false`,
`default_support_panel boolean not null default false` (§4).

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

**`support_tickets`** *(new)*

| Column | Type | Notes |
|---|---|---|
| id | bigserial PK | |
| player_id | FK → players | who opened it |
| subject | text | |
| status | text, check | `open` / `pending` / `closed` |
| category | text | e.g. `billing`, `report`, `bug`, `appeal` — free-standing list, not FK'd to anything else |
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

- **Queue** — open/pending tickets, filterable by category, assigned/unassigned.
- **Claim** — sets `assigned_staff_id`.
- **Thread view** — `support_ticket_messages` for a ticket, reply box, close/reopen.
- A player's own **My Tickets** view lives in the member portal (§5), not here — a ticket is one
  row viewed from two different access levels (owner vs. staff), not two separate objects.

## 9. Discord integration

Two different integrations, deliberately built differently because they solve different problems:

**One-way logs (webhooks — simple, no bot process needed):**
Staff actions (`staff_log` inserts), bans, anti-cheat flags crossing `high` confidence, and new
support tickets each fire a webhook POST to a configured Discord channel webhook URL. Fire-and-
forget from the Go handler — a webhook failure must never block or fail the underlying action (a
ban still applies even if Discord is down); log the webhook error, don't propagate it.

**Two-way ticket sync (a bot, via discordgo — genuinely needs a persistent connection):**
A webhook can only post *into* Discord, never read replies back out — a real bot (gateway
websocket connection) is unavoidable for staff to be able to reply from Discord and have it land
back in the ticket:
- New ticket → bot creates a thread in a configured support category, posts the opening message.
  `support_tickets.discord_thread_id` stores the thread ID.
- New reply on the web → bot posts it into the thread.
- New message in that thread on Discord → bot writes a `support_ticket_messages` row
  (`source = 'discord'`) and the web ticket view shows it, next poll/refresh.

The bot runs as a goroutine inside the same website binary (one process to deploy, matching §1's
reasoning) rather than a separate service — `discordgo`'s gateway client is designed to run
alongside a normal Go program, not as a standalone daemon.

Config (bot token, guild ID, ticket category ID, per-log-type webhook URLs) is external
configuration, not hardcoded — same `.ini`-or-env pattern as `config.ini` for the C++ extension and
`settings.json` for server_manager. A bot token is a credential; it must never be committed, same
rule as the Postgres credentials already `.gitignore`d.

## 10. Security notes

- CSRF token on every state-changing form (transfers, gang actions, admin/support writes) — this
  app has real financial actions (bank transfers) reachable from a browser session, unlike the
  in-game admin menu which has no cross-site attack surface at all.
- Rate limiting on login and on money-moving endpoints, separate from
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
