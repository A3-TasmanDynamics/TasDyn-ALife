# docs/

Design docs and architecture decisions — most importantly, the data contract
between the database schema, the C++ extension's return formats, and the SQF
functions that parse them. Write the contract down here before implementing
either side of it.

## arma/

A local, offline SQF command reference (`arma3.db`, indexed from `arma3Documentation.xml` via
`build.ps1`/`index_arma3.py`). Before writing or modifying any SQF — no exceptions for small
edits — verify every command's syntax, params, return type, and `since` version against it:

```bash
python docs/arma/search_arma3.py --exact "<command name>"
```

See `docs/arma/CLAUDE.md` for full usage. If the database has no result or a truncated entry,
flag the uncertainty rather than assuming.
