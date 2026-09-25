# database/

PostgreSQL schema for the ALife backend. `schema.sql` is the one canonical
schema — do not let a second, drifting copy accumulate here.

Column names defined here are the contract the C++ extension (`src/cpp_extension`)
and the SQF save/load functions (`src/mission`) both code against — see
[docs/DATA_CONTRACT.md](../docs/DATA_CONTRACT.md) for the `players` table's
field order, and [docs/ADMIN_TOOLS.md §3](../docs/ADMIN_TOOLS.md#3-data-model)
for the staff/admin/arsenal tables. If a column is renamed or reordered,
update the relevant doc and all affected sides in the same PR.

Apply it to a local dev database with:

```bash
psql -U alife_admin -d alife_db -f database/schema.sql
```
