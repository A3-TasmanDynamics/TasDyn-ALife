# src/cpp_extension/

The C++ bridge between Arma 3 (`callExtension`) and PostgreSQL, built on raw **libpq** (the C
client library that ships with the PostgreSQL server install — no vcpkg/extra tooling needed).

Rules for this extension:
- All queries use `PQprepare` + `PQexecPrepared` (prepared statements). Never build SQL by
  concatenating strings from `callExtension` input.
- Any string returned to SQF has a fixed, documented field order — see
  [docs/DATA_CONTRACT.md](../../docs/DATA_CONTRACT.md). The previous prototype lost rank data to
  exactly this kind of undocumented positional mismatch; don't add a new command here without
  updating that doc first.

## Building

`build.ps1` locates the Visual Studio x64 dev environment and `pg_config` (for the libpq
include/lib paths) automatically:

```powershell
./build.ps1
```

Produces `build/tasdyn_alife_x64.dll` — the `_x64` suffix is required (see
[callExtension](https://community.bistudio.com/wiki/callExtension)'s 64-bit extension naming
rule); Arma resolves it automatically from the SQF-side name `"tasdyn_alife"`.
