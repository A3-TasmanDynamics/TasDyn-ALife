# Data Contract: Player Save/Load

**Last changed: 2026-09-26.**

This is the single source of truth for the fields shared by the Postgres schema
(`database/schema.sql`), the C++ extension's `RVExtensionArgs` output, and the SQF code that
parses it. The prototype this project replaced corrupted rank/admin data because this exact
contract only ever existed implicitly, split across three files that drifted out of sync — see
the postmortem referenced in the README. **Any change to a field's name, presence, or type below
is a breaking change to the C++ and SQF sides simultaneously — update this doc, the schema, the
C++ code, and the SQF code in the same PR** (per [CONTRIBUTING.md](../CONTRIBUTING.md)).

## Wire format: key-value, not positional

`callExtension` always returns a single string to SQF — there's no native array return (see
`RVExtensionArgs`'s `outputSize` limit). Earlier drafts of this doc used a fixed-position
pipe-delimited string, the same shape the prototype's bug happened in. Once the player record
grew to ~20 fields (civilian/police/medic each carrying their own level/dept/licence/gear/cash),
a positional format became exactly the risk this project exists to avoid — a field inserted or
reordered silently shifts every index after it.

Instead, `load`'s response is a **key-value array**, built via SQF's own intended pattern for
this exact situation:

```sqf
private _data = extensionOutputString call parseSimpleArray;      // string -> [[key,value], ...]
private _record = _data createHashMapFromArray;                    // -> HashMap, keyed by field name
```

(`parseSimpleArray`'s own documentation states it's "primarily intended for use with
`callExtension` to parse the String output into Array" — this isn't a workaround, it's the
supported path.) A field being added, removed, or reordered on the C++ side can't silently
corrupt an unrelated field on the SQF side anymore — SQF looks fields up by name, not position.

`save` doesn't have this problem on its input side — `callExtension`'s array-argument form
already passes a real SQF array in, no string-parsing involved. Its response is a plain status
string (no data to key), so it stays simple.

### Why array-of-pairs, not objects

`parseSimpleArray`'s own grammar is explicit: "array consisting of Numbers, Strings, Booleans and
Arrays of all of the above" — there is no object/map literal (`{...}`) in it at all. So a JSONB
column like `gear` can't be handed to SQF as a JSON object; it has to be an array of `[key, value]`
pairs instead — `[["primaryWeapon","arifle_MX_F"], ...]`, never `{"primaryWeapon": "..."}`. This
is enforced at the schema level too (see the JSONB SHAPE RULE comment at the top of
`database/schema.sql`), not just documented here and hoped for: every JSONB column that can cross
this boundary is constrained to that shape, which means Postgres's raw JSONB text is *already*
valid `parseSimpleArray` input — no JSON-object-to-SQF-array conversion code needed on the C++
side at all.

### Why no `null`

Same grammar, same reasoning: there's no null/nil literal in a "simple array" either. Where SQL
would naturally use `NULL` (no department assigned yet, no death ever recorded), this contract
uses the type's own empty value instead — empty string `""` for text, empty array `[]` for
array-of-pairs fields — rather than inventing an unsupported representation. The schema backs this
up: `civ_position`/`cop_position`/`medic_position` are `NOT NULL DEFAULT '[]'::jsonb`, not
nullable columns.

## `load` — full record on join

Request (SQF → C++): `"tasdyn_alife" callExtension ["load", [uid]]`

| Arg | Type | Notes |
|---|---|---|
| `uid` | string | The player's Arma UID (Steam64, 17 digits) — unique key into `players.uid`. Arma delivers string args in `str` form (quoted, inner quotes doubled); the extension unwraps them (`UnwrapArmaString`) and rejects any uid that isn't 17 digits with `ERROR:invalid uid`. |

Response (C++ → SQF): a `parseSimpleArray`-compatible string encoding `[[key, value], ...]` pairs.
`status` is always present and must be checked before trusting any other key — `OK` or `ERROR`.
On `OK`, every key below is present:

