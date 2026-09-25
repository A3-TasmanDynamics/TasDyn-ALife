import './style.css';
import Chart from 'chart.js/auto';

import {
    GetSettings, SaveSettings, BrowseForArmaServerPath, TestDatabaseConnection,
    GetServerConfig, SaveServerConfig,
    LaunchServer, StopServer, GetServerStatus,
    GetStaffLog, GetAntiCheatLog, GetKickLog,
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
// visible going wrong. Caught this while testing, not something to
// reintroduce.
// -----------------------------------------------------------------------

const ICONS = {
    launch: '<svg viewBox="0 0 24 24" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z"/><path d="M12 15l-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z"/><path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0"/><path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5"/></svg>',
    console: '<svg viewBox="0 0 24 24" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>',
    logs: '<svg viewBox="0 0 24 24" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>',
    performance: '<svg viewBox="0 0 24 24" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>',
    settings: '<svg viewBox="0 0 24 24" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>',
};

document.querySelector('#app')!.innerHTML = `
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">
        <div class="brand-mark">TD</div>
        <div class="brand-text">
          <div class="brand-title">ALife</div>
          <div class="brand-subtitle">SERVER MANAGER</div>
        </div>
      </div>
      <nav>
        <button data-tab="launch" class="nav-item active">${ICONS.launch}<span>Launch</span></button>
        <button data-tab="console" class="nav-item">${ICONS.console}<span>Console</span></button>
        <button data-tab="logs" class="nav-item">${ICONS.logs}<span>Logs</span></button>
        <button data-tab="performance" class="nav-item">${ICONS.performance}<span>Performance</span></button>
        <button data-tab="settings" class="nav-item">${ICONS.settings}<span>Settings</span></button>
      </nav>
      <div class="sidebar-footer">
        <div class="status-badge" id="sidebar-status">
          <span class="status-dot" id="sidebar-status-dot"></span>
          <span id="sidebar-status-text">Checking...</span>
        </div>
        <div class="sidebar-org">TASMAN DYNAMICS</div>
      </div>
    </aside>
    <div class="content-area">
      <main>
        <section id="panel-launch" class="panel active"></section>
        <section id="panel-console" class="panel"></section>
        <section id="panel-logs" class="panel"></section>
        <section id="panel-performance" class="panel"></section>
        <section id="panel-settings" class="panel"></section>
      </main>
    </div>
  </div>
`;

document.querySelectorAll('.nav-item').forEach((btn) => {
    btn.addEventListener('click', () => {
        const tab = (btn as HTMLElement).dataset.tab!;
        document.querySelectorAll('.nav-item').forEach((b) => b.classList.remove('active'));
        btn.classList.add('active');
        document.querySelectorAll('.panel').forEach((p) => p.classList.remove('active'));
        document.getElementById(`panel-${tab}`)!.classList.add('active');
        if (tab === 'logs') loadSelectedLog();
        if (tab === 'performance') renderPerformancePanel();
    });
});

// -----------------------------------------------------------------------
// Sidebar status -- a persistent indicator of whether the dedicated server
// process is running, visible regardless of which tab is open (previously
// this only showed inside the Launch tab's own status card).
// -----------------------------------------------------------------------

function applySidebarStatus(status: main.ServerStatus) {
    const dot = document.getElementById('sidebar-status-dot');
    const text = document.getElementById('sidebar-status-text');
    if (dot) dot.classList.toggle('running', status.running);
    if (text) text.innerText = status.running ? `Running · PID ${status.pid}` : 'Stopped';
}

async function refreshSidebarStatus() {
    try {
        applySidebarStatus(await GetServerStatus());
    } catch (e) {
        console.error(e);
    }
}

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

    applySidebarStatus(status);

    panel.innerHTML = `
      <div class="page-header">
        <h1>Launch</h1>
        <p>Configure and start your Arma 3 dedicated server.</p>
      </div>

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
            const result = await LaunchServer();
            clearConsole();
            resetPerformanceHistory();
            await renderLaunchPanel();
            // renderLaunchPanel() just rebuilt cfg-msg -- set this after,
            // not before, or the re-render wipes it immediately.
            if (result.warnings && result.warnings.length > 0) {
                setMsg('cfg-msg', `Launched with warnings: ${result.warnings.join(' · ')}`);
            }
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
      <div class="page-header">
        <h1>Console</h1>
        <p>Live output from the running dedicated server.</p>
      </div>

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
    refreshSidebarStatus();
    resetPerformanceHistory();
    if (document.getElementById('panel-launch')?.classList.contains('active')) {
        renderLaunchPanel();
    }
    if (document.getElementById('panel-performance')?.classList.contains('active')) {
        renderPerformancePanel();
    }
});

// -----------------------------------------------------------------------
// Logs tab -- pick a log type, view its rows. Each type is its own Go
// call (GetStaffLog/GetAntiCheatLog/GetKickLog) rather than one big
// "everything" fetch, so picking a type is just a fresh, cheap query.
// -----------------------------------------------------------------------

type LogType = 'staff' | 'anticheat' | 'kick';
let currentLogType: LogType = 'staff';

function renderLogsPanel() {
    const panel = document.getElementById('panel-logs')!;
    panel.innerHTML = `
      <div class="page-header">
        <h1>Logs</h1>
        <p>Staff actions, anti-cheat flags, and kicks -- pick a log to view.</p>
      </div>

      <div class="card">
        <div class="field-grid" style="grid-template-columns: 1fr auto;">
          <div class="field">
            <label>Log type</label>
            <select id="log-type-select">
              <option value="staff">Staff Actions</option>
              <option value="anticheat">Anti-Cheat Flags</option>
              <option value="kick">Kicks</option>
            </select>
          </div>
          <div class="button-row" style="margin-top: 26px;">
            <button class="action secondary" id="btn-refresh-log">Refresh</button>
          </div>
        </div>
        <div id="log-table-content"><div class="empty-state">Loading...</div></div>
      </div>
    `;

    const select = document.getElementById('log-type-select') as HTMLSelectElement;
    select.value = currentLogType;
    select.addEventListener('change', () => {
        currentLogType = select.value as LogType;
        loadSelectedLog();
    });
    document.getElementById('btn-refresh-log')!.addEventListener('click', loadSelectedLog);

    loadSelectedLog();
}

async function loadSelectedLog() {
    const content = document.getElementById('log-table-content');
    if (!content) return;
    content.innerHTML = '<div class="empty-state">Loading...</div>';

    try {
        if (currentLogType === 'staff') {
            const rows = await GetStaffLog();
            content.innerHTML = renderStaffLogTable(rows);
        } else if (currentLogType === 'anticheat') {
            const rows = await GetAntiCheatLog();
            content.innerHTML = renderAntiCheatLogTable(rows);
        } else {
            const rows = await GetKickLog();
            content.innerHTML = renderKickLogTable(rows);
        }
    } catch (e) {
        console.error(e);
        content.innerHTML = `<div class="error-banner">Failed to load log: ${escapeHtml(String(e))}</div>`;
    }
}

function renderStaffLogTable(rows: main.StaffLogEntry[]): string {
    if (!rows || rows.length === 0) return '<div class="empty-state">No staff actions logged yet.</div>';
    return `
      <table>
        <thead><tr><th>Staff</th><th>Target</th><th>Action</th><th>Reason</th><th>When</th></tr></thead>
        <tbody>
          ${rows.map((r) => `<tr><td>${escapeHtml(r.staffName)}</td><td>${escapeHtml(r.targetName)}</td><td>${escapeHtml(r.action)}</td><td>${escapeHtml(r.reason)}</td><td>${escapeHtml(r.createdAt)}</td></tr>`).join('')}
        </tbody>
      </table>
    `;
}

function renderAntiCheatLogTable(rows: main.AntiCheatLogEntry[]): string {
    if (!rows || rows.length === 0) return '<div class="empty-state">No flags recorded yet.</div>';
    return `
      <table>
        <thead><tr><th>Player</th><th>Type</th><th>Confidence</th><th>Resolution</th><th>When</th></tr></thead>
        <tbody>
          ${rows.map((r) => `<tr><td>${escapeHtml(r.playerName)}</td><td>${escapeHtml(r.flagType)}</td><td>${escapeHtml(r.confidence)}</td><td>${r.resolution ? escapeHtml(r.resolution) : '<em>unreviewed</em>'}</td><td>${escapeHtml(r.createdAt)}</td></tr>`).join('')}
        </tbody>
      </table>
    `;
}

function renderKickLogTable(rows: main.KickLogEntry[]): string {
    if (!rows || rows.length === 0) return '<div class="empty-state">No kicks logged yet.</div>';
    return `
      <table>
        <thead><tr><th>Target</th><th>Kicked By</th><th>Type</th><th>Reason</th><th>When</th></tr></thead>
        <tbody>
          ${rows.map((r) => `<tr><td>${escapeHtml(r.targetName)}</td><td>${escapeHtml(r.kickedBy)}</td><td>${escapeHtml(r.kickType)}</td><td>${escapeHtml(r.reason)}</td><td>${escapeHtml(r.createdAt)}</td></tr>`).join('')}
        </tbody>
      </table>
    `;
}

// -----------------------------------------------------------------------
// Performance tab -- live CPU/memory graphs for the running dedicated
// server process, pushed via the "server:performance" event (see
// src/server_manager/serverprocess.go's samplePerformance). No history
// persists across a stop/relaunch -- this is live telemetry for the
// current run, not a stored metric (no DB table backs it, on purpose).
// -----------------------------------------------------------------------

const MAX_PERF_POINTS = 60; // ~2 minutes at one sample per 2s

const perfHistory: {
    labels: string[]; cpu: number[]; mem: number[]; netSent: number[]; netRecv: number[]; fps: (number | null)[];
} = { labels: [], cpu: [], mem: [], netSent: [], netRecv: [], fps: [] };
let cpuChart: Chart | null = null;
let memChart: Chart | null = null;
let netChart: Chart | null = null;
let fpsChart: Chart | null = null;

function resetPerformanceHistory() {
    perfHistory.labels = [];
    perfHistory.cpu = [];
    perfHistory.mem = [];
    perfHistory.netSent = [];
    perfHistory.netRecv = [];
    perfHistory.fps = [];
    cpuChart = null;
    memChart = null;
    netChart = null;
    fpsChart = null;
}

async function renderPerformancePanel() {
    const panel = document.getElementById('panel-performance')!;

    let status: main.ServerStatus;
    try {
        status = await GetServerStatus();
    } catch (e) {
        console.error(e);
        panel.innerHTML = `<div class="error-banner">Failed to load: ${escapeHtml(String(e))}</div>`;
        return;
    }

    if (!status.running && perfHistory.labels.length === 0) {
        panel.innerHTML = `
          <div class="page-header">
            <h1>Performance</h1>
            <p>Live CPU and memory usage for the running dedicated server.</p>
          </div>
          <div class="card">
            <div class="empty-state">No server running. Launch it from the Launch tab to see live performance graphs.</div>
          </div>
        `;
        return;
    }

    panel.innerHTML = `
      <div class="page-header">
        <h1>Performance</h1>
        <p>Live CPU, memory, network, and server FPS for the running dedicated server.</p>
      </div>

      <div class="stat-grid">
        <div class="stat-tile">
          <div class="label">CPU</div>
          <div class="value" id="perf-cpu-value">--</div>
        </div>
        <div class="stat-tile">
          <div class="label">Memory</div>
          <div class="value" id="perf-mem-value">--</div>
        </div>
        <div class="stat-tile">
          <div class="label">Server FPS</div>
          <div class="value" id="perf-fps-value">--</div>
        </div>
        <div class="stat-tile">
          <div class="label">Uptime</div>
          <div class="value" id="perf-uptime-value">--</div>
        </div>
      </div>

      <div class="card">
        <h2>CPU Usage</h2>
        <canvas id="perf-cpu-chart" height="70"></canvas>
      </div>

      <div class="card">
        <h2>Memory Usage</h2>
        <canvas id="perf-mem-chart" height="70"></canvas>
      </div>

      <div class="card">
        <h2>Server FPS</h2>
        <p class="hint">From the mission's own <code>diag_fps</code>, logged every 2s -- appears a few seconds after the mission finishes loading.</p>
        <canvas id="perf-fps-chart" height="70"></canvas>
      </div>

      <div class="card">
        <h2>Network I/O</h2>
        <p class="hint">System-wide, not isolated to this process -- Windows has no reliable per-process network byte counter the way it does for CPU/memory. A reasonable proxy on a host dedicated to this server.</p>
        <canvas id="perf-net-chart" height="70"></canvas>
      </div>
    `;

    const lineDefaults = {
        animation: false as const,
        scales: {
            y: { beginAtZero: true, ticks: { color: '#94a3b8' }, grid: { color: '#334155' } },
            x: { ticks: { color: '#94a3b8', maxTicksLimit: 6 }, grid: { display: false } },
        },
    };

    const cpuCtx = (document.getElementById('perf-cpu-chart') as HTMLCanvasElement).getContext('2d')!;
    cpuChart = new Chart(cpuCtx, {
        type: 'line',
        data: {
            labels: [...perfHistory.labels],
            datasets: [{
                label: 'CPU %',
                data: [...perfHistory.cpu],
                borderColor: '#f59e0b',
                backgroundColor: 'rgba(245, 158, 11, 0.12)',
                fill: true,
                tension: 0.25,
                pointRadius: 0,
            }],
        },
        options: { ...lineDefaults, plugins: { legend: { display: false } } },
    });

    const memCtx = (document.getElementById('perf-mem-chart') as HTMLCanvasElement).getContext('2d')!;
    memChart = new Chart(memCtx, {
        type: 'line',
        data: {
            labels: [...perfHistory.labels],
            datasets: [{
                label: 'Memory (MB)',
                data: [...perfHistory.mem],
                borderColor: '#fbbf24',
                backgroundColor: 'rgba(251, 191, 36, 0.12)',
                fill: true,
                tension: 0.25,
                pointRadius: 0,
            }],
        },
        options: { ...lineDefaults, plugins: { legend: { display: false } } },
    });

    const fpsCtx = (document.getElementById('perf-fps-chart') as HTMLCanvasElement).getContext('2d')!;
    fpsChart = new Chart(fpsCtx, {
        type: 'line',
        data: {
            labels: [...perfHistory.labels],
            datasets: [{
                label: 'Server FPS',
                data: [...perfHistory.fps],
                borderColor: '#22c55e',
                backgroundColor: 'rgba(34, 197, 94, 0.12)',
                fill: true,
                tension: 0.25,
                pointRadius: 0,
                spanGaps: true,
            }],
        },
        options: { ...lineDefaults, plugins: { legend: { display: false } } },
    });

    const netCtx = (document.getElementById('perf-net-chart') as HTMLCanvasElement).getContext('2d')!;
    netChart = new Chart(netCtx, {
        type: 'line',
        data: {
            labels: [...perfHistory.labels],
            datasets: [
                {
                    label: 'Sent (KB/s)',
                    data: [...perfHistory.netSent],
                    borderColor: '#f59e0b',
                    backgroundColor: 'transparent',
                    tension: 0.25,
                    pointRadius: 0,
                },
                {
                    label: 'Received (KB/s)',
                    data: [...perfHistory.netRecv],
                    borderColor: '#38bdf8',
                    backgroundColor: 'transparent',
                    tension: 0.25,
                    pointRadius: 0,
                },
            ],
        },
        options: {
            ...lineDefaults,
            plugins: { legend: { display: true, labels: { color: '#94a3b8', boxWidth: 12, font: { size: 11 } } } },
        },
    });

    if (perfHistory.cpu.length > 0) {
        const cpuEl = document.getElementById('perf-cpu-value');
        const memEl = document.getElementById('perf-mem-value');
        const fpsEl = document.getElementById('perf-fps-value');
        const upEl = document.getElementById('perf-uptime-value');
        const lastFps = perfHistory.fps[perfHistory.fps.length - 1];
        if (cpuEl) cpuEl.innerText = `${perfHistory.cpu[perfHistory.cpu.length - 1].toFixed(1)}%`;
        if (memEl) memEl.innerText = `${perfHistory.mem[perfHistory.mem.length - 1].toFixed(0)} MB`;
        if (fpsEl) fpsEl.innerText = lastFps === null ? 'waiting...' : lastFps.toFixed(1);
        if (upEl) upEl.innerText = formatUptime(lastUptimeSeconds);
    }
}

let lastUptimeSeconds = 0;

type PerformanceEvent = {
    timestamp: string;
    cpuPercent: number;
    memoryMB: number;
    uptimeSeconds: number;
    networkSentKBps: number;
    networkRecvKBps: number;
    serverFPS: number; // -1 = no diag_fps line read yet this run
};

EventsOn('server:performance', (sample: PerformanceEvent) => {
    const label = new Date(sample.timestamp).toLocaleTimeString();
    const fpsValue = sample.serverFPS < 0 ? null : sample.serverFPS;

    perfHistory.labels.push(label);
    perfHistory.cpu.push(sample.cpuPercent);
    perfHistory.mem.push(sample.memoryMB);
    perfHistory.netSent.push(sample.networkSentKBps);
    perfHistory.netRecv.push(sample.networkRecvKBps);
    perfHistory.fps.push(fpsValue);
    if (perfHistory.labels.length > MAX_PERF_POINTS) {
        perfHistory.labels.shift();
        perfHistory.cpu.shift();
        perfHistory.mem.shift();
        perfHistory.netSent.shift();
        perfHistory.netRecv.shift();
        perfHistory.fps.shift();
    }
    lastUptimeSeconds = sample.uptimeSeconds;

    const cpuEl = document.getElementById('perf-cpu-value');
    const memEl = document.getElementById('perf-mem-value');
    const fpsEl = document.getElementById('perf-fps-value');
    const upEl = document.getElementById('perf-uptime-value');
    if (cpuEl) cpuEl.innerText = `${sample.cpuPercent.toFixed(1)}%`;
    if (memEl) memEl.innerText = `${sample.memoryMB.toFixed(0)} MB`;
    if (fpsEl) fpsEl.innerText = fpsValue === null ? 'waiting...' : fpsValue.toFixed(1);
    if (upEl) upEl.innerText = formatUptime(sample.uptimeSeconds);

    if (cpuChart) {
        cpuChart.data.labels = [...perfHistory.labels];
        cpuChart.data.datasets[0].data = [...perfHistory.cpu];
        cpuChart.update();
    }
    if (memChart) {
        memChart.data.labels = [...perfHistory.labels];
        memChart.data.datasets[0].data = [...perfHistory.mem];
        memChart.update();
    }
    if (fpsChart) {
        fpsChart.data.labels = [...perfHistory.labels];
        fpsChart.data.datasets[0].data = [...perfHistory.fps];
        fpsChart.update();
    }
    if (netChart) {
        netChart.data.labels = [...perfHistory.labels];
        netChart.data.datasets[0].data = [...perfHistory.netSent];
        netChart.data.datasets[1].data = [...perfHistory.netRecv];
        netChart.update();
    }

    // First sample after the panel was showing its "no server running"
    // empty state (opened before launch, or opened right as it started) --
    // re-render once so the charts actually appear instead of staying on
    // the empty-state message forever.
    if (!cpuChart && document.getElementById('panel-performance')?.classList.contains('active')) {
        renderPerformancePanel();
    }
});

function formatUptime(totalSeconds: number): string {
    const h = Math.floor(totalSeconds / 3600);
    const m = Math.floor((totalSeconds % 3600) / 60);
    const s = totalSeconds % 60;
    if (h > 0) return `${h}h ${m}m`;
    if (m > 0) return `${m}m ${s}s`;
    return `${s}s`;
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
      <div class="page-header">
        <h1>Settings</h1>
        <p>Local paths and database connection.</p>
      </div>

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
renderLogsPanel();
renderPerformancePanel();
renderSettingsPanel();
refreshSidebarStatus();
