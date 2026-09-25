import './style.css';
import Chart from 'chart.js/auto';

import {
    GetSettings, SaveSettings, BrowseForArmaServerPath, TestDatabaseConnection,
    GetServerConfig, SaveServerConfig,
    LaunchServer, StopServer, GetServerStatus,
    GetDashboardData,
} from '../wailsjs/go/main/App';
import { main } from '../wailsjs/go/models';
import { EventsOn } from '../wailsjs/runtime/runtime';

// -----------------------------------------------------------------------
// Layout
//
// Every panel's initial data fetch is wrapped in try/catch below, each
// rendering its own error banner on failure -- a rejected promise used to
// fail silently (fire-and-forget async calls with no .catch()), leaving a
// panel stuck on its initial "Loading..." placeholder forever with nothing
// visible going wrong. GetDashboardData() worked fine when called directly
// from Go (proving the SQL/backend logic), but the real binding-call path
// triggered by a click had nowhere to report a failure if one happened --
// caught this while testing, not something to reintroduce.
// -----------------------------------------------------------------------

document.querySelector('#app')!.innerHTML = `
  <header>
    <div class="brand-mark">TD</div>
    <h1>ALife Server Manager</h1>
    <span class="subtitle">TASMAN DYNAMICS</span>
  </header>
  <nav>
    <button data-tab="launch" class="active">Launch</button>
    <button data-tab="console">Console</button>
    <button data-tab="dashboard">Dashboard</button>
    <button data-tab="settings">Settings</button>
  </nav>
  <main>
    <section id="panel-launch" class="panel active"></section>
    <section id="panel-console" class="panel"></section>
    <section id="panel-dashboard" class="panel"></section>
    <section id="panel-settings" class="panel"></section>
  </main>
`;

document.querySelectorAll('nav button').forEach((btn) => {
    btn.addEventListener('click', () => {
        const tab = (btn as HTMLElement).dataset.tab!;
        document.querySelectorAll('nav button').forEach((b) => b.classList.remove('active'));
        btn.classList.add('active');
        document.querySelectorAll('.panel').forEach((p) => p.classList.remove('active'));
        document.getElementById(`panel-${tab}`)!.classList.add('active');
        if (tab === 'dashboard') refreshDashboard();
    });
});

// -----------------------------------------------------------------------
// Launch tab
// -----------------------------------------------------------------------

