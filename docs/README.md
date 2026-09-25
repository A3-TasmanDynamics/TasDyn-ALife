# docs/

Design docs and architecture decisions.

| Doc | What's in it |
|---|---|
| [ROADMAP.md](ROADMAP.md) | Phased plan and timeline toward public launch |
| [ANTI_CHEAT.md](ANTI_CHEAT.md) | Threat model, defense layers, and what's explicitly out of scope |
| [ADMIN_TOOLS.md](ADMIN_TOOLS.md) | In-game staff menu spec and the DB-backed staff rank system |

Most importantly, this is also where the **data contract** between the database schema, the
C++ extension's return formats, and the SQF functions that parse them belongs (`DATA_CONTRACT.md`,
tracked in [Phase 0 of the roadmap](ROADMAP.md#phase-0--foundations-2026-09-25--2026-10-05)).
Write the contract down here before implementing either side of it.

## arma/

A local, offline SQF command reference (`arma3.db`, indexed from `arma3Documentation.xml` via
`build.ps1`/`index_arma3.py`). Before writing or modifying any SQF — no exceptions for small
edits — verify every command's syntax, params, return type, and `since` version against it:

```bash
python docs/arma/search_arma3.py --exact "<command name>"
```

See `docs/arma/CLAUDE.md` for full usage. If the database has no result or a truncated entry,
flag the uncertainty rather than assuming.
