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
include/lib paths) automatically, and copies every runtime dependency next to the built DLL:

```powershell
./build.ps1
```

Produces `build/tasdyn_alife_x64.dll` — the `_x64` suffix is required (see
[callExtension](https://community.bistudio.com/wiki/callExtension)'s 64-bit extension naming
rule); Arma resolves it automatically from the SQF-side name `"tasdyn_alife"`.

### Runtime dependencies

Dynamically linking against libpq means these have to ship next to the DLL (`build.ps1` copies
them from the local PostgreSQL install automatically — this list is the *complete* transitive
chain, verified with `dumpbin /dependents`, not just libpq's own direct imports):

`libpq.dll`, `libssl-3-x64.dll`, `libcrypto-3-x64.dll`, `libintl-9.dll`, `libwinpthread-1.dll`,
`libiconv-2.dll`

The last two are easy to miss — they're not direct dependencies of `libpq.dll` itself, but of
`libintl-9.dll` one level down (`libintl-9.dll` is a MinGW-built library, not MSVC — it needs
`libwinpthread-1.dll`/`libiconv-2.dll` where an MSVC-built DLL wouldn't). Everything else
(`KERNEL32.dll`, `WS2_32.dll`, the `api-ms-win-crt-*` apiset stubs, ...) is a standard Windows
system DLL, present on any Windows 10/11 host.

### Testing without an Arma 3 install

`test_harness.cpp` (build with `cl.exe ..\test_harness.cpp /Fe:test_harness.exe` from inside
`build/` after `vcvars64.bat`) loads the DLL with `LoadLibrary`/`GetProcAddress` and calls its
exports exactly the way Arma's `callExtension` does — proves the extension works end to end
(including the real Postgres connection) without needing Arma installed:

```powershell
.\test_harness.exe ping
```