async function renderLaunchPanel() {
    const panel = document.getElementById('panel-launch')!;

    let cfg: main.ServerConfig, status: main.ServerStatus;
    try {
        cfg = await GetServerConfig();
        status = await GetServerStatus();
    } catch (e) {
        console.error(e);
        panel.innerHTML = `<div class="error-banner">Failed to load: ${escapeHtml(String(e))}</div>`;
        return;
    }

    panel.innerHTML = `
      <div class="card">
        <h2>Server Status</h2>
        <div class="status-badge">
          <span class="status-dot ${status.running ? 'running' : ''}"></span>
          <span id="status-text">${status.running ? `Running (PID ${status.pid})` : 'Stopped'}</span>
        </div>
        <div class="button-row">
          <button class="action" id="btn-launch" ${status.running ? 'disabled' : ''}>Launch Server</button>
          <button class="action secondary" id="btn-stop" ${status.running ? '' : 'disabled'}>Stop Server</button>
        </div>
        <p class="hint">BattlEye/signature verification follow the toggles below -- for a real launch, keep both on (see docs/ANTI_CHEAT.md Layer 0).</p>
      </div>

      <div class="card">
        <h2>Server Configuration</h2>
        <div class="field-grid">
          <div class="field">
            <label>Server Name</label>
            <input type="text" id="cfg-hostname" value="${escapeHtml(cfg.hostname)}" />
          </div>
          <div class="field">
            <label>Mission Template</label>
            <input type="text" id="cfg-mission" value="${escapeHtml(cfg.missionTemplate)}" />
          </div>
          <div class="field">
            <label>Join Password (blank = none)</label>
            <input type="password" id="cfg-password" value="${escapeHtml(cfg.password)}" />
          </div>
          <div class="field">
            <label>Admin Password</label>
            <input type="password" id="cfg-admin" value="${escapeHtml(cfg.passwordAdmin)}" />
          </div>
          <div class="field">
            <label>Player Slots</label>
            <input type="number" id="cfg-slots" min="1" max="128" value="${cfg.maxPlayers}" />
          </div>
        </div>
        <div class="field row">
          <input type="checkbox" id="cfg-battleye" ${cfg.battlEye ? 'checked' : ''} />
          <label for="cfg-battleye">BattlEye enabled</label>
        </div>
        <div class="field row">
          <input type="checkbox" id="cfg-sigs" ${cfg.verifySignatures ? 'checked' : ''} />
          <label for="cfg-sigs">Verify signatures</label>
        </div>
        <div class="button-row">
          <button class="action secondary" id="btn-save-cfg">Save Configuration</button>
        </div>
        <div id="cfg-msg" class="hint"></div>
      </div>
    `;

    document.getElementById('btn-save-cfg')!.addEventListener('click', async () => {
        try {
            await SaveServerConfig(readServerConfigForm());
            setMsg('cfg-msg', 'Saved.');
        } catch (e) {
            setMsg('cfg-msg', `Save failed: ${e}`);
        }
    });

    document.getElementById('btn-launch')!.addEventListener('click', async () => {
        const btn = document.getElementById('btn-launch') as HTMLButtonElement;
        btn.disabled = true;
        try {
            await SaveServerConfig(readServerConfigForm());
            await LaunchServer();
            clearConsole();
            await renderLaunchPanel();
        } catch (e) {
            setMsg('cfg-msg', `Launch failed: ${e}`);
            btn.disabled = false;
        }
    });

    document.getElementById('btn-stop')!.addEventListener('click', async () => {
        try {
            await StopServer();
        } catch (e) {
            setMsg('cfg-msg', `Stop failed: ${e}`);
        }
        await renderLaunchPanel();
    });
}

function readServerConfigForm(): main.ServerConfig {
    return main.ServerConfig.createFrom({
        hostname: (document.getElementById('cfg-hostname') as HTMLInputElement).value,
        missionTemplate: (document.getElementById('cfg-mission') as HTMLInputElement).value,
        password: (document.getElementById('cfg-password') as HTMLInputElement).value,
        passwordAdmin: (document.getElementById('cfg-admin') as HTMLInputElement).value,
        maxPlayers: parseInt((document.getElementById('cfg-slots') as HTMLInputElement).value, 10) || 1,
        battlEye: (document.getElementById('cfg-battleye') as HTMLInputElement).checked,
        verifySignatures: (document.getElementById('cfg-sigs') as HTMLInputElement).checked,
    });
}

// -----------------------------------------------------------------------
// Console tab -- live RPT tail via the "server:log" event LaunchServer's
// background goroutine emits. Nothing shows until a server is launched;
// this isn't a historical log viewer, just the live stream.
// -----------------------------------------------------------------------

function renderConsolePanel() {
    const panel = document.getElementById('panel-console')!;
    panel.innerHTML = `
      <div class="card">
        <h2>Live Server Console</h2>
        <div class="console" id="console-output"></div>
      </div>
    `;
}

function clearConsole() {
    const el = document.getElementById('console-output');
    if (el) el.innerText = '';
}

EventsOn('server:log', (line: string) => {
    const el = document.getElementById('console-output');
    if (!el) return;
    el.innerText += line;
    el.scrollTop = el.scrollHeight;
});

EventsOn('server:stopped', () => {
    if (document.getElementById('panel-launch')?.classList.contains('active')) {
        renderLaunchPanel();
    }
});

// -----------------------------------------------------------------------
// Dashboard tab -- graphs over docs/DATA_CONTRACT.md's tables. Genuinely
// empty until the server has real players; an empty graph is shown
// honestly, not faked.
// -----------------------------------------------------------------------

let playerChart: Chart | null = null;

