# Data Contract: Player Save/Load

**Last changed: 2026-09-25.**

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

## `load` — full record on join

Request (SQF → C++): `"tasdyn_alife" callExtension ["load", [uid]]`

| Arg | Type | Notes |
|---|---|---|
| `uid` | string | The player's Arma UID — unique key into `players.uid`. |

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
| `cop_dept` / `medic_dept` | string or null | `players.cop_dept` / `players.medic_dept` | Civilian has no department |
| `civ_licence` / `cop_licence` / `medic_licence` | array of strings | `players.<faction>_licence` (JSONB) | e.g. `["driver","boat"]` — a set, not a single value |
| `civ_gear` / `cop_gear` / `medic_gear` | object | `players.<faction>_gear` (JSONB) | Full loadout + virtual items for that faction |

If no row exists for `uid`, the C++ side creates a blank record (all `*_cash`/`*_bank` = 0, empty
`*_licence`/`*_gear`, `status = 'active'`, no staff rank, and a matching `bank_accounts` row per
faction) *before* building the response — a new player and an existing player get the exact same
response shape, never a different one for the "just created" case. That divergence-on-first-join
is a classic place for this kind of contract to quietly drift.

**Not returned by `load`**: `players.status` (account status — a `banned` player shouldn't reach
this code path at all; that's enforced earlier, at connect time, not surfaced here) and anything
from `bank_transactions`/`player_log`/etc. — this call is the live record only, not history.

## `save` — persist one field

Request (SQF → C++): `"tasdyn_alife" callExtension ["save", [uid, field, value, requestToken]]`

| Arg | Type | Notes |
|---|---|---|
| `uid` | string | Same as above. |
| `field` | string | **An allowlist, not an arbitrary column name** — one of: `name`, `civ_cash`, `cop_cash`, `medic_cash`, `civ_licence`, `cop_licence`, `medic_licence`, `civ_gear`, `cop_gear`, `medic_gear`, `cop_level`, `medic_level`, `cop_dept`, `medic_dept`. Validated server-side against this known set before it ever reaches a query — the same "never trust client input as authoritative" principle as [ANTI_CHEAT.md Layer 1](ANTI_CHEAT.md#layer-1--api-surface-remoteexec-allowlist). |
| `value` | string | Parsed and range/type-checked server-side against `field`'s real column type before use — never interpolated into SQL. |
| `requestToken` | string | Idempotency token — see [ANTI_CHEAT.md Layer 2](ANTI_CHEAT.md#layer-2--economic-integrity). A token seen again for this `(account, token)` pair is rejected, not re-applied (enforced by `bank_transactions`' partial unique index, for the banking case). |

**`civ_bank`/`cop_bank`/`medic_bank` are deliberately not in the `save` allowlist.** Bank balance
changes go through a separate `bank_tx` command (deposit/withdrawal/transfer against
`bank_accounts`, writing a `bank_transactions` row) — never a direct field write, since a balance
is a ledger-derived number, not free-standing state. That command gets its own contract entry once
Phase 2's economy work defines the actual transaction types it needs.

One field per call, not the whole record — keeps this contract small and avoids ever reintroducing
a wide positional array on the request side either.

Response: plain string, one of `OK`, `ERROR`, or `DUPLICATE` (the idempotency-token-rejected
case). No data payload — there's nothing to echo back for a single-field write.

## Escaping

`name` is the only genuinely free-text field. Any characters that would break `parseSimpleArray`'s
string parsing (unescaped `"` in particular) are sanitized server-side **at write time**
(on `save`/record-creation), not assumed clean at read time. Sanitize once, at the boundary where
untrusted text enters the system — not on every subsequent read.

## What's deliberately not in this version

- `bank_tx` (deposit/withdrawal/transfer) — noted above, needs Phase 2's economy design first.
- Vehicles, houses, gangs — all have their own tables in `database/schema.sql` already, but none
  of them have a documented C++/SQF contract yet. Same rule applies: write it here before writing
  either side of the code.
- `player_sessions`, `staff_log`, `kick_log`, `anti_cheat_flags` — these are written *to* by the
  extension as a side effect of other commands (join/disconnect, admin actions, flag triggers),
  not read back via a dedicated `load`-style call. Each gets documented alongside the command that
  writes it, when that command is built.
