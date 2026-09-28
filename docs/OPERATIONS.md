# Staff Operations & Faction Records

A real, in-house replacement for the Google Docs/Sheets a staff team ends up using for case
tracking, meeting notes, and SOPs when nothing purpose-built exists — plus a genuinely new idea:
letting **players**, not just staff, log their own faction-duty records (arrests, incidents, gang
activity) through the same website. Inspired by reviewing an old personal project
([Gamepanel](https://github.com/KieranHolroyd/Gamepanel), a PHP staff panel built for a previous
Arma Life server) for feature ideas — not its code, which is a different stack entirely (PHP/MySQL
vs. this project's Go/Postgres) and was never itself finished/hardened enough to port.

This is a genuine scope increase on top of [WEBSITE.md](WEBSITE.md) and
[ADMIN_TOOLS.md](ADMIN_TOOLS.md) — see [§7](#7-timeline-impact--the-honest-part).

For the complete Gamepanel feature-by-feature mapping (all 44 features, not just the five here)
and the overall build order, see [GAMEPANEL_PARITY.md](GAMEPANEL_PARITY.md).

## 1. Scope note

Five features, two audiences:

- **Staff-only**: structured disciplinary cases (§2), staff lifecycle status (§3), staff meetings
  (§4), an internal wiki (§5).
- **Faction members** (not staff — any player currently holding a police/EMS rank, or in a gang):
  faction records (§6) — arrest reports, incident logs, gang activity. This is the part that isn't
  staff tooling at all; it's a roleplay records system (the genre's usual "MDT"), gated on **in-game
  rank**, not staff permission. That's a permission dimension this project hasn't needed before —
  see §6.

Faction *ranks* (Police Constable → Sergeant progression) are a separate, still-unbuilt concept
noted in [ADMIN_TOOLS.md §1](ADMIN_TOOLS.md#1-scope-note) — this doc's faction *records* don't
depend on that existing; they gate on the numeric `cop_level`/`medic_level` columns already in
`players` today.

## 2. Structured disciplinary cases

The gap this closes: `staff_log` (already built) is a flat, one-row-per-action audit trail — good
for "what did this staff member do," bad for "show me everything tied to this investigation," since
a single case (a report → an investigation → a ban → a follow-up) is several `staff_log` rows with
no relationship between them.

```sql
CREATE TABLE staff_cases (
    id            BIGSERIAL PRIMARY KEY,
    case_type     TEXT NOT NULL,                 -- 'ban_appeal' / 'report' / 'investigation' / 'other'
    status        TEXT NOT NULL DEFAULT 'open',   -- 'open' / 'closed'
    summary       TEXT NOT NULL,
    lead_staff_id BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at     TIMESTAMPTZ
);

-- Every player involved, in whatever capacity -- a case can have more than
-- one subject (e.g. a group ban) or more than one staff member (lead +
-- assisting), so this is a join table, not columns on staff_cases.
CREATE TABLE staff_case_participants (
    id       BIGSERIAL PRIMARY KEY,
    case_id  BIGINT NOT NULL REFERENCES staff_cases(id) ON DELETE CASCADE,
    player_id BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    role     TEXT NOT NULL  -- 'subject' / 'reporter' / 'witness' / 'assisting_staff'
);

-- The actual timeline within a case -- notes, evidence links, and (via
-- action_ref) a pointer to whatever concrete thing happened, so this table
-- doesn't duplicate banlist/staff_log, just ties their rows to a case.
CREATE TABLE staff_case_entries (
    id           BIGSERIAL PRIMARY KEY,
    case_id      BIGINT NOT NULL REFERENCES staff_cases(id) ON DELETE CASCADE,
    author_id    BIGINT REFERENCES players(id) ON DELETE SET NULL,
    body         TEXT NOT NULL,      -- plain text, same reasoning as devlog_posts.body
    action_ref   TEXT,               -- e.g. 'banlist:482' or 'staff_log:9931' -- informational, not an FK (those tables' rows can outlive a case or exist without one)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`banlist` gets one addition: `case_id BIGINT REFERENCES staff_cases(id) ON DELETE SET NULL` — a ban
issued as part of a case links back to it directly (a real FK, unlike the informational
`action_ref` above, since `banlist` already exists and is the actual source of truth for bans).

**Permission**: `command_key`s `cases.view` / `cases.create` / `cases.close`, same
`staff_permission_overrides` mechanism as everything else — see
[WEBSITE.md §4](WEBSITE.md#4-panel-access-model).

## 3. Staff lifecycle

Right now `players.staff_rank_id` says *what* a staff member is; nothing says whether they're
currently active. Three columns on `players`, not a new table (this is per-player state, exactly
where `staff_rank_id` already lives):

```sql
ALTER TABLE players ADD COLUMN staff_status TEXT NOT NULL DEFAULT 'active';  -- 'active' / 'suspended' / 'loa'
ALTER TABLE players ADD COLUMN staff_status_reason TEXT;
ALTER TABLE players ADD COLUMN staff_status_until TIMESTAMPTZ;  -- LOA/suspension end date, nullable = indefinite
```

A `suspended`/`loa` staff member keeps their `staff_rank_id` (so nothing else has to change when
they come back) but the Admin Panel's access check gains one clause: `staff_status = 'active'`
required in addition to today's rank/override resolution. Displaying *why* someone's away (the
old project's `errors/youreonloa.php`-style page) is a small, honest touch worth keeping — a staff
member who tries to load a gated page while suspended sees why, not a generic 403.

## 4. Staff meetings

```sql
CREATE TABLE staff_meetings (
    id            BIGSERIAL PRIMARY KEY,
    title         TEXT NOT NULL,
    team          TEXT NOT NULL,   -- 'slt' / 'police' / 'ems' / 'staff' -- which team this meeting is for
    scheduled_at  TIMESTAMPTZ NOT NULL,
    created_by    BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE staff_meeting_agenda_items (
    id          BIGSERIAL PRIMARY KEY,
    meeting_id  BIGINT NOT NULL REFERENCES staff_meetings(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT NOT NULL,
    author_id   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per voter, not a running counter -- lets a vote be changed/undone
-- and lets the UI show who voted which way, not just a total.
CREATE TABLE staff_meeting_votes (
    agenda_item_id BIGINT NOT NULL REFERENCES staff_meeting_agenda_items(id) ON DELETE CASCADE,
    player_id      BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    vote           SMALLINT NOT NULL,  -- 1 or -1, CHECK constraint below
    PRIMARY KEY (agenda_item_id, player_id),
    CHECK (vote IN (1, -1))
);

CREATE TABLE staff_meeting_comments (
    id             BIGSERIAL PRIMARY KEY,
    agenda_item_id BIGINT NOT NULL REFERENCES staff_meeting_agenda_items(id) ON DELETE CASCADE,
    author_id      BIGINT REFERENCES players(id) ON DELETE SET NULL,
    body           TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Permission**: `meetings.view` (scoped further by `team` in application code — a PD meeting isn't
shown to EMS-only staff) / `meetings.create`.

## 5. Internal wiki

```sql
CREATE TABLE staff_wiki_pages (
    id          BIGSERIAL PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,   -- plain text + white-space: pre-wrap, same reasoning as devlog_posts.body
    author_id   BIGINT REFERENCES players(id) ON DELETE SET NULL,
    updated_by  BIGINT REFERENCES players(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Plain text, not markdown/rich text, for the same reason `devlog_posts` is — this is genuinely a
worse editing experience than a real wiki, and worth revisiting once there's evidence staff
actually want structure (headings, lists) badly enough to justify a real (sanitized) markdown
pipeline. Not building that pipeline speculatively.

**Permission**: `wiki.view` / `wiki.edit`.

## 6. Faction records (the new idea — player-facing, not staff-facing)

A police officer, EMS medic, or gang member logs their own duty records — arrests, incidents, gang
activity — through the website. **This is not an admin feature.** The permission check is "does
this player currently hold this in-game rank," read straight from columns that already exist:
`players.cop_level > 0`, `players.medic_level > 0`, or a `gang_members` row — not from
`staff_ranks`/`staff_permission_overrides` at all. A regular civilian player, and most staff, will
never see this section.

```sql
CREATE TABLE faction_records (
    id            BIGSERIAL PRIMARY KEY,
    faction       TEXT NOT NULL,        -- 'police' / 'ems' / 'gang'
    record_type   TEXT NOT NULL,        -- faction-specific: 'arrest' / 'citation' / 'bolo' (police); 'incident' (ems); 'log' (gang)
    author_id     BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    gang_id       INTEGER REFERENCES gangs(id) ON DELETE CASCADE,  -- set only when faction = 'gang'
    title         TEXT NOT NULL,
    body          TEXT NOT NULL,        -- plain text, same reasoning throughout this doc
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Named subjects (arrested player, patient, etc.) -- free text, not a
-- players FK, since a subject may not have a linked account, may be
-- identified only by an in-game name at the time, or may be an NPC-side
-- description with no real player behind it at all.
CREATE TABLE faction_record_subjects (
    id         BIGSERIAL PRIMARY KEY,
    record_id  BIGINT NOT NULL REFERENCES faction_records(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'subject'  -- 'subject' / 'victim' / 'witness'
);
```

Access rules, checked server-side on every request (never trust which nav link the client clicked):
- **Police records** (`faction = 'police'`): visible/writable to any player with `cop_level > 0`.
- **EMS records**: `medic_level > 0`.
- **Gang records**: any `gang_members` row for that `gang_id` — a gang's own log is visible to its
  own members only, never cross-gang, and never to civilians outside it.
- Staff with the right permission (`records.review`) can view across all three for
  moderation/dispute purposes — a separate grant from being in the faction, same pattern as Support
  Panel staff seeing a player's ticket.

This is genuinely new ground for the permission model — every existing check answers "is this
person staff with X grant"; this answers "is this person currently playing faction Y in-game."
Building it means adding that second kind of check to `internal/auth` alongside the existing
`RequireAdminPanel`/`RequireSupportPanel`, not extending `staff_permission_overrides` to cover
something it was never shaped for.

## 7. Timeline impact — the honest part

This is not a small add-on. Five features, one of them (faction records) introducing a permission
model this project didn't have before, on top of a Phase W that
[ROADMAP.md](ROADMAP.md#risk) already flags as the least-proven estimate of the three phases at
risk of slipping. Realistic sequencing, smallest/most-foundational first:

1. **Staff lifecycle** (§3) — a three-column schema change plus one clause in an existing access
   check. Smallest possible slice, and every other staff feature below benefits from it existing
   first (a suspended staff member shouldn't show up as an option to assign a case to).
2. **Structured disciplinary cases** (§2) — the highest day-to-day value of the staff-only features;
   directly replaces ad-hoc note-taking with something searchable and permanent.
3. **Faction records** (§6) — the biggest single build (new permission dimension, three
   faction-specific UIs), but also the most requested and the one with no existing internal
   substitute at all (unlike meetings/wiki, which at least have Docs/Sheets today, however bad).
4. **Staff meetings** (§4) and **internal wiki** (§5) — genuinely useful, lower urgency than the
   above three; a small staff team can keep using Docs/Sheets a while longer for these
   specifically without it actively hurting moderation quality the way an ad-hoc case system does.

No date estimate is given here on purpose — [ROADMAP.md](ROADMAP.md) is the single place dates
live, and folding five more features into Phase W's timeline needs its own explicit pass there,
not a number invented in this doc while the scope is still this fresh.