function renderDashboardPanel() {
    const panel = document.getElementById('panel-dashboard')!;
    panel.innerHTML = `
      <div class="button-row" style="margin-bottom:16px;">
        <button class="action secondary" id="btn-refresh-dash">Refresh</button>
      </div>
      <div id="dash-content"><div class="empty-state">Loading...</div></div>
    `;
    document.getElementById('btn-refresh-dash')!.addEventListener('click', refreshDashboard);
}

async function refreshDashboard() {
    const content = document.getElementById('dash-content');
    if (!content) return;

    content.innerHTML = '<div class="empty-state">Loading...</div>';

    let data: main.DashboardData;
    try {
        data = await GetDashboardData();
    } catch (e) {
        console.error(e);
        content.innerHTML = `<div class="error-banner">Failed to reach the app backend: ${escapeHtml(String(e))}</div>`;
        return;
    }

    if (data.error) {
        content.innerHTML = `<div class="error-banner">${escapeHtml(data.error)}</div>`;
        return;
    }

    content.innerHTML = `
      <div class="stat-grid">
        <div class="stat-tile">
          <div class="label">Players</div>
          <div class="value">${data.economy.playerCount}</div>
        </div>
        <div class="stat-tile">
          <div class="label">Total Cash In Circulation</div>
          <div class="value">$${data.economy.totalCash.toLocaleString()}</div>
        </div>
        <div class="stat-tile">
          <div class="label">Total Bank Balance</div>
          <div class="value">$${data.economy.totalBank.toLocaleString()}</div>
        </div>
      </div>

      <div class="card">
        <h2>Player Connections (last 7 days)</h2>
        ${data.playerCounts.length === 0
            ? '<div class="empty-state">No sessions recorded yet.</div>'
            : '<canvas id="player-chart" height="80"></canvas>'}
      </div>

      <div class="card">
        <h2>Anti-Cheat Flags</h2>
        ${renderAntiCheatTable(data.antiCheatFlags)}
      </div>

      <div class="card">
        <h2>Recent Staff Actions</h2>
        ${renderStaffLogTable(data.recentStaffLog)}
      </div>
    `;

    if (data.playerCounts.length > 0) {
        const ctx = (document.getElementById('player-chart') as HTMLCanvasElement).getContext('2d')!;
        if (playerChart) playerChart.destroy();
        playerChart = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: data.playerCounts.map((p) => p.bucket),
                datasets: [{
                    label: 'Sessions',
                    data: data.playerCounts.map((p) => p.count),
                    backgroundColor: '#f59e0b',
                }],
            },
            options: {
                scales: {
                    y: { beginAtZero: true, ticks: { color: '#94a3b8' }, grid: { color: '#334155' } },
                    x: { ticks: { color: '#94a3b8' }, grid: { display: false } },
                },
                plugins: { legend: { display: false } },
            },
        });
    }
}

function renderAntiCheatTable(rows: main.AntiCheatFlagCount[]): string {
    if (rows.length === 0) return '<div class="empty-state">No flags recorded yet.</div>';
    return `
      <table>
        <thead><tr><th>Type</th><th>Total</th><th>Unreviewed</th></tr></thead>
        <tbody>
          ${rows.map((r) => `<tr><td>${escapeHtml(r.flagType)}</td><td>${r.total}</td><td>${r.unreviewed}</td></tr>`).join('')}
        </tbody>
      </table>
    `;
}

function renderStaffLogTable(rows: main.StaffLogEntry[]): string {
    if (rows.length === 0) return '<div class="empty-state">No staff actions recorded yet.</div>';
    return `
      <table>
        <thead><tr><th>Action</th><th>Reason</th><th>When</th></tr></thead>
        <tbody>
          ${rows.map((r) => `<tr><td>${escapeHtml(r.action)}</td><td>${escapeHtml(r.reason)}</td><td>${escapeHtml(r.createdAt)}</td></tr>`).join('')}
        </tbody>
      </table>
    `;
}

// -----------------------------------------------------------------------
// Settings tab
// -----------------------------------------------------------------------

