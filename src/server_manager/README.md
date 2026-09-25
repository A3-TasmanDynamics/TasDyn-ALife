# src/server_manager/

A desktop app for actually running the dedicated server day to day — configure the server name,
join/admin passwords, and slot count; launch and stop `arma3server_x64.exe` (deploying the mission
and, best-effort, the C++ extension first — see "Launch actually deploys things now" below); watch
its live log; view staff/anti-cheat/kick logs; and see live CPU, memory, network, and server FPS
graphs for the running server process.

Distinct from `tools/test_local_server.ps1`, which is a one-shot config-parses-cleanly smoke test
for CI/dev work. This is for a real host actually running the server.

## Stack

Go backend (process management, Postgres queries) + a Wails-embedded web frontend
(TypeScript/Vite, Chart.js for graphs) — compiles to a single native Windows `.exe`. Picked over a
native Go GUI toolkit specifically because a web frontend makes decent-looking graphs trivial;
picked over Electron because Wails ships a single small binary instead of bundling a whole
Chromium runtime.

## Building & running

```powershell
wails build   # produces build/bin/server_manager.exe
wails dev     # hot-reloading dev mode, also serves a browser-accessible instance at :34115
```

Needs the Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`), which needs Go
and Node/npm on `PATH`. Run `wails doctor` to check.

## Launch actually deploys things now

Early versions of this app wrote `server.cfg` naming a mission template but never actually put a
mission folder by that name into the Arma 3 Server install's `mpmissions/` — so Arma had nothing to
load, silently. `LaunchServer` (`serverprocess.go`) now calls `deployMission` (`deploy.go`) first,
which copies `src/ALife.Altis` into `<ArmaServerPath>/mpmissions/<template>` fresh on every launch
(same thing `tools/test_local_server.ps1` already did for its own smoke test, now wired into the
real launch path too) — a source change always takes effect on the next launch, not a stale
previously-deployed copy. `deploy.go` finds the repo root by walking up from this executable's own
location looking for `database/schema.sql`, since this tool is still run from inside a repo
checkout (`build/bin/server_manager.exe`), not installed standalone somewhere unrelated to the
source it deploys.

It also best-effort deploys the built C++ extension DLL, its runtime dependencies, and
`config.ini` — unlike the mission (a hard failure if missing, since there's nothing to launch
without it), a missing extension only warns (`LaunchResult.Warnings`, shown on the Launch tab)
rather than blocking launch, since the mission still loads without it; only `ALife_fnc_load`/`save`
calls fail once a player actually joins. Warnings are returned directly from `LaunchServer`, not
pushed as `server:log` console events — those get wiped by the frontend's `clearConsole()` call
right after a successful launch, which would silently lose anything emitted before `cmd.Start()`.

## What's here

```text
src/server_manager/
├── app.go               # App struct -- every exported method is callable from the frontend
├── settings.go           # Arma 3 Server path + Postgres connection, persisted to %AppData%
├── serverconfig.go        # server.cfg fields (name/passwords/slots) + the actual .cfg renderer
├── serverprocess.go        # Launch/stop arma3server_x64.exe, live RPT log tail + performance sampling
├── deploy.go                # Mission + C++ extension deployment (see above)
├── deploy_test.go            # Integration test against a real local Arma 3 Server install
├── logs.go                    # The Postgres queries behind the Logs tab (staff/anti-cheat/kick)
├── logs_test.go                 # Integration test against a real local DB -- see below
└── frontend/
    └── src/
        ├── main.ts            # All five tabs: Launch, Console, Logs, Performance, Settings
        └── style.css           # Dark navy/amber theme, matching the org's brand palette
```

Settings and the server config are stored outside this repo, at
`%AppData%\TasDyn-ALife-ServerManager\` — this app's own state, not something to commit or ship.

## Testing

`logs_test.go` runs the real Logs-tab queries against a live local Postgres instance (skipped by
default, since it needs one):

```powershell
$env:ALIFE_TEST_DB = "1"; go test ./... -run TestLogQueries -v
```

`deploy_test.go` runs the real mission/extension deploy against a live local Arma 3 Server install
(also skipped by default), and cleans up the mission folder it deploys afterward (the DLLs/
`config.ini` it copies are left in place, same as a real launch would leave them):

```powershell
$env:ALIFE_TEST_ARMA_PATH = "G:\SteamLibrary\steamapps\common\Arma 3 Server"; go test ./... -run TestDeployMission -v
```

These integration tests are deliberately the primary way to verify the Postgres query logic and
the deploy logic — **not** clicking through the built app. Confirmed the hard way during
development, twice: once when `GetDashboardData()` (the Logs tab's predecessor) worked fine called
directly from Go but the actual frontend-triggered binding call path didn't visibly fail the same
way (there's IPC in between) -- every panel's data fetch now wraps its Go call in a try/catch and
renders a visible error banner on failure rather than hanging on a silent "Loading..." forever. And
again when the missing mission-deploy step above went unnoticed until a real launch was tried --
`deploy_test.go` exists so that specific class of bug (code that compiles and "looks right" but
never actually runs against the real filesystem it needs to touch) gets caught by a test run
instead of by someone hitting "Launch" and getting a mission-not-found message with no clear cause.

## Known gaps

- One dedicated server at a time — this manages a single local install, not a fleet.
- `serverCommandPassword` in the rendered `server.cfg` mirrors the admin password field rather
  than being its own UI field, to avoid two passwords the user has to keep in sync for one
  practical purpose.
- Performance-tab samples aren't persisted anywhere (no DB table backs them, on purpose — live
  process telemetry, not durable game state) — history resets on every stop/relaunch and isn't
  visible after closing the app.
- Performance-tab network I/O is **system-wide** (`gopsutil`'s `net.IOCounters`, all interfaces
  summed), not isolated to `arma3server_x64.exe` specifically — Windows has no reliable
  per-process network byte counter the way it does for CPU/memory short of ETW, which is a much
  heavier integration than this warranted. Labeled as such in the UI; a reasonable proxy on a host
  dedicated to running this server, not literally "this process's" traffic.
- Performance-tab Server FPS depends on the mission itself logging `diag_fps` (see
  `src/ALife.Altis/initServer.sqf`) via `diag_log`, read back out of the RPT log `tailLog` already
  tails — Arma has no external query for a running server's frame rate, so a server running a
  *different* mission (one that doesn't log this) would show "waiting..." forever, not an error.
- `findRepoRoot()` assumes it's running from inside a repo checkout (walks up from its own `.exe`
  looking for `database/schema.sql`) — this tool isn't packaged/distributed standalone yet, so that
  assumption hasn't needed revisiting.
