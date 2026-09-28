# Discord & Steam Integration Plan

How every feature in [GAMEPANEL_PARITY.md](GAMEPANEL_PARITY.md) connects to **Steam** (who the
player is, their account standing, their BattlEye identity) and **Discord** (where the community
and staff actually talk). The aim is that staff never have to copy a Steam ID into Discord by
hand, or remember to give someone a role after promoting them. The website is the source of
truth, and Steam and Discord follow it.

## 1. What exists today

| | Built | Not built |
|---|---|---|
| **Steam** | "Sign in with Steam" (OpenID 2.0, `internal/auth/steam.go`); `players.uid` is the Steam64 ID. | No Steam Web API use, so no persona name, avatar, account age or VAC/game-ban data. That's why panels show "Player #35" for someone who hasn't joined the game yet. |
| **Discord** | Bot (`internal/discord/bot.go`) with `/link <code>`; OAuth2 "Connect Discord" + "Sign in with Discord" for linked accounts; ticket-created webhook. | Staff-log, ban and anti-cheat webhooks are configured in `config.go` but never called. Two-way ticket-thread sync is designed ([WEBSITE.md §9](WEBSITE.md#9-discord-integration)) but not built. No role sync, no DMs. |

The identity rules in [WEBSITE.md §3](WEBSITE.md#3-authentication--steam-not-a-new-identity-system)
don't change: **Steam creates the account, and Discord is a linked second identity.** Everything
below builds on that.

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
and lets bans be enforced at the BattlEye layer (§4, Wave 2).

### 2.3 Discord role sync

This is the main new piece. It's **one-way**, from website to Discord. The website decides
who should have which role, and the bot makes Discord match.

```sql
CREATE TABLE discord_role_map (
    id       SERIAL PRIMARY KEY,
    kind     TEXT NOT NULL CHECK (kind IN
               ('linked', 'staff_rank', 'staff_team', 'staff_loa', 'staff_suspended',
                'faction', 'faction_rank', 'gang')),
    ref      TEXT NOT NULL DEFAULT '',   -- e.g. staff_ranks.key, team name, 'police', 'police:3'
    role_id  TEXT NOT NULL,              -- Discord role snowflake
    UNIQUE (kind, ref)
);
```

- **Desired roles** for a player are computed from their website state: linked account →
  "Verified"; staff rank → its role; staff team → its role; on LOA → the LOA role *instead of*
  their rank role; suspended → rank role removed; police/EMS level → faction role plus rank role.
- **Reconciling:** the bot applies the difference (roles to add or remove) when something changes
  on the website, whenever a member joins the Discord server, and in a full pass every 15 minutes
  that catches anything missed while the bot was offline.
- **The bot only touches roles listed in `discord_role_map`.** Hand-assigned roles
  (Nitro booster, event roles) are never removed. This rule is what makes role sync safe to turn
  on.
- The role editor (Wave 1) gets a **Discord role picker** populated from the guild's real roles,
  so nobody pastes role IDs by hand. That page shows a warning if the bot's own role sits below a
  mapped role, since Discord won't let the bot assign roles above its own.
- **Requires:** the bot's `Manage Roles` permission and the privileged **Server Members** intent,
  both turned on in the Discord developer portal.

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

In addition to `/link` (already built):

| Command | Who | What |
|---|---|---|
| `/status` | Everyone | Same data as the `/status` page: each component's state, plus game-server player count. |
| `/profile` | Everyone | Your own linked account: in-game name, faction rank, links to the website. Only you see the reply. |
| `/whois @user` | Staff only | That member's linked Steam profile, BattlEye GUID, staff/faction status, and active warning points, with a link to their website profile. Only the staff member sees the reply. |
| `/rules` | Everyone | Links to the public rules pages (the wiki's public pages at `/rules`). |

The staff check runs against the **caller's linked website account's rank and permissions**, never
against Discord roles. That way, an admin editing roles by hand in Discord can't accidentally give
someone `/whois`.

## 3. Account linking rules

- **Staff must link Discord.** The staff directory flags unlinked staff, and accepting a staff
  application requires a linked account (§4, Wave 3). Without that, role sync has no one to sync
  and DMs have nowhere to go.
- **Players don't have to link.** Playing the game and using the website only need Steam.
- **Unlinking** (a new dashboard button) removes every synced role straight away and clears
  `discord_id`. That's the only way the link is broken; nothing breaks it silently.
- **One Discord account per player account** (`discord_id` is already `UNIQUE`). Re-linking to a
  different Discord account moves the synced roles to the new account.

## 4. Feature-by-feature

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
| Bans from a case (§3.2) | **Game ban** = `banlist` row by Steam64 + **BattlEye GUID** pushed to BattlEye's ban list through `server_manager`, so a ban holds even if the mission-side check is bypassed. | Optional **Discord ban** tick box on the ban form (off by default, see §6). Every ban → `#bans` webhook. The player gets a DM **before** any Discord ban, because a ban removes the bot's ability to DM them. |
| Unban | Removes the `banlist` row and the BattlEye ban. | Lifts the Discord ban if one was issued, restores synced roles, DMs the player. |
| Ban appeals | Appeals always go through the website (Steam login still works while banned), so a Discord-banned player can still appeal. | The appeal ticket syncs into a staff Discord thread through the planned ticket-thread sync. |
| Case list & search, activity (§3.3–3.4) | Search by any Steam identifier. | Search by Discord user. |

### Wave 3: People pipeline

| Feature | Steam | Discord |
|---|---|---|
| Staff applications (§4.1) | **Eligibility checks shown to reviewers** (not auto-reject): Steam account age, VAC/game bans and days since, server playtime (`player_sessions`). Configurable minimums block submission only where you've decided they should (§6). | **Discord link required to apply**. This replaces Gamepanel's free-text "Discord handle", which proved nothing. New application → `#applications`. Decision → DM. **Accepted → staff role assigned automatically** by role sync. |
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

## 5. Security & privacy

- **Steam ↔ Discord mapping is never public.** Only `/whois` shows it, only to staff, and only
  the caller sees the reply. The public site and public channels never show which Discord account
  belongs to which Steam account.
- **Least-privilege bot.** It needs `Manage Roles`, `Send Messages`, `Create Public/Private
  Threads`, `Manage Events`, plus `Ban Members` **only if** Discord bans are enabled. No
  `Administrator` permission. Missing permissions are reported on the admin dashboard, not
  discovered when an action fails.
- **Webhook posts carry no evidence or private text:** case numbers, names and links only. The
  details stay behind website permissions. A Discord channel's permissions are one more thing that
  can be misconfigured, and they don't mirror the website's rules.
- **Secrets:** `STEAM_WEB_API_KEY`, the bot token and webhook URLs are environment variables only,
  never committed (same rule as today).
- **Failure isolation:** Steam API down → cached data. Bot offline → the website works normally and
  the next reconcile catches up. Webhook fails → logged, action still applied. No Steam or
  Discord outage can block a ban, a login, or a page.

## 6. Decisions needed

1. **Discord bans:** when staff ban someone from the game, should a Discord ban be (a) a tick box,
   off by default (**recommended**: some bans are game-only), (b) automatic for permanent bans
   only, or (c) never?
2. **Application minimums:** hard requirements (e.g. Steam account ≥ 6 months old, no VAC/game ban
   in the last 2 years, ≥ 20 hours on the server), or only shown to reviewers?
   **Recommendation:** show everything, and hard-block only on the linked Discord account.
3. **Discord nicknames:** should the bot set nicknames like `[SGT] In-game Name` for faction
   members? **Recommendation:** no to start with. Nicknames are something players care about, and
   a bot overwriting them causes friction. Roles alone are enough.
4. **Gang roles:** sync gang membership to Discord roles? **Recommendation:** off by default.

## 7. Build order

The foundations get built at the **start of Wave 1**, because nearly every feature after that uses
them:

1. Steam Web API client + profile/ban cache + BattlEye GUID (§2.1–2.2). Small, and it immediately
   fixes the "Player #35" names on the panel.
2. The `logStaffAction` helper wired to the staff-log webhook (§2.4). Small; every Wave 1 feature
   writes staff-log rows.
3. The role sync engine with the "Verified" (linked) and staff-rank roles (§2.3). Built alongside
   the role editor, which needs its role picker.
4. Then each wave's Discord/Steam additions ship **together with the feature they belong to**, per
   §4, never as a separate integration pass afterwards.

The ticket-thread sync already designed in WEBSITE.md §9 slots in with Wave 2, where ban appeals
make it matter most.