async function renderSettingsPanel() {
    const panel = document.getElementById('panel-settings')!;

    let s: main.Settings;
    try {
        s = await GetSettings();
    } catch (e) {
        console.error(e);
        panel.innerHTML = `<div class="error-banner">Failed to load: ${escapeHtml(String(e))}</div>`;
        return;
    }

    panel.innerHTML = `
      <div class="card">
        <h2>Arma 3 Server</h2>
        <div class="field">
          <label>Install Path</label>
          <input type="text" id="set-arma-path" value="${escapeHtml(s.armaServerPath)}" />
        </div>
        <div class="button-row">
          <button class="action secondary" id="btn-browse">Browse...</button>
        </div>
      </div>

      <div class="card">
        <h2>Postgres Connection</h2>
        <div class="field-grid">
          <div class="field">
            <label>Host</label>
            <input type="text" id="set-pg-host" value="${escapeHtml(s.postgresHost)}" />
          </div>
          <div class="field">
            <label>Port</label>
            <input type="text" id="set-pg-port" value="${escapeHtml(s.postgresPort)}" />
          </div>
          <div class="field">
            <label>Database</label>
            <input type="text" id="set-pg-db" value="${escapeHtml(s.postgresDb)}" />
          </div>
          <div class="field">
            <label>User</label>
            <input type="text" id="set-pg-user" value="${escapeHtml(s.postgresUser)}" />
          </div>
          <div class="field">
            <label>Password</label>
            <input type="password" id="set-pg-password" value="${escapeHtml(s.postgresPassword)}" />
          </div>
        </div>
        <div class="button-row">
          <button class="action secondary" id="btn-test-db">Test Connection</button>
        </div>
        <div id="db-test-msg" class="hint"></div>
      </div>

      <div class="button-row">
        <button class="action" id="btn-save-settings">Save Settings</button>
      </div>
      <div id="settings-msg" class="hint"></div>
    `;

    document.getElementById('btn-browse')!.addEventListener('click', async () => {
        try {
            const path = await BrowseForArmaServerPath();
            if (path) (document.getElementById('set-arma-path') as HTMLInputElement).value = path;
        } catch (e) {
            setMsg('settings-msg', `Browse failed: ${e}`);
        }
    });

    document.getElementById('btn-save-settings')!.addEventListener('click', async () => {
        try {
            await SaveSettings(readSettingsForm());
            setMsg('settings-msg', 'Saved.');
        } catch (e) {
            setMsg('settings-msg', `Save failed: ${e}`);
        }
    });

    document.getElementById('btn-test-db')!.addEventListener('click', async () => {
        await SaveSettings(readSettingsForm());
        setMsg('db-test-msg', 'Testing...');
        try {
            await TestDatabaseConnection();
            setMsg('db-test-msg', 'Connected successfully.');
        } catch (e) {
            setMsg('db-test-msg', `Failed: ${e}`);
        }
    });
}

function readSettingsForm(): main.Settings {
    return main.Settings.createFrom({
        armaServerPath: (document.getElementById('set-arma-path') as HTMLInputElement).value,
        postgresHost: (document.getElementById('set-pg-host') as HTMLInputElement).value,
        postgresPort: (document.getElementById('set-pg-port') as HTMLInputElement).value,
        postgresDb: (document.getElementById('set-pg-db') as HTMLInputElement).value,
        postgresUser: (document.getElementById('set-pg-user') as HTMLInputElement).value,
        postgresPassword: (document.getElementById('set-pg-password') as HTMLInputElement).value,
    });
}

// -----------------------------------------------------------------------
// Small helpers
// -----------------------------------------------------------------------

function setMsg(id: string, text: string) {
    const el = document.getElementById(id);
    if (el) el.innerText = text;
}

function escapeHtml(s: string | undefined): string {
    if (!s) return '';
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// -----------------------------------------------------------------------
// Boot
// -----------------------------------------------------------------------

renderLaunchPanel();
renderConsolePanel();
renderDashboardPanel();
renderSettingsPanel();
