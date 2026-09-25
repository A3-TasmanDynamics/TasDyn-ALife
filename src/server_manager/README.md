# src/server_manager/

A desktop app for actually running the dedicated server day to day — configure the server name,
join/admin passwords, and slot count; launch and stop `arma3server_x64.exe`; watch its live log;
and see graphs pulled straight from Postgres (players, economy, anti-cheat flags, staff actions).

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

## What's here

```text
src/server_manager/
├── app.go               # App struct -- every exported method is callable from the frontend
├── settings.go           # Arma 3 Server path + Postgres connection, persisted to %AppData%
├── serverconfig.go        # server.cfg fields (name/passwords/slots) + the actual .cfg renderer
├── serverprocess.go        # Launch/stop arma3server_x64.exe, live RPT log tail as events
├── dashboard.go             # The Postgres queries behind the Dashboard tab
├── dashboard_test.go         # Integration test against a real local DB -- see below
└── frontend/
    └── src/
        ├── main.ts            # All four tabs: Launch, Console, Dashboard, Settings
        └── style.css           # Dark navy/amber theme, matching the org's brand palette
```

Settings and the server config are stored outside this repo, at
`%AppData%\TasDyn-ALife-ServerManager\` — this app's own state, not something to commit or ship.

## Testing

`dashboard_test.go` runs the real dashboard queries against a live local Postgres instance
(skipped by default, since it needs one):

```powershell
$env:ALIFE_TEST_DB = "1"; go test ./... -run TestDashboardQueries -v
```

This is deliberately the primary way to verify the Postgres query logic — **not** clicking through
the built app. Confirmed the hard way during development: `GetDashboardData()` worked fine when
called directly from Go, but that doesn't prove the actual frontend-triggered binding call path
does the same thing (there's IPC in between). Every panel's data fetch now wraps its Go call in a
try/catch and renders a visible error banner on failure rather than hanging on a silent
"Loading..." forever — the actual bug this was chasing, whatever the root cause turns out to be.

## Known gaps

- No historical economy graph — `EconomySnapshot` is a current-totals snapshot
  (`SUM(civ_cash + cop_cash + medic_cash)` etc. across all players), not a time series. Would need
  periodic snapshots written somewhere, which nothing does yet.
- One dedicated server at a time — this manages a single local install, not a fleet.
- `serverCommandPassword` in the rendered `server.cfg` mirrors the admin password field rather
  than being its own UI field, to avoid two passwords the user has to keep in sync for one
  practical purpose.
