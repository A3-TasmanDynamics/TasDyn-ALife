# docs/

Design docs and architecture decisions.

| Doc | What's in it |
|---|---|
| [ROADMAP.md](ROADMAP.md) | Phased plan and timeline toward public launch |
| [ANTI_CHEAT.md](ANTI_CHEAT.md) | Threat model, defense layers, and what's explicitly out of scope |

Most importantly, this is also where the **data contract** between the database schema, the
C++ extension's return formats, and the SQF functions that parse them belongs (`DATA_CONTRACT.md`,
tracked in [Phase 0 of the roadmap](ROADMAP.md#phase-0--foundations-2026-09-25--2026-10-05)).
Write the contract down here before implementing either side of it.
