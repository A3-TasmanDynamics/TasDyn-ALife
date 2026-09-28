# Discord Bot

The complete plan for the TasDyn-ALife Discord bot: what it does, how it's built, and in what
order. [INTEGRATIONS.md](INTEGRATIONS.md) covers *what* each website feature sends to Discord
(and TeamSpeak). This doc covers the bot itself: its commands, buttons, events, channel layout,
reliability and security.

**Built so far (stages B0-B2, §11):** the command registry and button routing, `/admin/discord`
settings, `/link` `/unlink` `/profile` `/status` `/players`, staff `/whois` (and the *Whois* context
menu) `/lookup` `/promote` `/demote` `/loa` `/reinstate` `/sync`, `/bot health`, the #welcome
message, rejoin and drift handling, logging of moderation done in Discord, and the live
#server-status message with presence. Not yet: `/rules` (the site has no rules pages yet) and
everything from B3 on (tickets, discipline, applications, factions).

## 1. Principles

1. **One bot, inside the website.** It keeps running as a goroutine in the website binary
   ([WEBSITE.md §9](WEBSITE.md#9-discord-integration)), so it shares the database pool, the
   permission checks and the business logic. It never reimplements a rule the website already
   enforces.
2. **The bot is another way into the website, not a second system.** `/ban` calls the same Go
   function as the website's Ban button, with the same checks, the same `staff_log` row and the
   same Discord/TeamSpeak sync. If a rule changes, it changes in one place.
3. **Permission comes from the linked website account, never from Discord roles.** Every staff
   command looks up the caller's linked player, their staff rank, status and permission
   overrides. Someone handing themselves a role in Discord gains nothing. Suspended or LOA staff
   are denied, just as they are on the website.
4. **Private by default.** Anything showing player data (whois, lookups, balances, case details)
   is replied to *ephemerally*, so only the caller sees it. Public posts carry only names, numbers
   and links.
5. **The site never depends on the bot.** If the bot is down, the website and game keep working.
   Messages queue and are sent when it's back (§8).

## 2. Architecture

```
internal/discord/
  bot.go          lifecycle: connect, register commands, health, shutdown
  registry.go     declarative command table: name, options, required permission, handler
  perms.go        caller -> linked player -> permission check (reuses internal/auth)
  components.go   button / select-menu / modal routing by custom_id prefix
  events.go       member join/leave/update, thread messages, audit-log entries
  outbox.go       reliable delivery worker for DMs and channel posts (§8)
  rolesync.go     Discord adapter for the shared role-sync engine (INTEGRATIONS §2.3)
  tickets.go      ticket <-> forum-thread sync (§6)
  status.go       live status message + bot presence (§7)
  cmd_*.go        one file per command group (player, staff, cases, factions, admin)
```

- **Command registration:** on startup, all commands are registered in one *bulk overwrite* for
  the guild. That's idempotent, so commands that were removed from the code disappear and new ones
  appear. The current "delete `/link` on shutdown" behaviour is dropped: it made the command
  vanish during every restart.
- **Handlers are thin.** Each one parses options, runs the permission check, calls into the same
  service function the website uses (e.g. `bans.Issue(ctx, actor, target, …)`), and formats the
  reply. There's no SQL in the command files.
- **Slow actions** (ban, sync) use Discord's *deferred* reply ("Bot is thinking…"), so the
  3-second interaction limit is never hit.
- **Testability:** handlers take a small interface over the discordgo session. That way they can
  be unit-tested against a fake, and a separate **staging guild and bot token** is used for manual
  testing, never the live server.

## 3. Server layout the bot expects

Channels and roles are **configured in the admin panel** (a new *Discord settings* page with
channel and role pickers filled from the real guild), stored in a `discord_settings` table.
Nobody pastes IDs into `.env`. Any channel left unset means that feature simply doesn't post.
The bot posts directly to these channels; the webhook env vars from INTEGRATIONS §2.4 are only a
fallback for when the bot isn't running.

| Area | Channel | Used for |
|---|---|---|
| Public | `#welcome` | Onboarding message with the **Link account** button (§5.1). |
| | `#rules` | Mirror of the public rules pages. |
| | `#announcements` | Announcements, policy changes, devlog posts. |
| | `#server-status` | Live status message + up/down posts (§7). |
| Support | `#tickets` (**forum channel**, staff-only) | One forum post per support ticket (§6). |
| Staff | `#staff-log` | Every staff action and rank change, plus drift reports. |
| | `#bans`, `#cases`, `#applications` | Posts with action buttons (§4.4). |
| | `#bot-admin` | Sync problems, bot errors, missing-permission warnings. |
| Factions | `#police-command`, `#ems-command` | Roster changes, faction applications. |

## 4. Commands

**Visible** is who can see and use the command. Staff commands are also hidden in Discord from
people without a staff role (using Discord's default member permissions). That's only a
convenience: the real check is always the linked account (principle 3).

### 4.1 Everyone

| Command | What it does |
|---|---|
| `/link <code>` | *(built)* Link your Discord to your website account. |
| `/unlink` | Unlink; synced roles are removed (confirmation button first). |
| `/profile` | Your linked account: in-game name, factions and ranks, active warning points, links. Ephemeral. |
| `/status` | Server status: game server players, each service's state. |
| `/players` | Online count and faction breakdown. Not names: who's online isn't public. |
| `/rules [section]` | Rules page link, or one section's text. |
| `/ticket` | Opens a **modal** (category, subject, details) and creates a real support ticket. It shows up on the website and in the staff forum, same as one made on the website. |
| `/apply <staff\|police\|ems>` | Link to the website application form (applications need the full form and Steam checks, so they stay on the website). |
| `/balance` | Your bank balance. Ephemeral. *(See §12, decision 4.)* |

Context menu **"Report message"** (right-click any message → Apps): opens a ticket in the
*Discord → Report a member* category with a link to the message and its text, so the evidence is
captured even if the message is later deleted.

### 4.2 Staff

| Command | Permission | What it does |
|---|---|---|
| `/whois @user` · context menu "Whois" | `players.view` | Linked Steam profile, BattlEye GUID, TeamSpeak identities, staff/faction status, active points, open cases, recent bans. Ephemeral. |
| `/lookup <steam64 \| profile URL \| GUID \| name>` | `players.view` | Same view, for players who aren't on Discord. |
| `/case new @user` · context menu "Open case" | `cases.create` | Modal → new case with that user as subject. Replies with the case link. |
| `/case note <id>` | `cases.create` | Modal → appends a case entry. |
| `/case view <id>` | `cases.view` | Case summary. Ephemeral. |
| `/warn @user <points> <rule>` | `cases.create` | Issues punishment points (on a new or given case). The player is DMed. |
| `/ban @user` | `bans.issue` (+ `bans.permanent`) | Modal: reason, length, case, Discord/TeamSpeak ban tick boxes. Then a **confirm** button showing exactly what will happen. |
| `/unban <player>` | `bans.revoke` | Same confirm flow. |
| `/promote` · `/demote @user <rank>` | `staff.edit` | Staff rank change through the promotion pipeline (INTEGRATIONS §3). Modal for the reason. |
| `/setrank @user <police\|ems> <level>` | faction command or `players.edit_*` | Faction rank change, same pipeline. |
| `/loa @user <until> <reason>` · `/reinstate @user` | `staff.loa` | LOA on/off. |
| `/tickets` | Support Panel | Your assigned and unassigned ticket counts, with links. |

### 4.3 Senior staff / Head Admin

| Command | Permission | What it does |
|---|---|---|
| `/announce` | `announce.post` | Modal → posts to `#announcements` (and, once the in-game bridge exists, broadcasts in-game). |
| `/sync @user` · `/sync all` | `roles.manage` | Force a role/group reconcile now and show what changed. |
| `/restart <server> <minutes>` | `server.control` | The restart flow from GAMEPANEL_PARITY §8.1, with a typed-confirmation modal. |
| `/bot health` | `roles.manage` | Gateway latency, outbox backlog, last sync pass, missing permissions. |

### 4.4 Buttons on staff posts

Posts in staff channels carry action buttons, so routine work doesn't need the website open:

- `#applications`: **Accept · Interview · Reject** (reason modal on Reject).
- `#bans`: **Open case · Unban**.
- `#cases`: **View · Add note · Close**.
- Faction command channels: **Accept · Reject** on faction applications.
- `#bot-admin` sync problems: **Retry now**.

Each button's `custom_id` holds only an action and a record ID (e.g. `app:accept:482`). The
**permission is checked again on click**, against the clicker's linked account, and a record
that has already been handled replies "Already handled by X". The button itself is never trusted.

## 5. Events

### 5.1 Onboarding (member joins)

1. The bot posts in `#welcome` (or sends a DM, per settings): "Link your Steam account to get
   access", with a **Link account** button. The button opens the website's Discord OAuth connect
   flow; `/link` with a code also works.
2. After linking, role sync gives the **Verified** role plus whatever their account entitles them
   to (staff, faction). Anyone who rejoins gets their roles back automatically, because the
   website is the source of truth.
3. The recommended Discord setup is that `@everyone` can only see `#welcome`, `#rules` and
   `#announcements`, and Verified can see the rest. That uses Discord's channel permissions, not
   bot code.

### 5.2 Role drift (member updated)

When a member's roles change, the bot checks the change against the mapped roles *immediately*,
instead of waiting for the 15-minute pass. It reverts any mapped role that doesn't match their
entitlements and reports it (INTEGRATIONS §3.3).

### 5.3 Discord-side moderation gets logged

The bot listens to the guild **audit log**. When staff kick, ban or time someone out *in Discord
directly*, a `staff_log` row is written (`source = 'discord'`) for the linked staff member and
target where known, and posted to `#staff-log`. Moderation done inside Discord is recorded just
like website actions. This is what makes "everything is logged" true beyond the website.

### 5.4 Member leaves

Nothing is deleted. Their link and history stay, and roles are reapplied if they return.

## 6. Support tickets ↔ Discord forum

This is the two-way sync already designed in [WEBSITE.md §9](WEBSITE.md#9-discord-integration),
made concrete:

- **New ticket** (website, `/ticket`, or "Report message") → a new post in the staff-only
  `#tickets` **forum channel**. Title: `#123 · Subject`. Forum **tags** hold the category and
  priority, so staff can filter in Discord. The opening post has **Claim · Priority · Close**
  buttons. `support_tickets.discord_thread_id` stores the link.
- **Staff reply in the forum thread** → copied to the ticket as a staff reply (`source =
  'discord'`), shown to the player on the website, and DMed to them. A message starting with
  **`!note`** is stored as an *internal note* instead, which the player never sees. The bot reacts
  ✅ on each message it copies, so staff can see it went through.
- **Reply on the website** (player or staff) → posted into the thread under the author's name.
- **Status changes** (claim, priority, close, reopen) from either side are reflected on the other:
  the tags update, and closing a ticket archives the thread.
- **Players never see the forum.** They use the website, or (see §12, decision 2) reply to the
  bot's DM.
- **Requires** the privileged **Message Content** intent. It's needed only to read staff messages
  in ticket threads; the bot ignores message content everywhere else.

## 7. Server status in Discord

- **One live message** in `#server-status`, edited every minute with each service's state and the
  game server's player count. It's a single message, not a stream of posts.
- **Up/down posts:** a separate post when something goes down or comes back, so people get
  notified (the live message edits don't trigger notifications).
- **Bot presence:** "Watching 12/64 on Altis", taken from the game-server A2S check. It shows
  "Server offline" when down.

## 8. Reliable delivery (the outbox)

Webhooks and direct sends are fire-and-forget, which is fine for logs but not for things a player
must receive, like a ban notice or an application decision. Anything that matters goes through a
database outbox:

```sql
CREATE TABLE discord_outbox (
    id            BIGSERIAL PRIMARY KEY,
    kind          TEXT NOT NULL CHECK (kind IN ('dm', 'channel_post')),  -- more kinds added as features need them
    target        TEXT NOT NULL,            -- user ID, channel ID or thread ID
    payload       JSONB NOT NULL,           -- message content / embed / buttons
    dedupe_key    TEXT UNIQUE,              -- e.g. 'ban-notice:9123' so a retry never double-sends
    attempts      INTEGER NOT NULL DEFAULT 0,
    next_attempt  TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at       TIMESTAMPTZ,
    gave_up_at    TIMESTAMPTZ,              -- permanently undeliverable, or still failing after 24h
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- The website writes the outbox row **in the same transaction** as the action (the ban, the
  decision). So if the action is saved, the message is guaranteed to be queued.
- A worker sends messages in order, retries with backoff, and respects Discord rate limits.
  Anything still failing after 24 hours shows up in `#bot-admin` and on the admin dashboard.
- **DMs closed** (Discord error 50007) → marked as permanently undeliverable for that player, not
  retried. The website shows the notification anyway.

## 9. Permissions & setup

**Developer portal:**
- Privileged intents: **Server Members** (role sync, onboarding) and **Message Content** (ticket
  threads only). Presence intent is *not* needed.
- Invite scopes: `bot` and `applications.commands`.

**Bot permissions** (no `Administrator`): View Channels, Send Messages, Send Messages in Threads,
Create Public Threads, Manage Threads (archive/tag ticket posts), Embed Links, Add Reactions, Read
Message History, Manage Roles, View Audit Log, Manage Events, and, **only if** Discord bans are
enabled, Ban Members and Moderate Members.

**Role position:** the bot's role must be above every mapped role. `/bot health` and the settings
page both warn if it isn't.

## 10. Security

- **Caller identity comes from Discord's interaction payload**, never from command arguments (the
  same property `/link` already relies on).
- **Every staff command re-checks permission and staff status** at run time, including button
  clicks on old posts.
- **Replies containing private data are ephemeral.** Mentions in bot posts are restricted
  (allowed-mentions), so a player name like `@everyone` can't ping the server.
- **User-supplied text** (ticket subjects, reasons) is sent as plain content with markdown escaped,
  never interpreted as bot instructions.
- **Rate limits per user** on player commands (e.g. `/ticket` 3 per 10 minutes) stop spam.
- **Command audit:** every staff command and button produces a `staff_log` row, because it goes
  through the same service functions as the website. Player commands are counted for metrics only.
- The token lives in env only; a leaked token is rotated in the developer portal and the site
  restarted. No other secret is stored.

## 11. Build order

Stage B0 comes first, at the start of Wave 1 with the other foundations. After that, each
command ships with its feature's wave, never ahead of the website feature it calls.

| Stage | Contents | Ships with |
|---|---|---|
| **B0. Bot core** ✅ | Registry + bulk command registration (fixes `/link` vanishing on restart), the permission helper, component routing, the outbox worker, the `discord_settings` table + admin settings page, `/bot health`. | Wave 1 foundations |
| **B1. Identity & roles** ✅ | Onboarding + Link button, `/unlink`, `/profile`, the role-sync adapter, drift detection, audit-log logging, `/whois`, `/lookup`, `/promote` `/demote` `/loa` `/reinstate`, `/sync`. | Wave 1 |
| **B2. Status & public** ✅ | Live status message, up/down posts, presence, `/status`, `/players`, `/rules` *(waits for rules pages)*. | Wave 1 (status page already exists) |
| **B3. Tickets** | Forum sync both ways, `/ticket`, "Report message", `/tickets`. | Wave 2 |
| **B4. Discipline** | `/case`, `/warn`, `/ban`, `/unban`, `#bans`/`#cases` posts with buttons, DM notices through the outbox. | Wave 2 |
| **B5. Pipeline** | `#applications` buttons, `/apply`, decision DMs, interview reminders + scheduled events. | Wave 3 |
| **B6. Factions** | `/setrank`, faction command channels + application buttons. | Wave 4 |
| **B7. Knowledge & ops** | Meeting scheduled events, `/announce`, `#rules` mirror, `/restart`, `/balance`. | Waves 5–6 |

## 12. Decisions needed

1. **Welcome: channel post or DM?** **Recommendation:** a pinned `#welcome` message with the
   button. DMs from bots to brand-new members often go unseen or get flagged.
2. **Can players reply to tickets by replying to the bot's DM?** It's convenient, but it means
   the bot reads DMs. **Recommendation:** not in B3. Players reply on the website. Revisit if
   players ask for it.
3. **Is `/players` allowed to show a faction breakdown** (e.g. "Police: 4 online")? Rival players
   could use it to time crimes. **Recommendation:** total count only.
4. **Include `/balance`?** Handy, but it encourages checking the economy out of game.
   **Recommendation:** leave it until Phase 4 economy tuning is done.
5. **Ticket forum vs. private threads in a text channel:** **Recommendation:** forum. Tags give
   staff filtering by category and priority for free.
