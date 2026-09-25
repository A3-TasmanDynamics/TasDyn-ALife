# Claude Instructions

## SQF Database Research (Required)

Before writing or modifying any SQF code — no matter how small the change — always search the Arma 3 documentation database to verify every command used, its syntax, return types, and version availability. This applies to all edits, including single-line fixes and refactors.

The database is at `docs/arma/arma3.db`. Use the search script:

```bash
python docs/arma/search_arma3.py "<command name>"
```

Or query the database directly for full details:

```bash
python -c "
import sqlite3
conn = sqlite3.connect('docs/arma/arma3.db')
cur = conn.cursor()
cur.execute(\"SELECT title, syntax, params, returns, descr FROM commands WHERE title = '<command>'\")
row = cur.fetchone()
if row:
    print('Syntax:', row[1])
    print('Params:', row[2])
    print('Returns:', row[3])
    print('Descr:', row[4])
conn.close()
"
```

Always verify:
- The command exists in Arma 3 (not ArmA 2 only)
- The correct syntax and parameter order
- What the command returns (especially null/objNull cases)
- The `since` version to ensure compatibility

If the database returns no result or truncated info, flag the uncertainty rather than assuming.