# Contributing to TasDyn-ALife

## Branching

- `main` is always deployable. Nobody commits to it directly, including maintainers.
- Branch names are prefixed by intent: `feature/<slug>`, `fix/<slug>`, `docs/<slug>`, `chore/<slug>`.

## Pull requests

- Every change lands via PR, reviewed before merge.
- Never commit real credentials. Copy `config.ini.example` to `config.ini` locally — it's git-ignored.
- Never commit build output (`build/`, `*.dll`, `*.pbo`) or the `server_dist/` deploy tree — see [.gitignore](.gitignore).

## SQF command verification

Before writing or modifying any SQF — including single-line edits — verify every command's
syntax, parameter order, return type (especially null/`objNull` cases), and `since` version
against the local reference: `python docs/arma/search_arma3.py --exact "<command name>"`. See
`docs/arma/CLAUDE.md`. Flag uncertainty rather than assuming if the lookup comes back empty or
truncated.

## The data contract

`database/`, `src/cpp_extension/`, and `src/ALife.Altis/` share one save/load array format across
Postgres columns, the C++ bridge, and SQF. If a column is renamed, reordered, or added, update the
documented contract in `docs/` and all three sides in the same PR — an undocumented version of
exactly this contract is what corrupted rank data in the prototype this project replaced.

## Delivery board

Every issue and PR opened anywhere in the `A3-TasmanDynamics` org is auto-added to the
[Tasman Dynamics — Delivery Board](https://github.com/orgs/A3-TasmanDynamics/projects/1) via
`.github/workflows/sync-to-project.yml`.
