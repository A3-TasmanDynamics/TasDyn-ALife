# website/

The public site, member portal, Admin Panel, and Support Panel -- see
[docs/WEBSITE.md](../../docs/WEBSITE.md) for the full design. This is the initial scaffold: auth,
sessions, panel access gating, and a working (if minimal) version of each surface, built and
smoke-tested end-to-end against a real local Postgres instance. What's explicitly not built yet is
called out inline in `internal/handlers/admin.go` and `internal/discord/bot.go`.

## What's here

- **Auth**: "Sign in with Steam" (OpenID 2.0, `internal/auth/steam.go`) is the primary login --
  it's what creates a `players` row at all, since `players.uid` is the Steam64 ID the game itself
  uses. Discord (OAuth2, `internal/auth/discord.go`) is a *second* login method, valid only for an
  account that has already linked a Discord identity -- a brand-new visitor still has to start with
  Steam.
- **Discord account linking, two ways**: from the website (`/auth/discord/connect`, OAuth2) or from
  Discord itself (a `/link <code>` bot command redeeming a code the website generated) --
  `internal/auth/discordlink.go`, `internal/discord/bot.go`.
- **Sessions**: DB-backed (`web_sessions`), not JWT -- a session row can be deleted to log someone
  out immediately (a ban, a rank change), which a signed token can't do without its own revocation
  table anyway.
- **Panel access**: resolved at login from `staff_ranks.default_admin_panel`/
  `default_support_panel` plus any `staff_permission_overrides` rows for `panel.admin`/
  `panel.support` -- see `internal/auth/session.go`'s `resolvePanelAccess`. A staff member holding
  both grants sees a switcher link on both panel pages.
- **Support tickets**: full create → claim → reply → close lifecycle, shared between the member
  portal (`/tickets`) and the Support Panel (`/support`) -- one `support_tickets` row, two access
  levels, per `docs/WEBSITE.md` §8. New tickets post to Discord via `internal/discord/webhook.go`
  if `DISCORD_TICKET_LOG_WEBHOOK` is configured.
- **Member dashboard**: read-only faction stats and gang info.
- **Admin Panel**: currently a read-only `staff_log` viewer plus the access-gated route itself --
  player lookup, ban/unban, rank management, anti-cheat review, and the arsenal editor are designed
  in `docs/WEBSITE.md` §7 but not yet built.

## Not yet built

- Member-portal bank transfers and gang invite/remove/rank actions (`docs/WEBSITE.md` §5) -- the
  `fn_save.sqf`/`*_bank` cache question flagged there needs resolving first.
- Two-way Discord ticket-thread sync (a thread per ticket, mirrored replies) -- `internal/discord/bot.go`
  has the account-linking half of the bot; ticket sync is the documented next piece.
- CSRF tokens on state-changing forms (`docs/WEBSITE.md` §10) -- needed before this is
  internet-facing, not needed to develop against locally.
- Rate limiting on login/money-moving endpoints.

## Running locally

```bash
cp .env.example .env   # fill in DATABASE_URL if it differs from the default alife_admin/alife_db
go run .
```

Requires the schema additions in `database/schema.sql` (search for "Website" in that file) applied
to your local DB -- same `psql -f database/schema.sql` as any other schema change, or apply just
the new pieces if your DB already has the rest.

Discord OAuth login/connect and the `/link` bot command are both optional at runtime -- leave
`DISCORD_OAUTH_CLIENT_ID`/`DISCORD_BOT_TOKEN` empty and the site runs fine without them (Steam login
and the website-generated link code still work; the bot-side half of linking just won't be
available). See `.env.example` for what each variable is for and where to get it.

## Smoke-testing without a real Steam/Discord login

Steam OpenID and Discord OAuth both require a real account and a reachable callback URL, so they
can't be curl'd directly in local dev. To exercise everything downstream of login (dashboard,
tickets, Admin/Support Panels, access gating) without going through either flow, insert a session
row directly:

```sql
INSERT INTO players (uid, name, staff_rank_id, status)
VALUES ('76561198000000001', 'DevTest', (SELECT id FROM staff_ranks WHERE key = 'head_admin'), 'active');

-- token_hash = sha256("some-raw-token"); the cookie value is the raw token, never the hash
INSERT INTO web_sessions (player_id, token_hash, admin_panel_access, support_panel_access, expires_at)
VALUES (currval('players_id_seq'), '<sha256 hex of "some-raw-token">', true, true, now() + interval '1 day');
```

Then `curl -H "Cookie: alife_session=some-raw-token" http://localhost:8080/dashboard`. Delete the
row afterward -- this is a bypass for local testing, not a real login path.