| Key | Type | Source | Notes |
|---|---|---|---|
| `uid` | string | `players.uid` | Echoed back so SQF can sanity-check the response matches the request |
| `name` | string | `players.name` | See [Escaping](#escaping) |
| `staff_rank_key` | string | `staff_ranks.key` via `players.staff_rank_id` | Empty string if `staff_rank_id IS NULL` |
| `staff_level` | number | `staff_ranks.level` via the same join | `0` if no staff rank |
| `civ_cash` / `cop_cash` / `medic_cash` | number | `players.<faction>_cash` | Physical cash on hand, per faction |
| `civ_bank` / `cop_bank` / `medic_bank` | number | `players.<faction>_bank` | **Cache** — `bank_accounts.balance` is authoritative, this is kept in sync by DB trigger (see `database/schema.sql`), never written independently by application code |
| `cop_level` / `medic_level` | number | `players.cop_level` / `players.medic_level` | Civilian has no level (no `civ_level` key) |
| `cop_dept` / `medic_dept` | string | `players.cop_dept` / `players.medic_dept` | Empty string if not yet assigned — **not `null`**, see [Why no `null`](#why-no-null) |
| `civ_licence` / `cop_licence` / `medic_licence` | array of strings | `players.<faction>_licence` (JSONB) | e.g. `["driver","boat"]` — a set, not a single value |
| `civ_gear` / `cop_gear` / `medic_gear` | array of `[key, value]` pairs | `players.<faction>_gear` (JSONB) | Full loadout + virtual items, e.g. `[["primaryWeapon","arifle_MX_F"],["uniform","U_B_CombatUniform_mcam"]]` — **not a JSON object**, see [Why array-of-pairs, not objects](#why-array-of-pairs-not-objects) |
| `civ_alive` / `cop_alive` / `medic_alive` | boolean | `players.<faction>_alive` | **Must be checked on spawn.** A player who disconnects dead/unconscious has to resume dead at `*_position`, not respawn fresh — see the comment on `players` in `database/schema.sql` for why this isn't optional (a well-known disconnect-to-escape exploit in this genre) |
| `civ_position` / `cop_position` / `medic_position` | array of `[key, value]` pairs | `players.<faction>_position` | World position to resume at when `*_alive` is `false`. Empty array (`[]`) until the first death is ever recorded for that faction — **not `null`** |
| `civ_items` / `cop_items` / `medic_items` | array of `[class, count]` pairs | `players.<faction>_items` | Virtual items (e.g. `[["medkit", 3]]`), classes from the mission's `config/CfgItems.hpp`. `[]` when empty |
| `civ_bounty` | number | `players.civ_bounty` | **Cache** — authoritative source is `wanted_crimes` (sum of outstanding, i.e. `cleared_at IS NULL`, rows), synced by trigger. No `cop_bounty`/`medic_bounty` — the wanted list is civilian-only |

**`BANNED`.** If `banlist` has an active row for `uid` (no `expires_at`, or one in the future; lifting a
ban sets `expires_at` to the lift time), the response is `[["status","BANNED"],["reason",…],["until",…]]`
instead, and no blank record is created. `until` is `""` for a permanent ban, otherwise e.g.
`"27 Oct 2026 14:00 UTC"`. The server must not spawn the player (`fn_playerJoin.sqf` shows the reason
on the loading screen and stops there). Bans are issued and lifted from the website (Admin Panel → Bans).

If no row exists for `uid`, the C++ side creates a blank record (all `*_cash`/`*_bank` = 0, empty
`*_licence`/`*_gear`, `status = 'active'`, no staff rank, and a matching `bank_accounts` row per
faction) *before* building the response — a new player and an existing player get the exact same
response shape, never a different one for the "just created" case. That divergence-on-first-join
is a classic place for this kind of contract to quietly drift.

**Not returned by `load`**: `players.status` (account status — a `banned` player shouldn't reach
this code path at all; that's enforced earlier, at connect time, not surfaced here), `last_seen`
and `*_playtime_seconds` (staff/stats-facing, not something the join flow needs to act on), and
anything from `bank_transactions`/`player_log`/`wanted_crimes`/etc. — this call is the live record
only, not history.

## `save` — persist one field

Request (SQF → C++): `"tasdyn_alife" callExtension ["save", [uid, field, value, requestToken]]`

| Arg | Type | Notes |
|---|---|---|
| `uid` | string | Same as above. |
| `field` | string | **An allowlist, not an arbitrary column name** — one of: `name`, `civ_cash`, `cop_cash`, `medic_cash`, `civ_licence`, `cop_licence`, `medic_licence`, `civ_gear`, `cop_gear`, `medic_gear`, `cop_level`, `medic_level`, `cop_dept`, `medic_dept`, `civ_alive`, `cop_alive`, `medic_alive`, `civ_position`, `cop_position`, `medic_position`, `civ_items`, `cop_items`, `medic_items`. Validated server-side against this known set before it ever reaches a query — the same "never trust client input as authoritative" principle as [ANTI_CHEAT.md Layer 1](ANTI_CHEAT.md#layer-1--api-surface-remoteexec-allowlist). |
| `value` | string | Meaning depends on `field` — see [Delta vs. absolute-set fields](#delta-vs-absolute-set-fields) below. Always parsed and range/type-checked server-side before use — never interpolated into SQL. For array-of-pairs fields (`*_gear`, `*_position`), this is SQF's `str` of the array — e.g. `str [["primaryWeapon","arifle_MX_F"]]` — which happens to already be valid JSON text, but the C++ side still structurally validates it before writing to the JSONB column rather than trusting that it is. |
| `requestToken` | string | Idempotency token — required for `civ_cash`/`cop_cash`/`medic_cash`, ignored for every other field (see below). A token seen again for the same player is rejected, not re-applied — checked against `applied_request_tokens` in the same transaction as the cash update, so a check-then-insert race can't slip a duplicate through. |

**`civ_bank`/`cop_bank`/`medic_bank` are deliberately not in the `save` allowlist.** Bank balance
changes go through the `bank` command below (writing `bank_transactions` rows against
`bank_accounts`) — never a direct field write, since a balance is a ledger-derived number, not
free-standing state.

### Delta vs. absolute-set fields

- **`civ_cash` / `cop_cash` / `medic_cash`: `value` is a signed delta**, not a new total — e.g.
  `"-500"` to spend 500. Applied as `civ_cash = civ_cash + delta` (rejected if it would go
  negative), atomically with the idempotency check above. This is deliberate, not an oversight:
  physical cash has the exact same concurrent-double-apply risk that `bank_accounts` was split
  into its own ledger to avoid (two near-simultaneous earns both reading the same starting total
  and racing to overwrite each other) — an absolute-set `save` on cash would just reintroduce that
  risk without a ledger to catch it. See `applied_request_tokens` in `database/schema.sql`.
- **Every other allowlisted field: `value` is the new absolute value.** These don't need the
  token check — setting `cop_dept` or `civ_gear` to the same value twice in a row is harmless,
  unlike money, so there's no duplication class to guard against here.

One field per call, not the whole record — keeps this contract small and avoids ever reintroducing
a wide positional array on the request side either.

Response: plain string, one of `OK`, `ERROR`, or `DUPLICATE` (the idempotency-token-rejected
case, cash fields only). No data payload — there's nothing to echo back for a single-field write.

## `bank` — bank ledger changes

Args: `[op, uid, faction, amount, requestToken, otherUid, otherFaction, memo]`.

| `op` | Does |
|---|---|
| `purchase` | Debits `uid`'s `faction` account by `amount` (`bank_transactions.type = 'purchase'`). `otherUid`/`otherFaction` are `""`. |
| `transfer` | Moves `amount` from `uid`'s `faction` account to `otherUid`'s `otherFaction` account (`transfer_out` + `transfer_in`). |
| `deposit` | ATM: moves `amount` from `players.<faction>_cash` into `uid`'s `faction` account (`deposit`), only if the cash is there. |
| `withdraw` | ATM: moves `amount` from `uid`'s `faction` account into `players.<faction>_cash` (`withdrawal`), only if the balance covers it. |

`faction` is `civ`/`cop`/`medic` (mapped to `bank_accounts.faction` civilian/police/medic).
`amount` is a positive whole number. Both accounts are locked (in id order, like the website's
`bank.moveMoney`) and the debit is only written if the balance covers it; the DB triggers then
update `bank_accounts.balance` and `players.<faction>_bank`. `requestToken` is required — a repeat
is rejected by `idx_bank_transactions_token`. Response: `OK`, `INSUFFICIENT`, `DUPLICATE`,
`NO_ACCOUNT`, or `ERROR[:reason]`.

## `combatlog` — disconnected while incapacitated

Args: `[uid, faction, killer, weapon, distance, secondsIncapacitated]` (`killer`/`weapon`/`distance`
are `""` when nobody else was involved). Writes, in one statement, an `anti_cheat_flags` row
(`combat_log`, high) and a `staff_log` row (`player.combat_log`, source `game`), both with the
details as JSON. Response: `OK` or `ERROR:reason`.

## `session` — who's online

Args: `[op, uid]`. `start` opens a `player_sessions` row for `uid` (closing any left open first),
`end` closes it, `reset` (no uid) closes every open row — the server calls it at start, when nobody
is connected yet. The website's online counts, player status and `players.last_seen` (trigger)
read these rows. Response: `OK` or `ERROR:reason`.

## `market` — dynamic market prices

The current price of each virtual item, in `market_prices` (one row per item; an item with no row
is at its `CfgMarket` base price). The game server keeps live prices in memory
(`ALife_fnc_market`) and writes each change through this command.

- `["load"]` returns every saved price as an SQF array, `[["redgull",1500.00],["water",11.00]]`,
  or `[]`.
- `["set", item, price]` upserts one price. `item` must be a config class name
  (`[A-Za-z0-9_]{1,64}`) and `price` a positive decimal with at most two places, or the answer is
  `ERROR:invalid item` / `ERROR:invalid price` before anything reaches the database.
- `["reset"]` deletes every row: all items go back to their base price on the next server start.
  This is the wipe or economy reset.

Response: the array for `load`, otherwise `OK` or `ERROR:reason`.

## `vehicle` — bought vehicles

- `["add", uid, faction, classname, price]` records a vehicle the player just bought, in one statement:
  a `vehicles` row (owner, classname, `faction_scope` civilian / police / medic, status `spawned`,
  a new unique six-character plate drawn from `A-Z2-9` without `I O 0 1`) and its `purchase`
  `vehicle_logs` row with the price. Response: `OK:<id>:<plate>`. `price` is whole dollars;
  `classname` must be a config class name.
- `["status", id, status]` sets a vehicle's status: `spawned`, `garaged`, `impounded` or `destroyed`.
  Response: `OK`, or `ERROR:no such vehicle`.

Validation errors (`ERROR:invalid uid` / `faction` / `classname` / `price` / `id` / `status`) are
returned before anything reaches the database.

## Gear

`<faction>_gear` is saved by the game server's autosave while the player is alive, as pairs:
`[["loadout", <getUnitLoadout>], ["food", 0-100], ["water", 0-100]]`. Spawning puts the loadout
back on (`setUnitLoadout`) and restores food and water (missing = 100, full); respawning after
death saves `[]`, so the gear stays with the body (a revive keeps it) and the new life starts full.

## Escaping

`name` is the only genuinely free-text field. Any characters that would break `parseSimpleArray`'s
string parsing (unescaped `"` in particular) are sanitized server-side **at write time**
(on `save`/record-creation), not assumed clean at read time. Sanitize once, at the boundary where
untrusted text enters the system — not on every subsequent read.

## What's deliberately not in this version

- `bank_tx` (deposit/withdrawal/transfer) — noted above, needs Phase 2's economy design first.
- `wanted_add`/`wanted_clear` (adding/clearing a `wanted_crimes` row) — same reasoning as `bank_tx`:
  a ledger-style table needs its own command, not a `save` field write.
- Vehicles, houses, gangs — all have their own tables in `database/schema.sql` already, but none
  of them have a documented C++/SQF contract yet. Same rule applies: write it here before writing
  either side of the code.
- `player_sessions`, `staff_log`, `kick_log`, `anti_cheat_flags` — these are written *to* by the
  extension as a side effect of other commands (join/disconnect, admin actions, flag triggers),
  not read back via a dedicated `load`-style call. Each gets documented alongside the command that
  writes it, when that command is built.
