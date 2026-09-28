# Discord, Steam & TeamSpeak Integration Plan

How every feature in [GAMEPANEL_PARITY.md](GAMEPANEL_PARITY.md) connects to **Steam** (who the
player is, their account standing, their BattlEye identity), **Discord** (where the community and
staff talk) and **TeamSpeak 3** (where they talk in voice while playing). The aim is that staff
never have to copy a Steam ID into Discord by hand, or remember to give someone a role or server
group after promoting them. The website's database is the source of truth: **every promotion or
role change is logged once, then applied to Discord and TeamSpeak automatically**, whichever tool
the change was made from (§3).

## 1. What exists today

| | Built | Not built |
|---|---|---|
| **Steam** | "Sign in with Steam" (OpenID 2.0, `internal/auth/steam.go`); `players.uid` is the Steam64 ID. | No Steam Web API use, so no persona name, avatar, account age or VAC/game-ban data. That's why panels show "Player #35" for someone who hasn't joined the game yet. |
| **Discord** | Bot (`internal/discord/bot.go`) with `/link <code>`; OAuth2 "Connect Discord" + "Sign in with Discord" for linked accounts; ticket-created webhook. | Staff-log, ban and anti-cheat webhooks are configured in `config.go` but never called. Two-way ticket-thread sync is designed ([WEBSITE.md §9](WEBSITE.md#9-discord-integration)) but not built. No role sync, no DMs. |
| **TeamSpeak 3** | Nothing. | Everything: account linking, server-group sync, bans, status. |

The identity rules in [WEBSITE.md §3](WEBSITE.md#3-authentication--steam-not-a-new-identity-system)
don't change: **Steam creates the account; Discord and TeamSpeak are linked extra identities.**
Everything below builds on that.

## 2. Foundations (built once, used by every feature)

### 2.1 Steam Web API client

New `internal/steam` package, using a `STEAM_WEB_API_KEY` env var (server-side only, never sent to
a browser). Two calls. Endpoint shapes get verified against Valve's Web API docs at
implementation time, the same verify-don't-assume rule as everywhere else:

- **Player summaries** (`ISteamUser/GetPlayerSummaries`): persona name, avatar, profile URL, and
  account creation date *if the profile is public*.
- **Player bans** (`ISteamUser/GetPlayerBans`): VAC ban count, game ban count, days since last ban,
  community ban. This works even for private profiles.

```sql
ALTER TABLE players ADD COLUMN steam_name          TEXT;         -- display cache, never identity
ALTER TABLE players ADD COLUMN steam_avatar_url    TEXT;
ALTER TABLE players ADD COLUMN steam_created_at    TIMESTAMPTZ;  -- NULL = private profile / unknown
ALTER TABLE players ADD COLUMN steam_vac_bans      INTEGER;
ALTER TABLE players ADD COLUMN steam_game_bans     INTEGER;
ALTER TABLE players ADD COLUMN steam_days_since_ban INTEGER;
ALTER TABLE players ADD COLUMN steam_community_banned BOOLEAN;
ALTER TABLE players ADD COLUMN steam_refreshed_at  TIMESTAMPTZ;
```

Data is refreshed **on every login**, and by a background sweep for anyone not refreshed in 24
hours (both endpoints take 100 IDs per call, so the sweep is cheap). The display name everywhere
becomes: in-game name → Steam persona name → "Player #id". A **new** VAC or game ban appearing on
a refresh creates an `anti_cheat_flags` row (`flag_type` gains `'steam_ban'`), so it lands in the
existing review queue instead of being silently stored. If Steam is down or the key is missing,
cached values stay as they are; nothing fails a login or a page.

### 2.2 BattlEye GUID

BattlEye identifies players by a GUID derived from the Steam64 ID (an MD5 of `"BE"` followed by
the 8-byte little-endian Steam64 ID; confirm against BattlEye's documentation and a known pair
before relying on it). Gamepanel stored GUIDs by hand on cases, but we can compute them:

```sql
ALTER TABLE players ADD COLUMN be_guid TEXT UNIQUE;  -- set in Go on insert; one-off backfill for existing rows
```

This makes a GUID searchable in player lookup (BattlEye kick and ban messages only show GUIDs),
and lets bans be enforced at the BattlEye layer (§5, Wave 2).

### 2.3 Role sync engine (Discord roles + TeamSpeak server groups)

This is the main new piece. It's **one-way**, from the website's database out to Discord and
TeamSpeak. The website decides what each player should have, and each platform is made to match.
It's built once, with one small adapter per platform, so adding a third platform later (e.g. a
forum) is a new adapter, not a new sync system.

**Step 1: entitlements.** From a player's database state, compute a set of platform-neutral
entitlements such as `linked`, `staff_rank:admin`, `staff_team:support`, `staff_loa`,
`faction:police`, `faction_rank:police:3`, `gang:12`. Rules: on LOA → `staff_loa` *instead of*
their rank entitlement; suspended → no staff entitlements at all.

**Step 2: map entitlements to each platform's groups.**

```sql
CREATE TABLE platform_group_map (
    id           SERIAL PRIMARY KEY,
    platform     TEXT NOT NULL CHECK (platform IN ('discord', 'teamspeak')),
    entitlement  TEXT NOT NULL,     -- e.g. 'staff_rank:admin', 'faction_rank:police:3', 'linked'
    group_id     TEXT NOT NULL,     -- Discord role snowflake, or TeamSpeak server group ID
    UNIQUE (platform, entitlement, group_id)
);
```

**Step 3: reconcile.** Each adapter compares what the player *should* have with what they *do*
have on that platform, and applies only the difference.
- **When:** immediately on any change (§3), whenever a player joins the Discord server or connects
  to TeamSpeak, and in a full pass every 15 minutes that catches anything missed while a platform
  was unreachable.
- **Only mapped groups are touched.** A Discord role or TeamSpeak server group that isn't in
  `platform_group_map` is never added or removed, so hand-given roles (Nitro booster, event roles,
  TeamSpeak "Guest") are safe. This rule is what makes sync safe to turn on.
- **Manual edits to mapped groups get reverted and reported.** If someone gives a mapped role or
  group by hand in Discord or TeamSpeak, the next reconcile reverts it and records it as **drift**
  (§3.3). That makes it visible instead of silent. The correct place to promote someone is the
  website, or the Discord `/promote` command, which goes through the website (§3.2).
- The role editor (Wave 1) gets a **group picker for each platform**, populated from the real
  Discord roles and TeamSpeak server groups, so nobody pastes IDs by hand.
- **Discord requirements:** `Manage Roles`, the privileged **Server Members** intent, and the bot's
  role placed above every mapped role (the role editor warns if it isn't).
- **TeamSpeak requirements:** a ServerQuery account allowed to add and remove the mapped server
  groups (§2.6).

### 2.4 Discord posts: webhooks and DMs

- **One webhook helper for every staff action:** a single `logStaffAction` helper writes the
  `staff_log` row *and* posts to `DISCORD_STAFF_LOG_WEBHOOK`. This connects the webhook config
  that exists but isn't used yet, so no handler can write one without the other. A webhook
  failure never blocks the action, as WEBSITE.md §9 already requires.
- **New webhook channels** (each optional; an empty value means the channel gets no posts):
  `DISCORD_BANS_WEBHOOK`, `DISCORD_CASES_WEBHOOK`, `DISCORD_APPLICATIONS_WEBHOOK`,
  `DISCORD_STATUS_WEBHOOK`, `DISCORD_POLICE_COMMAND_WEBHOOK`, `DISCORD_EMS_COMMAND_WEBHOOK`.
- **DMs:** the notifications system ([GAMEPANEL_PARITY.md §7.3](GAMEPANEL_PARITY.md#73-notifications-with-acknowledgement))
  also sends each notification as a Discord DM when the player has linked Discord and has DMs
  turned on (`players.discord_dm_notifications BOOLEAN NOT NULL DEFAULT true`, a toggle on the
  dashboard). DMs link back to the website. "Essential" notifications still have to be
  acknowledged **on the website**; a DM being delivered doesn't count as acknowledgement. If a
  player has DMs closed, the bot records that and stops trying.

### 2.5 Bot commands

The full bot design (every command, buttons, events, ticket forum, outbox, permissions and build
stages) is in [DISCORD_BOT.md](DISCORD_BOT.md). The commands below are the ones this integration
plan depends on.

In addition to `/link` (already built):

| Command | Who | What |
|---|---|---|
| `/status` | Everyone | Same data as the `/status` page: each component's state, plus game-server player count. |
| `/profile` | Everyone | Your own linked account: in-game name, faction rank, links to the website. Only you see the reply. |
| `/whois @user` | Staff only | That member's linked Steam profile, BattlEye GUID, staff/faction status, and active warning points, with a link to their website profile. Only the staff member sees the reply. |
| `/rules` | Everyone | Links to the public rules pages (the wiki's public pages at `/rules`). |
| `/promote @user <rank>` · `/demote @user <rank>` · `/setrank @user <faction> <level>` | Staff / faction command | Change a rank **from Discord**. The command calls the same website code as the rank buttons on the website, with the same permission checks and the same logging. Only then do Discord and TeamSpeak update (§3). It's another way to reach the pipeline, not a second one. |

The permission check runs against the **caller's linked website account's rank and
permissions**, never against Discord roles. That way, an admin editing roles by hand in Discord
can't accidentally give someone `/whois` or `/promote`.

### 2.6 TeamSpeak 3 connector

New `internal/teamspeak` package. It keeps one persistent **ServerQuery** connection to the TS3
server (using the query port, 10011 raw or 10022 SSH; **SSH is preferred** so the query password
isn't sent in plain text). It lives inside the website binary, like the Discord bot, and is
optional: if the config is left unset, TeamSpeak sync is off and nothing else is affected.

- **Config:** `TS3_QUERY_ADDR`, `TS3_QUERY_USER`, `TS3_QUERY_PASSWORD`, `TS3_VIRTUAL_SERVER_ID`,
  `TS3_BOT_NICKNAME`. Use a **dedicated query login**, not `serveradmin`, whose permissions are
  limited to: listing clients, adding and removing the mapped server groups, sending text
  messages, and adding and removing bans. Add the website host to the TS3 server's query IP
  allowlist, so the connection isn't flood-banned.
- **Keepalive:** TS3 drops idle query connections after 5 minutes, so the connector sends a
  harmless command every few minutes and reconnects with backoff if the connection drops.
- **Events:** it subscribes to server events (client joined) and private text messages, so it
  can sync a player's groups as soon as they connect and handle the `!link` command below.

**Linking a TeamSpeak identity** (same idea as the Discord `/link` flow):

```sql
CREATE TABLE teamspeak_identities (
    id            BIGSERIAL PRIMARY KEY,
    player_id     BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    ts_uid        TEXT NOT NULL UNIQUE,   -- TS3 unique identity (base64); what server groups attach to
    ts_nickname   TEXT,                   -- display cache, never identity
    linked_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ
);
-- Codes are shared with Discord linking: discord_link_codes gains a 'platform' column
-- (or is renamed account_link_codes) rather than adding a second near-identical table.
```

1. On the dashboard, the player clicks **Link TeamSpeak**. The website shows a short-lived code.
2. In TeamSpeak, the player sends the bot a private message: `!link 4K7Q-92`.
3. The connector checks the code and records that TeamSpeak identity against the player. It
   replies "Linked to <in-game name>" and applies their server groups straight away.

The **Unique ID** is what gets stored, not the nickname, since anyone can change their nickname.
A player can link **several identities** (a desktop and a laptop each have their own), and each
one is listed on the dashboard with an Unlink button. The code proves ownership in the same
direction as Discord linking: the website session proves Steam, and sending the code from inside
TeamSpeak proves that identity.

**What the connector does:**
- **Server groups:** reconcile through the shared engine (§2.3). Groups are applied to *every*
  linked identity, so a promotion shows up on whichever computer the player uses.
- **Bans:** a game ban can also be a TeamSpeak ban on each of the player's linked identities
  (optional, same tick-box model as Discord bans, §7). Unban lifts it.
- **Status page:** TeamSpeak is added as a `/status` component, checked with a server-info query.
- **Channel groups** (e.g. "Channel Commander" in the police channels) aren't part of the first
  version. They're per-channel and are usually managed inside TeamSpeak. They can be added to the
  engine later as a new entitlement kind if you want them.

## 3. Promotions & role changes: logged once, synced everywhere

This is the rule the whole plan follows: **a rank or role change is recorded exactly once, in the
database, and every platform updates from that record.** The rest of this section is how that
holds no matter *where* the change was made.

### 3.1 Capture at the database, not the button

Rank changes can come from several places: the website, the Discord `/promote` command, the
**in-game admin menu** ([ADMIN_TOOLS.md](ADMIN_TOOLS.md), which writes through the C++ extension),
or a manual `psql` fix. If logging lived in each of those, one of them would eventually be
missed. So it lives in a **Postgres trigger** on `players` that fires whenever a sync-relevant
column changes (`staff_rank_id`, `staff_status`, `staff_team`, `cop_level`, `medic_level`,
`discord_id`), plus triggers on `gang_members` and `teamspeak_identities`:

```sql
CREATE TABLE rank_changes (
    id           BIGSERIAL PRIMARY KEY,
    player_id    BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    field        TEXT NOT NULL,          -- 'staff_rank_id', 'cop_level', 'staff_status', ...
    old_value    TEXT,
    new_value    TEXT,
    source       TEXT NOT NULL,          -- 'website', 'discord', 'game', 'manual' (see below)
    actor_id     BIGINT REFERENCES players(id) ON DELETE SET NULL,  -- who made the change, when known
    reason       TEXT,
    case_id      BIGINT,                 -- when the change came from a disciplinary case
    changed_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- **Who and where:** before each write, the website and the C++ extension set transaction-local
  settings (`SET LOCAL tasdyn.actor_id`, `tasdyn.source`, `tasdyn.reason`), and the trigger reads
  them. A change with no settings (someone in `psql`) is recorded as `source = 'manual'` with no
  actor, so it's flagged rather than hidden.
- **Nothing to forget:** a change can't reach the database without a `rank_changes` row, because
  the trigger runs in the same transaction as the change.
- The trigger also sends a Postgres `NOTIFY rank_changed, '<player_id>'`. The website listens for
  it and immediately runs the sync engine for that player (§2.3). The 15-minute full pass covers
  any notification missed while the website was restarting.

### 3.2 What happens on a promotion

Taking "Admin promotes Sam from Moderator to Admin on the website" as the example:

1. The website checks permission (the actor must outrank the new rank) and updates
   `players.staff_rank_id` in a transaction with the actor, source and reason set.
2. The trigger writes the `rank_changes` row and sends `NOTIFY`. The website's
   `logStaffAction` also writes a `staff_log` row, and posts to `#staff-log`:
   *"Admin promoted Sam: Moderator → Admin (reason)"*.
3. The sync engine recalculates Sam's entitlements. **Discord:** removes the Moderator role and
   adds Admin. **TeamSpeak:** does the same with server groups on each of Sam's linked identities.
4. Each platform change is written to `sync_log` (§3.3).
5. Sam gets a website notification, a Discord DM, and a TeamSpeak poke:
   *"You've been promoted to Admin."*

The same five steps run for a Discord `/promote` (step 1 happens inside the bot), for an in-game
admin-menu promotion (step 1 happens in the game; the website picks it up from `NOTIFY` and
writes the `staff_log` row and post from the `rank_changes` row), and for demotions, LOA,
suspension, faction rank changes, and linking or unlinking an account.

### 3.3 Sync log and drift

```sql
CREATE TABLE sync_log (
    id           BIGSERIAL PRIMARY KEY,
    player_id    BIGINT REFERENCES players(id) ON DELETE SET NULL,
    platform     TEXT NOT NULL CHECK (platform IN ('discord', 'teamspeak')),
    action       TEXT NOT NULL CHECK (action IN ('add', 'remove', 'ban', 'unban', 'drift_reverted')),
    group_id     TEXT,
    rank_change_id BIGINT REFERENCES rank_changes(id) ON DELETE SET NULL,  -- what caused it; NULL = periodic pass
    ok           BOOLEAN NOT NULL,
    error        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- The **player profile** shows a combined history of every rank change (who, where, why) and what
  happened on each platform as a result. That answers "why does Sam have this role?" in one
  place.
- **Failures are retried** on the next pass. Anything still failing after an hour shows up as a
  **Sync problems** card on the admin dashboard. The usual causes are the bot's Discord role being
  too low, or the TeamSpeak query login lacking a permission.
- **Drift** (a mapped role or group given or removed by hand on Discord or TeamSpeak) gets reverted
  and posted to `#staff-log`: *"Reverted manual change on TeamSpeak: Sam had 'Admin' group without
  the Admin rank."* That makes it obvious when someone bypasses the website.
- Retention: `rank_changes` is kept permanently (it's the promotion history). `sync_log` keeps
  180 days.

## 4. Account linking rules

- **Staff must link Discord, and TeamSpeak too if staff are expected on TS.** The staff directory
  flags anyone missing a link, and accepting a staff application requires a linked Discord
  account (§5, Wave 3). Without links, sync has no one to apply roles to and notifications have
  nowhere to go.
- **Players don't have to link.** Playing the game and using the website only need Steam.
  Faction members get faction roles and groups only on the platforms they've linked.
- **Unlinking** (dashboard buttons for Discord and for each TeamSpeak identity) removes every
  synced role or group from that account straight away. Unlinking is recorded in `rank_changes`
  like any other change. It's the only way a link is broken; nothing breaks it silently.
- **One Discord account per player account** (`discord_id` is already `UNIQUE`); **several
  TeamSpeak identities** per player, but each identity belongs to only one player. Re-linking
  moves the synced roles and groups to the new account or identity.

## 5. Feature-by-feature

Every row that says a role or rank changes goes through §3, so it's logged and applied to **both**
Discord and TeamSpeak even where the tables below only mention Discord.

Waves and section numbers match [GAMEPANEL_PARITY.md §11](GAMEPANEL_PARITY.md#11-build-order).

### Wave 1: Staff core

| Feature | Steam | Discord |
|---|---|---|
| Role editor (§2.1) | — | Each rank gets a mapped Discord role (role picker, §2.3). Changing a rank's role re-syncs everyone holding that rank. |
| Staff directory & profiles (§2.2) | Steam avatar + persona name; profile link. | Linked Discord username; "not linked" warning; team → team role. |
| LOA / suspend (§2.3) | — | LOA → rank role swapped for the LOA role; suspended → rank role removed; reinstated → restored. DM sent to the staff member with the reason. Posted to the staff-log channel. |
| Staff log (§2.4) | — | Every row is posted to `#staff-log` through the single `logStaffAction` helper. |
| Player lookup (+ vehicles) | Search by Steam64, **Steam profile URL / vanity name** (resolved through the Web API), and **BattlEye GUID**. Profile shows Steam name, avatar, account age, VAC/game bans. | Search by Discord username or ID; profile shows the linked Discord account. |

### Wave 2: Discipline

| Feature | Steam | Discord |
|---|---|---|
| Cases (OPERATIONS §2) | Participants can be added by Steam64/profile URL/GUID, even if they've never logged into the website (a `players` row is created from Steam). | Participants can be added by Discord user. New or closed case → `#cases` webhook (case number, type, lead; no evidence text). |
| Punishment points (§3.1) | — | The player gets a DM: points, rule broken, expiry, and how to appeal. |
| Bans from a case (§3.2) | **Game ban** = `banlist` row by Steam64 + **BattlEye GUID** pushed to BattlEye's ban list through `server_manager`, so a ban holds even if the mission-side check is bypassed. | Optional **Discord ban** tick box on the ban form (off by default, see §7). Every ban → `#bans` webhook. The player gets a DM **before** any Discord ban, because a ban removes the bot's ability to DM them. |
| Unban | Removes the `banlist` row and the BattlEye ban. | Lifts the Discord ban if one was issued, restores synced roles, DMs the player. |
| Ban appeals | Appeals always go through the website (Steam login still works while banned), so a Discord-banned player can still appeal. | The appeal ticket syncs into a staff Discord thread through the planned ticket-thread sync. |
| Case list & search, activity (§3.3–3.4) | Search by any Steam identifier. | Search by Discord user. |

### Wave 3: People pipeline

| Feature | Steam | Discord |
|---|---|---|
| Staff applications (§4.1) | **Eligibility checks shown to reviewers** (not auto-reject): Steam account age, VAC/game bans and days since, server playtime (`player_sessions`). Configurable minimums block submission only where you've decided they should (§7). | **Discord link required to apply**. This replaces Gamepanel's free-text "Discord handle", which proved nothing. New application → `#applications`. Decision → DM. **Accepted → staff role assigned automatically** by role sync. |
| Interviews (§4.2) | — | Interview time → DM to the applicant and the interviewer. Optional: create a Discord **scheduled event** in a private staff channel. |
| Notifications (§7.3) | — | Sent as DMs as well (§2.4). |
| Faction rank names (§6.3) | — | Each faction rank gets a mapped `faction_rank` role. |

### Wave 4: Factions

| Feature | Steam | Discord |
|---|---|---|
| Faction command panel (§6.1) | Roster shows Steam names/avatars alongside in-game names. | Promote, demote, recruit or remove → faction and rank roles re-synced immediately; posted to that faction's command channel webhook. |
| Faction applications (§6.2) | Applicant's Steam standing shown to command (same data as staff applications). | New application → faction command channel. Decision → DM. Accepted → faction role. |
| Faction records (OPERATIONS §6) | — | Optional: new BOLO/warrant records → police channel webhook (title and link only; record text stays on the website). |
| Gangs | — | Optional `gang` role per gang, synced from `gang_members`. Off by default, since each gang would add a role to the Discord server. |

### Wave 5: Knowledge

| Feature | Steam | Discord |
|---|---|---|
| Meetings + minutes (OPERATIONS §4, §7.1) | — | Creating a meeting creates a Discord **scheduled event** in the right team's channel; a reminder goes out an hour before; minutes are posted as a link afterwards. |
| Wiki / public rules (§7.2) | — | `/rules` command. Publishing a public page, or marking a staff page "essential", posts to the announcements channel and sends an essential notification. |
| Suggestions (§7.4) | — | A ticket category, so they automatically get the planned ticket-thread sync into Discord. |

### Wave 6: Operations & economy

| Feature | Steam | Discord |
|---|---|---|
| Status page (built) | Game-server check is already a Steam A2S query. | Component goes down or comes back → `#server-status` webhook (on state changes only, not every minute). `/status` command. |
| Server restart (§8.1) | — | Restart announced in `#server-status` with the countdown, alongside the in-game broadcast. Logged to `#staff-log`. |
| Compensation (§5.3) | — | The player gets a DM receipt: amount, reason, case number if any. |
| Economy dashboard, item prices, live logs, DB browser | — | Deliberately **not** posted to Discord. Money supply, rich lists and raw logs stay behind the panel's access checks. |

### TeamSpeak in each wave

| Wave | TeamSpeak |
|---|---|
| 1. Staff core | Role editor gets a TeamSpeak server-group picker next to the Discord one. Staff directory shows linked TS identities and flags staff with none. LOA/suspend swap or remove staff groups. Player lookup can search by TS unique ID. |
| 2. Discipline | Optional TeamSpeak ban on each linked identity when banning (same tick box as Discord). Unban lifts it. `/whois` and the player profile list TS identities. |
| 3. People pipeline | An accepted applicant gets their staff group on TeamSpeak automatically. Interview reminders are also sent as a TeamSpeak poke if they're connected. |
| 4. Factions | Faction and rank server groups (e.g. "Police", "Police · Sergeant") follow faction command's roster changes immediately. Optional gang groups, off by default. |
| 5. Knowledge | Meeting reminder sent as a poke to attendees who are connected to TeamSpeak. |
| 6. Operations | TeamSpeak becomes a `/status` component. The server-restart countdown is also sent to TeamSpeak as a server message. |

## 6. Security & privacy

- **Identity mapping is never public.** Only staff can see which Discord account or TeamSpeak
  identity belongs to which Steam account (`/whois`, the player profile), and only the caller sees
  the `/whois` reply. The public site and public channels never show it.
- **Least privilege on both platforms.** Discord bot: `Manage Roles`, `Send Messages`, `Create
  Public/Private Threads`, `Manage Events`, plus `Ban Members` **only if** Discord bans are
  enabled, and never `Administrator`. TeamSpeak: a dedicated query login limited to the actions in
  §2.6, never `serveradmin`. Missing permissions show up on the admin dashboard's Sync problems
  card, not as a surprise failure.
- **Rank changes can only happen through the pipeline.** The website, `/promote`, and the in-game
  menu all apply the same "must outrank the target and the new rank" rule. Manual changes on
  Discord or TeamSpeak are reverted and reported (§3.3).
- **Webhook posts carry no evidence or private text:** case numbers, names and links only. The
  details stay behind website permissions. A Discord channel's permissions are one more thing that
  can be misconfigured, and they don't mirror the website's rules.
- **Secrets:** `STEAM_WEB_API_KEY`, the bot token, webhook URLs and TeamSpeak query credentials are
  environment variables only, never committed (same rule as today).
- **Failure isolation:** Steam API down → cached data. Discord bot or TeamSpeak connection down →
  the website works normally; the change is still recorded in `rank_changes`, and the next
  reconcile applies it. Webhook fails → logged, action still applied. No Steam, Discord or
  TeamSpeak outage can block a ban, a promotion, a login, or a page.

## 7. Decisions needed

1. **Discord and TeamSpeak bans:** when staff ban someone from the game, should they also be
   banned on Discord/TeamSpeak (a) by tick box, off by default (**recommended**: some bans are
   game-only), (b) automatically for permanent bans only, or (c) never?
2. **Application minimums:** hard requirements (e.g. Steam account ≥ 6 months old, no VAC/game ban
   in the last 2 years, ≥ 20 hours on the server), or only shown to reviewers?
   **Recommendation:** show everything, and hard-block only on the linked Discord account.
3. **Must staff link TeamSpeak?** **Recommendation:** yes, if staff are expected to take support
   calls on TS; otherwise just flag it.
4. **Nicknames:** should the bot set Discord nicknames like `[SGT] In-game Name`?
   **Recommendation:** no to start with. Nicknames are something players care about, and a bot
   overwriting them causes friction. Roles and groups alone are enough.
5. **Gang roles/groups:** sync gang membership to Discord roles and TeamSpeak groups?
   **Recommendation:** off by default.
6. **TeamSpeak version:** this plan targets a **TeamSpeak 3** server's ServerQuery. If you move to
   the newer TeamSpeak 6 server, check its query interface before building §2.6.

## 8. Build order

The foundations get built at the **start of Wave 1**, because nearly every feature after that uses
them:

1. Steam Web API client + profile/ban cache + BattlEye GUID (§2.1–2.2). Small, and it immediately
   fixes the "Player #35" names on the panel.
2. The `rank_changes` table and trigger (§3.1) and the `logStaffAction` helper wired to the
   staff-log webhook (§2.4). Everything from here on is logged, whichever tool made the change.
3. The role sync engine (§2.3) with the Discord adapter, starting with the "Verified" and
   staff-rank roles. Built alongside the role editor, which needs its group pickers.
4. The TeamSpeak connector (§2.6): ServerQuery connection, `!link`, and the TeamSpeak adapter for
   the same engine. It comes after Discord, because the engine and its `sync_log` are proven by
   then and TeamSpeak is only a second adapter.
5. The Discord `/promote` / `/demote` / `/setrank` commands (§2.5).
6. Then each wave's Steam/Discord/TeamSpeak additions ship **together with the feature they belong
   to**, per §5, never as a separate integration pass afterwards.

The ticket-thread sync already designed in WEBSITE.md §9 slots in with Wave 2, where ban appeals
make it matter most.
