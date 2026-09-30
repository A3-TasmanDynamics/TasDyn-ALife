# Gamepanel Feature Parity Plan

Every feature in [Gamepanel](https://github.com/KieranHolroyd/Gamepanel) (a PHP/MySQL staff panel
built for a previous Arma Life server), mapped to what TasDyn-ALife does about it: already built,
already designed elsewhere, newly planned here, or deliberately skipped. The inventory comes from
reading the whole codebase: every page, both API versions (`api/v1`, `api/v2`), the schema
(`panel_base_schema.sql`), and all 47 permission keys. It isn't taken from the README.

This is a **feature** plan, not a port. The stacks differ (PHP/MySQL/Vue vs. Go/Postgres/
server-rendered templates) and several of Gamepanel's implementations aren't safe to copy (see
[§10](#10-not-carried-over-and-why)). [OPERATIONS.md](OPERATIONS.md) already holds the detailed
designs for five of these features and isn't repeated here. This doc covers the rest and sets one
build order for all of them.

How each feature connects to Discord, Steam and TeamSpeak (role/group sync, DMs, webhooks, Steam bans, BattlEye
GUIDs) is planned in [INTEGRATIONS.md](INTEGRATIONS.md).

Status key: **Built** · **Designed** (detailed design exists in another doc) · **New** (designed
here) · **Skip** (deliberately not doing it, with the reason).

## 1. Feature matrix

### Access & roles

| # | Gamepanel feature | Where | Status | Our version |
|---|---|---|---|---|
| 1 | Email/password signup & login | `passport.php`, `auth/*` | **Skip** | Steam OpenID + Discord OAuth, already built. No passwords to store. |
| 2 | Rank groups with permission lists, `*` wildcard, ordered by position | `rank_groups`, `Permissions.php` | **Built** | `staff_ranks` (level-ordered) + `staff_permission_overrides` by `command_key`. |
| 3 | Role editor: create/edit/delete, reorder, tick permissions | `staff/roles.php` | **New** §2.1 | Web UI over `staff_ranks` + a permission catalogue. |
| 4 | "Server owner" bypass flag | `isServerOwner` | **Skip** | Level 100 Head Admin already covers this. A second bypass path is a second thing to audit. |
| 5 | "Awaiting approval" / "not staff" / suspended / LOA pages | `errors/*` | **Built** (suspended/LOA, PR #111) · **New** (awaiting approval, with §4) | Specific denial message instead of a generic 403. |

### Staff management

| # | Gamepanel feature | Where | Status | Our version |
|---|---|---|---|---|
| 6 | Staff directory & team overview (counts per team, unassigned, senior team) | `staff/overview.php`, `rollcall` | **New** §2.2 | `/admin/staff`, grouped by team. Replaces the separate roll-call table. |
| 7 | Staff profile: rank, team, region, notes, promotion notes, remove from staff | `staff/index.php` | **New** §2.2 | Staff profile page. Name/UID editing is dropped because identity is Steam. |
| 8 | Put on LOA / suspend, with reason and end date | `SEND_USER_ON_LOA`, `SEND_USER_ON_SUSPENSION` | **Built** (schema + enforcement) · **New** (UI) §2.3 | Buttons on the staff profile, audited in `staff_log`. |
| 9 | Per-staff activity & statistics (cases per day/week) | `staff/statistics.php`, `getStaffActivity` | **New** §3.4 | Shown on the staff profile. |
| 10 | Staff audit log | `staff/audit.php` | **Built** (latest 50) · **New** (filters, paging) §2.4 | Filter by staff member, target, action, date. |
| 11 | "My Activity" (my own cases) | `me.php` | **New** §3.4 | Personal view of the same data as #9. |
| 12 | Public staff application form + review queue | `staff/apply.php`, `staff/applications.php` | **New** §4.1 | Tied to the applicant's Steam account, not free-text identity. |
| 13 | Structured staff interviews (set questions, pass/fail, interviewer) | `staff/interviews.php`, `staff_interviews` | **New** §4.2 | Linked to the application it follows up. |

### Cases & discipline

| # | Gamepanel feature | Where | Status | Our version |
|---|---|---|---|---|
| 14 | Case logger: lead + assisting staff, type, description, involved players | `case_logs`, `case_players` | **Designed** | [OPERATIONS.md §2](OPERATIONS.md#2-structured-disciplinary-cases) |
| 15 | Punishment reports: points, rules broken, comments per player | `punishment_reports` | **New** §3.1 | Warning points that add up per player and expire. |
| 16 | Ban reports: length, message, which platforms, permanent, manual expiry | `ban_reports` | **Designed** (ban ↔ case link) · **New** (scope + permanent gate) §3.2 | A ban is always issued through `banlist`. Permanent bans need their own permission. |
| 17 | Case list, case viewer, case search | `viewer.php`, `newviewer.php`, `search/` | **New** §3.3 | Filterable list plus search by player or staff member. |
| 18 | Case edits held for approval, shown as a diff | `staff/approve_edits.php`, `DiffViewer.php` | **New, changed** §3.1 | Cases are append-only: corrections are new entries, so history can't be rewritten and no approval queue is needed. |
| 19 | Case statistics (daily/weekly charts) | `StatisticsController` | **New** §3.4 | Admin dashboard chart. |

### Players & economy

| # | Gamepanel feature | Where | Status | Our version |
|---|---|---|---|---|
| 20 | Player search (name / GUID) | `SearchController@players` | **Designed** | [WEBSITE.md §7](WEBSITE.md#7-admin-panel) player lookup, via `player_aliases`. |
| 21 | Player profile incl. vehicles | `game/players.php`, `VIEW_GAME_VEHICLES` | **Designed** · **New** (vehicles tab) §5.1 | Adds the `vehicles`/`garages` view to the designed profile. |
| 22 | Player audit trail | `game/players.php` | **Designed** | `player_log`, `kick_log`, `anti_cheat_flags`, `staff_log` as target. |
| 23 | Set police/medic/admin level and department | `EDIT_PLAYER_POLICE/MEDIC/ADMIN` | **New** §5.2 | Admin level is **not** editable here: staff rank goes through the role system only. |
| 24 | Balance edit / compensation | `EDIT_PLAYER_BALANCE`, "Compensating" | **New** §5.3 | Written as a `bank_transactions` row with a reason, never a direct balance overwrite. |
| 25 | Dashboard economy stats: totals, server money supply, rich list | `index.php` | **New** §5.4 | Admin dashboard cards. |
| 26 | In-game item price list | `staff/itemprices.php` | **New** §5.5 | Read-only at first; editing waits for the Phase 4 economy config. |
| 27 | Faction rank names per level ("Constable", "Sergeant"…) | `GetLevelData`, `factionConfigs` | **New** §6.3 | `faction_rank_names` table, shown everywhere a level number appears today. |

### Factions

| # | Gamepanel feature | Where | Status | Our version |
|---|---|---|---|---|
| 28 | Faction command panel: roster, set rank, recruit into police/EMS | `factions/manage.php`, `VIEW_COMMAND` | **New** §6.1 | Gated on **in-game command rank**, not staff permission. Same second permission type as faction records. |
| 29 | Faction waiting list (players in no faction) | `factions/waiting.php` | **New, changed** §6.2 | Players apply to a faction; command reviews. Not a list of every civilian. |
| 30 | Faction command audit log | `factions/audit.php` | **New** §6.1 | Every roster change logged, visible to that faction's command and to staff. |
| 31 | Player-written faction duty records (MDT) | *(not in Gamepanel)* | **Designed** | [OPERATIONS.md §6](OPERATIONS.md#6-faction-records-the-new-idea--player-facing-not-staff-facing) |

### Knowledge & communication

| # | Gamepanel feature | Where | Status | Our version |
|---|---|---|---|---|
| 32 | Meetings: agenda points, up/down votes, comments | `meetings/`, `meeting_points` | **Designed** | [OPERATIONS.md §4](OPERATIONS.md#4-staff-meetings) |
| 33 | Meeting recordings / minutes | `meetings/x/recordings.php` | **New** §7.1 | A minutes field plus an optional recording link on each meeting. |
| 34 | Guides / policies, with a public policies view | `guides`, `policies.php` | **Designed** · **New** (public flag) §7.2 | [OPERATIONS.md §5](OPERATIONS.md#5-internal-wiki) wiki, plus pages marked public shown at `/rules`. |
| 35 | Staff notebook | `notebook/`, `pages` | **New, merged** §7.2 | Private "draft" wiki pages, not a second editor. |
| 36 | Notifications, incl. "essential" must-acknowledge | `notifications`, `markEssentialRead` | **New** §7.3 | In-panel notifications. "Essential" ones (e.g. a policy change) must be acknowledged, and the panel records who has. |
| 37 | Suggestion box | `suggestions.php`, `viewSuggestions.php` | **New, merged** §7.4 | A "Suggestion" support-ticket category. Reuses the whole ticket system. |
| 38 | Global search (players + cases) | `search/` | **New** §3.3 | One search box on the admin panel. |
| 39 | Staff chat / messages | `chat.php`, `staffMessages` | **Skip** | Discord already does this better. A second chat splits conversations. |

### Game server

| # | Gamepanel feature | Where | Status | Our version |
|---|---|---|---|---|
| 40 | Restart main / dev server from the web | `game/manage.php` (WebSocket) | **New** §8.1 | Through `src/server_manager`, Head Admin only, typed confirmation, audited. |
| 41 | Live server log viewer (several log files) | `logs.php` (WebSocket) | **New** §8.2 | Streamed from `server_manager`, read-only, senior staff. |
| 42 | Server statistics | `ServerStatistics` | **Built** | `/status` (PR #112): real checks and 60-day history. |

### Commerce & infrastructure

| # | Gamepanel feature | Where | Status | Our version |
|---|---|---|---|---|
| 43 | Purchase activation (licence key → reserved slot) | `purchases/activate.php` | **Decision needed** §9 | Depends on monetisation policy. See §9. |
| 44 | REST API + login tokens | `api/v2`, `login_tokens` | **Skip** | Server-rendered pages with session cookies. Revisit only if an external client needs it. |
| 45 | Error log table | `errorlog` | **Skip** | Structured logging (`slog`) already covers it. |

**Totals:** 45 rows (44 Gamepanel features, plus #31 for context). Rows touching each status: 5
built, 8 designed elsewhere, 32 new or changed here, 5 skipped, 1 decision needed. A row can have
two statuses (e.g. "Designed · New"), so these add up to more than 45.

## 2. Staff management

### 2.1 Role editor (#3)

`/admin/roles`: list `staff_ranks` by level. Edit display name, level, and the default Admin/Support
panel grants. Tick `command_key`s from a **permission catalogue**, a Go slice in `internal/auth`
listing every key with a label and group, so the UI can never offer a key that no code checks.
Reordering changes `level`. **Guardrails:** nobody can create or edit a rank at or above their own
level, and nobody can remove the last level-100 rank. Every change is written to `staff_log` with
before/after values. Permission: `roles.manage`.

**Built** (`/admin/roles`, `internal/roles`). Ticked keys are stored per rank in `rank_permissions`,
and that's what `auth.Can` checks after any per-player override. One extra guardrail: you can only
grant a key you hold yourself. The page also has the faction rank names tab (§6.3,
`factions.configure`).

### 2.2 Staff directory & profiles (#6, #7)

```sql
ALTER TABLE players ADD COLUMN staff_team   TEXT;   -- e.g. 'moderation', 'support', 'development'; NULL = unassigned
ALTER TABLE players ADD COLUMN staff_region TEXT;   -- 'AU-East', 'AU-West', 'NZ', ...
CREATE TABLE staff_notes (
    id          BIGSERIAL PRIMARY KEY,
    player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,  -- the staff member the note is about
    author_id   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('note', 'promotion')),
    body        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`/admin/staff` groups staff by team, with an "Unassigned" group and counts per group, matching
Gamepanel's overview. Each staff profile shows rank, team, region, status (active/LOA/suspended),
notes history, and activity (§3.4). Notes are append-only rows, not one overwritable text field:
Gamepanel's `saveStaffNotes` overwrote the previous note, which lost history. "Remove from staff"
clears `staff_rank_id` and logs it. Permissions: `staff.view`, `staff.edit`, `staff.remove`.

### 2.3 LOA / suspension controls (#8)

The schema and enforcement shipped in PR #111. What's missing is the UI: "Put on LOA" and
"Suspend" on the staff profile, each with a required reason and an optional end date, plus a
"Reinstate" action. A small background sweep returns expired LOAs to `active` (suspensions are
lifted by hand on purpose). Each action is logged. Permissions: `staff.loa`, `staff.suspend`.

### 2.4 Staff log filters (#10)

Filter by staff member, target, action, and date range, with paging (Gamepanel pages 500 at a
time). Uses the existing `staff_log` table; no schema change.

## 3. Cases & discipline

The core design (`staff_cases`, `staff_case_participants`, `staff_case_entries`) is in
[OPERATIONS.md §2](OPERATIONS.md#2-structured-disciplinary-cases). This section adds the rest of
what Gamepanel's case system did.

### 3.1 Punishment points & append-only corrections (#15, #18)

```sql
CREATE TABLE punishment_points (
    id          BIGSERIAL PRIMARY KEY,
    case_id     BIGINT NOT NULL REFERENCES staff_cases(id) ON DELETE CASCADE,
    player_id   BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    points      INTEGER NOT NULL CHECK (points > 0),
    rules       TEXT NOT NULL,        -- rule references, e.g. '2.1, 4.3'
    comment     TEXT,
    issued_by   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    expires_at  TIMESTAMPTZ,          -- points stop counting after this
    revoked_at  TIMESTAMPTZ,          -- revoked through a later case entry, never deleted
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

A player's **active points** (not expired, not revoked) appear on their profile and in case views.
The thresholds (e.g. 10 active points → review for a temporary ban) are a **policy decision** to
make before building. They're shown as guidance to staff, not enforced automatically.

Gamepanel held case edits in an approval queue with a diff view. We don't let anyone edit case
entries at all: a correction is a new entry that references the one it corrects, so the original
text stays visible. That gives the same protection against quietly rewritten history, with no
approval queue to maintain.

### 3.2 Bans from a case (#16)

A ban is always written to `banlist`, the table the game already enforces, with `case_id` set.
Gamepanel's `ban_reports` recorded separate TeamSpeak/in-game/website flags. Our equivalent is a
`scope` on the ban: `game` (always), plus `discord`, which the bot applies as a role or ban once
Discord moderation is wired up. **Permanent bans need a separate permission** (`bans.permanent`),
the same split Gamepanel made between `ADD_BAN` and `ADD_BAN_PERMANENT`.

### 3.3 Case list & search (#17, #38)

`/admin/cases` lists cases with filters (status, type, lead staff, date) and a search box that
finds cases by any participant's name, Steam ID, or alias (through `player_aliases`). The same
search box on the admin dashboard also returns matching players. Postgres `ILIKE` with the
existing indexes is enough at this server's scale; no external search engine.

### 3.4 Activity & statistics (#9, #11, #19)

All derived from `staff_cases` / `staff_case_participants`, so no new tables:
- Admin dashboard: cases opened per day (last 14 days) as a small bar chart.
- Staff profile: cases led and assisted this week, last 30 days, and all time.
- `/admin/me`: your own case history (Gamepanel's "My Activity").

## 4. Recruitment

### 4.1 Staff applications (#12)

```sql
CREATE TABLE staff_applications (
    id           BIGSERIAL PRIMARY KEY,
    player_id    BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,  -- Steam-verified applicant
    answers      JSONB NOT NULL,       -- question key -> answer, against the question set below
    status       TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'interview', 'accepted', 'rejected', 'withdrawn')),
    reviewed_by  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    review_note  TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at   TIMESTAMPTZ
);
```

Signed-in players apply from their dashboard. The questions are those from Gamepanel's form (age,
timezone, about me, why me, experience), defined in code so answers stay structured. Email is
dropped because Discord is linked instead. Only one pending application is allowed per player,
and there's a cooldown after a rejection. Reviewers can move an application to **interview**,
**accepted** (which assigns the entry rank and team), or **rejected**. The applicant sees their
status on their dashboard, and a notification is sent (§7.3). Permissions: `applications.view`,
`applications.decide`.

### 4.2 Interviews (#13)

```sql
CREATE TABLE staff_interviews (
    id              BIGSERIAL PRIMARY KEY,
    application_id  BIGINT NOT NULL REFERENCES staff_applications(id) ON DELETE CASCADE,
    interviewer_id  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    answers         JSONB NOT NULL,   -- Gamepanel's set: prior experience, ever banned (why), weekly hours, time away, flexibility
    outcome         TEXT CHECK (outcome IN ('pass', 'fail')),  -- NULL = in progress
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Linked to the application rather than a free-text applicant name, as Gamepanel had it. Its
"processed" flag becomes the application's status.

## 5. Players & economy

### 5.1 Vehicles tab (#21)

Add a vehicles tab to the player profile designed in [WEBSITE.md §7](WEBSITE.md#7-admin-panel):
`vehicles` joined with `garages`, plus `vehicle_logs` for history. Read-only. Permission:
`players.vehicles`.

### 5.2 Faction level & department edits (#23)

Staff can set `cop_level` / `medic_level` (and department, once departments exist) from the player
profile. This is mainly an override; day-to-day roster changes belong to faction command (§6.1).
A required reason is recorded in `staff_log`. If the player is online, the change takes effect on
their next save/load; the admin panel doesn't push live changes into the game. **The admin level
is not editable here:** staff rank changes go only through §2.1/§2.2, so there is one path to
audit. Permissions: `players.edit_police`, `players.edit_medic`.

**Only Management can override** (decided 29 Sep 2026). Both keys are seeded to Head Admin only and
are `NoOverride`, so they can't be handed to an individual; a future Management rank gets them by
ticking them on Roles & Permissions. Overrides go through the same `internal/factions` service as
faction command and appear in the faction's Command log as **staff override**.

### 5.3 Compensation (#24)

"Compensate" on the player profile adds a `bank_transactions` row (type `compensation`, amount,
reason, linked case if any). It never overwrites a balance, so the economy's audit trail stays
intact and the existing bank-sync triggers do the rest. Amounts above a threshold need a
higher-level permission. Permissions: `players.compensate`, `players.compensate_large`.

### 5.4 Economy dashboard (#25)

Admin dashboard cards: total players, police count, EMS count, **total money supply** (the sum of
bank balances plus cash), and a top-10 rich list. The rich list is staff-only: publishing
player wealth invites targeting in-game.

### 5.5 Item prices (#26)

Read-only table of item prices from the economy config. Editing waits until Phase 4 decides where
prices live (DB vs. mission config). Building an editor before then would mean building it twice.

## 6. Factions

### 6.1 Faction command panel (#28, #30)

A player-facing panel (not part of the staff Admin Panel) for police and EMS **command**, meaning
players whose faction level is at or above a configured command threshold. It uses the same new
"is this player currently in faction Y, at level Z" check that
[OPERATIONS.md §6](OPERATIONS.md#6-faction-records-the-new-idea--player-facing-not-staff-facing)
introduces for faction records. The two features should share that check, not build it twice.

- Roster: every member with their rank name (§6.3) and last-seen time.
- Promote/demote, but **only below your own level**, and remove from the faction.
- Recruit: accept a pending faction application (§6.2).
- Every change is written to a `faction_log` table (faction, actor, target, before/after, reason),
  which that faction's command and staff with `factions.audit` can view. This is Gamepanel's
  `PD_EMS_COMMAND` audit, as its own table.

**Command authority** is set per rank on Roles & Permissions → Faction rank names: "can set ranks up
to" (`faction_rank_names.promote_up_to`). Any rank with it set is command. Command can recruit,
promote, demote and remove members whose current *and* new rank are within that authority, which is
always below their own. Optional slot limits per rank bind command but not Management.

**Staff who are in a faction, or are faction command** (decided 29 Sep 2026):

1. Staff rank and faction rank are separate authorities. A staff member who is police command uses
   the command panel for everyday roster work, exactly like any other commander.
2. **Nobody changes their own rank**, on either path.
3. **Only Management overrides** (§5.2). An override by a staff member who is in that faction is
   allowed but flagged **own faction** in the Command log and Staff Log.
4. Every Command log entry records which authority was used: *command* or *staff override*.
5. A staff suspension or LOA pauses staff access only; faction membership and command are unaffected.
6. When Cases exist: leading a case against a member of your own faction shows a warning and suggests
   handing it to someone else.
7. Discord roles stack: staff, faction and faction-rank roles each come from their own mapping.

**Command tools beyond the roster** (layout plan "Police command", built 30 Sep 2026). All are done
under command authority and re-checked on every action; staff with `factions.audit` see them
read-only. Everything is recorded in the Command log.

- **Recruits & training.** Being recruited (by command, or by an accepted application) starts a
  probation (`faction_settings.probation_days`, 14 by default). Command assigns an FTO, who must be
  ranked above the recruit. They sign off each item on the faction's training sheet
  (`faction_training_items`) as *pass*, *needs work* or *not done*; an item with a retake wait (the
  theory exam, 24 h) can't be passed again until the wait is over. *Confirm* needs every item passed and
  promotes one rank; *End probation* removes them and records a "Probation ended" discharge.
- **Discipline.** Only for members ranked below you. The offence guide (`faction_offences`) sets each
  offence's point range and whether a marked verbal warning (MVW) is allowed. Three active warnings
  convert to 10 points. The ladder (10 → 1-day suspension, 15 → 3-day, 20 → 7-day with demotion
  recommended, 30 → 7-day with forced demotion, 40 → termination, 50 → termination and blacklist)
  only *suggests*; command ticks a box to apply it, and a rank change beyond their authority is
  recorded as "needs higher command" rather than applied. Points expire after
  `points_expiry_days` (90 by default), warnings after `mvw_days` (7). Entries are append-only; a
  correction is a new entry that cancels the points and ends any suspension the entry caused.
  Discharges (resigned, inactivity, honourable, contract termination) remove the member. The
  blacklist stops a player applying or being recruited by command until it ends (Management
  overrides are not blocked); lifting it early needs authority up to their last rank.
  **Suspensions are recorded and shown, but not yet enforced in game**: the game writes faction
  levels back to the database, so hiding a suspended officer's level from the game could remove
  their rank for good. Enforcing it needs a mission change.
- **Divisions & quals.** One specialist division per member (`faction_member_divisions`), with
  ordered roles and entry requirements (a qualification and/or minimum rank). Qualifications
  (`faction_quals`) are recorded from the member's service record; each can have a head trainer.
- **Ranks & gear.** The faction's top rank edits the rules of every rank below their own: name,
  short name, slots, minimum days in rank, required qualifications, who it can promote, and the
  public description. Staff with `factions.configure` can edit any rank. Command promotions must meet
  the rules (time in the current rank; the new rank's qualifications; a probation confirmation skips
  the time rule). The top rank also sets the faction's settings above. Gear lists wait for loadouts
  to move out of the mission config.

Offences, training items, divisions and qualifications are seeded from the layout plan (Police) and
edited in the database until they have an editor. EMS has offences and a starter training sheet,
but no divisions or qualifications yet.

### 6.2 Faction applications (#29)

Gamepanel's "waiting list" simply listed every player not in a faction. We replace it with
`faction_applications` (player, faction, short answers, status). Players apply from their
dashboard, and command sees their own faction's queue. That gives command an actual intake
process instead of a list of every civilian.

### 6.3 Rank names (#27)

```sql
CREATE TABLE faction_rank_names (
    faction  TEXT NOT NULL CHECK (faction IN ('police', 'ems')),
    level    INTEGER NOT NULL,
    name     TEXT NOT NULL,
    PRIMARY KEY (faction, level)
);
```

Wherever a raw `cop_level` / `medic_level` number is shown today (dashboard, player profile,
roster), show the name instead. Staff edit these under `factions.configure`.

## 7. Knowledge & communication

### 7.1 Meeting minutes & recordings (#33)

Add `minutes TEXT` and `recording_url TEXT` (validated as http/https) to `staff_meetings` from
[OPERATIONS.md §4](OPERATIONS.md#4-staff-meetings). Recordings are linked, not uploaded, so the
website doesn't have to host media.

### 7.2 Public policies & private notebook (#34, #35)

Add a `visibility` column to `staff_wiki_pages` from
[OPERATIONS.md §5](OPERATIONS.md#5-internal-wiki): `public` pages appear to everyone at `/rules`
(Gamepanel's `policies.php`), `staff` pages are internal, and `private` pages are visible only to
their author (Gamepanel's notebook). That's one editor and one table for all three.

### 7.3 Notifications with acknowledgement (#36)

```sql
CREATE TABLE notifications (
    id           BIGSERIAL PRIMARY KEY,
    player_id    BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    body         TEXT NOT NULL,
    link         TEXT,                 -- in-site path only, e.g. '/admin/cases/42'
    essential    BOOLEAN NOT NULL DEFAULT false,
    read_at      TIMESTAMPTZ,
    acknowledged_at TIMESTAMPTZ,       -- essential only: an explicit "I've read this"
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

A bell in the header shows the unread count. An **essential** notification (a policy change, a
new rule) shows a banner on every panel page until it's acknowledged, and staff can see who has
acknowledged it. `link` is restricted to same-site paths, so a notification can't redirect anyone
off-site. The first uses are ticket replies, application decisions, case assignments and policy
updates.

### 7.4 Suggestions (#37)

Add "Suggestion" as a top-level [support ticket category](WEBSITE.md#8-support-panel). The
existing ticket system already provides submission, staff triage, threaded replies and status.
A separate suggestion box would duplicate all of that.

## 8. Game server operations

Both features depend on `src/server_manager` (the operator's desktop tool, which already manages
the Arma process and reads its logs) exposing a small **authenticated local API**. The website
must never start or kill processes itself. The website and server manager may run on different
machines, so the manager is the only thing that touches the game process.

### 8.1 Restart control (#40)

Restart / stop / start for each configured server (main, dev). Head Admin only
(`server.control`). A restart requires typing the server name to confirm, and an optional
in-game warning countdown (broadcast → wait → restart). Each action goes to `staff_log` and the
Discord staff-log webhook. There's a rate limit of one restart per server per 5 minutes.

### 8.2 Live logs (#41)

A read-only live tail of the server's log files (RPT, BattlEye, server-side scripts) streamed from
`server_manager` over server-sent events. Senior staff only (`server.logs`). Log lines are
HTML-escaped before rendering. Gamepanel did this by hand with a hand-written escape function; we
rely on `html/template`. Any lines that show IP addresses are masked for everyone below Head Admin.

## 9. Decision needed: paid reserved slots (#43)

Gamepanel sold reserved slots activated with a licence key. Before planning anything like it:
Bohemia Interactive's Arma 3 server monetisation rules restrict what servers may charge for
(broadly: no pay-to-win, and cosmetics/priority must be approved). Reserved-slot or queue-priority
sales may need their approval. **Recommendation:** leave this out of the build plan until you
decide whether the server will take money at all. If it will, check the current BI rules first,
then design it with the payment provider's own licensing (e.g. Tebex) rather than hand-made keys.

## 10. Not carried over, and why

Beyond the Skip rows above, some Gamepanel code patterns are deliberately not reproduced:

- **`temprunquery.php` / `test.php`**: ad-hoc endpoints that run database queries. Any database
  access from the web goes through the read-only, audited design in the layout plan's Database
  browser, never a raw query endpoint.
- **A default admin login in the README**: TasDyn-ALife has no passwords at all (Steam login), so
  there's no default credential to leak.
- **Permissions stored as JSON arrays on a rank row**: works, but nothing stops a typo'd key from
  silently granting nothing. The permission catalogue (§2.1) makes an unknown key impossible to
  grant.
- **Overwritable notes and editable case text**: replaced with append-only history throughout
  (§2.2, §3.1).

## 11. Build order

Continues [OPERATIONS.md §7](OPERATIONS.md#7-timeline-impact--the-honest-part), whose step 1 (staff
lifecycle) shipped in PR #111. Each wave builds on the one before it; within a wave, items are
independent.

| Wave | Features | Why this order |
|---|---|---|
| **1. Staff core** | Role editor (2.1), staff directory & profiles (2.2), LOA/suspend UI (2.3), staff log filters (2.4), player lookup + vehicles ([WEBSITE §7](WEBSITE.md#7-admin-panel), 5.1) | Everything later assigns work to staff or acts on a player. These are the screens that everything else links to. |
| **2. Discipline** | Cases ([OPERATIONS §2](OPERATIONS.md#2-structured-disciplinary-cases)), punishment points (3.1), bans from cases (3.2), case list & search (3.3), activity & stats (3.4), ban/unban (WEBSITE §7) | The highest day-to-day value for moderation. Needs wave 1's player and staff pages. |
| **3. People pipeline** | Staff applications (4.1), interviews (4.2), notifications (7.3), faction rank names (6.3) | Notifications arrive here because applications are the first feature that needs to notify someone. |
| **4. Factions** | In-faction permission check + faction records ([OPERATIONS §6](OPERATIONS.md#6-faction-records-the-new-idea--player-facing-not-staff-facing)), faction command panel (6.1), faction applications (6.2), level edits (5.2) | Builds the new permission type once and uses it for all three features. |
| **5. Knowledge** | Wiki with public/staff/private pages (OPERATIONS §5 + 7.2), meetings + minutes (OPERATIONS §4 + 7.1), suggestions category (7.4) | Lowest urgency. Docs/Discord cover these until then. |
| **6. Operations & economy** | Compensation (5.3), economy dashboard (5.4), item prices (5.5), server restart (8.1), live logs (8.2), read-only Database browser (layout plan) | 8.x waits on the `server_manager` API. 5.5 waits on Phase 4's economy config. |

No dates here on purpose: [ROADMAP.md](ROADMAP.md) is the only place dates live. Folding waves 1–6
into Phase W needs its own pass there once the scope is confirmed.
