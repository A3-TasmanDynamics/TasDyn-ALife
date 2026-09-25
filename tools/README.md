# tools/

Dev tooling that doesn't belong inside `src/` (which is shipped code) or `docs/` (which is
reference material).

## `test_local_server.ps1`

Deploys the mission + C++ extension to a local Arma 3 Server install, launches it headless, and
reports whether the mission's config (`description.ext`, `CfgFunctions.hpp`, `CfgRemoteExec.hpp`,
...) parsed without errors — catches the thing most likely to silently break from a typo in a
`.hpp`/`.ext` file with no other way to find out early. Cleans up everything it copies afterward;
never leaves files in the Arma 3 Server install, which is a Steam-managed directory outside this
repo.

```powershell
.\tools\test_local_server.ps1 -ArmaServerPath "C:\path\to\Arma 3 Server"
```

**Does not test a real player joining** (`load`/`save`/spawning) — that needs an interactive
client connecting, which this script can't drive. It's a config-parses-cleanly smoke test, not a
gameplay test.

Prerequisites: `src/cpp_extension/build.ps1` already run, and a real `config.ini` at the repo root
(copied from `config.ini.example`).
