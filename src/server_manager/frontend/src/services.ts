// Services tab -- the Arma 3 server, website, Discord bot and TeamSpeak 3
// server (services.go). Cards are built once and patched in place every
// couple of seconds (and on "services:changed"), so nothing you're typing
// in the setup forms gets wiped by a refresh.

import {
    GetServices, GetServiceHealth, StartService, StopService, RestartService,
    StartAllServices, StopAllServices, SetServiceOptions, GetServiceLog, OpenServiceLogs,
    GetServiceSettings, SaveServiceSettings, CreateTS3AdminKey, BrowseForFolder,
} from '../wailsjs/go/main/App';
import { main } from '../wailsjs/go/models';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { escapeHtml, formatUptime, setMsg } from './util';

const SERVICE_ICONS: Record<string, string> = {
    arma: '<svg viewBox="0 0 24 24" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="4"/><line x1="12" y1="1" x2="12" y2="5"/><line x1="12" y1="19" x2="12" y2="23"/><line x1="1" y1="12" x2="5" y2="12"/><line x1="19" y1="12" x2="23" y2="12"/></svg>',
    website: '<svg viewBox="0 0 24 24" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>',
    bot: '<svg viewBox="0 0 24 24" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="8" width="18" height="12" rx="3"/><line x1="12" y1="4" x2="12" y2="8"/><circle cx="12" cy="3" r="1"/><circle cx="8.5" cy="14" r="1.3"/><circle cx="15.5" cy="14" r="1.3"/></svg>',
    teamspeak: '<svg viewBox="0 0 24 24" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 18v-6a9 9 0 0 1 18 0v6"/><path d="M21 19a2 2 0 0 1-2 2h-1a2 2 0 0 1-2-2v-3a2 2 0 0 1 2-2h3z"/><path d="M3 19a2 2 0 0 0 2 2h1a2 2 0 0 0 2-2v-3a2 2 0 0 0-2-2H3z"/></svg>',
};

const STATE_LABELS: Record<string, string> = {
    'stopped': 'Stopped', 'starting': 'Starting...', 'running': 'Running', 'stopping': 'Stopping...',
    'crashed': 'Crashed · restarting', 'crash-loop': 'Crash loop · stopped',
};

const serviceHealth: Record<string, string> = {};
let built = false;
let logService = 'website';

const input = (id: string) => document.getElementById(id) as HTMLInputElement;

export async function renderServicesPanel() {
    const panel = document.getElementById('panel-services')!;
    let list: main.ServiceStatus[], ss: main.ServiceSettings;
    try {
        list = await GetServices();
        ss = await GetServiceSettings();
    } catch (e) {
        console.error(e);
        panel.innerHTML = `<div class="error-banner">Failed to load: ${escapeHtml(String(e))}</div>`;
        return;
    }

    panel.innerHTML = `
      <div class="page-header page-header-row">
        <div>
          <h1>Services</h1>
          <p>Everything this machine runs for TasDyn ALife, in one place.</p>
        </div>
        <div class="button-row">
          <button class="action secondary" id="svc-stop-all">Stop all</button>
          <button class="action" id="svc-start-all">Start all</button>
        </div>
      </div>
      <div id="svc-msg" class="hint svc-msg"></div>

      <div class="service-grid">
        ${list.map((s) => `
          <div class="card service-card" id="svc-card-${s.key}">
            <div class="service-head">
              <div class="service-icon">${SERVICE_ICONS[s.key] ?? ''}</div>
              <div class="service-title">
                <h2>${escapeHtml(s.title)}</h2>
                <div class="service-meta" id="svc-meta-${s.key}"></div>
              </div>
              <span class="state-pill" id="svc-state-${s.key}"></span>
            </div>
            <div class="service-health" id="svc-health-${s.key}"></div>
            <div class="service-error" id="svc-error-${s.key}"></div>
            <div class="button-row">
              <button class="action" data-svc-act="start" data-svc="${s.key}">Start</button>
              <button class="action secondary" data-svc-act="restart" data-svc="${s.key}">Restart</button>
              <button class="action secondary" data-svc-act="stop" data-svc="${s.key}">Stop</button>
              <button class="action secondary" data-svc-act="logs" data-svc="${s.key}">Log</button>
            </div>
            <div class="service-toggles">
              <label><input type="checkbox" data-svc-opt="auto" data-svc="${s.key}" ${s.autoStart ? 'checked' : ''}/> Start when the manager opens</label>
              <label><input type="checkbox" data-svc-opt="crash" data-svc="${s.key}" ${s.restartOnCrash ? 'checked' : ''}/> Restart if it crashes</label>
            </div>
          </div>`).join('')}
      </div>

      <div class="card">
        <h2>Log</h2>
        <div class="log-toolbar">
          <select id="svc-log-select">
            ${list.map((s) => `<option value="${s.key}">${escapeHtml(s.title)}</option>`).join('')}
          </select>
          <label class="inline-check"><input type="checkbox" id="svc-log-follow" checked/> Follow</label>
          <button class="action secondary" id="svc-log-folder">Open log folder</button>
        </div>
        <div class="console service-log" id="svc-log"></div>
      </div>

      <div class="card">
        <h2>TeamSpeak 3 setup</h2>
        <p class="hint">Download the <strong>TeamSpeak 3 Server</strong> for Windows (64-bit) from teamspeak.com, unzip it somewhere permanent and pick that folder here.
          The manager starts it, sets its ServerQuery admin password and shows the admin key a new server prints on its first start.
          Players connect on the voice port (UDP), so it needs the same port forwarding as the game server.</p>
        <div class="field">
          <label>TeamSpeak 3 server folder (contains ts3server.exe)</label>
          <div class="input-with-button">
            <input type="text" id="ts3-path" value="${escapeHtml(ss.ts3Path)}" placeholder="e.g. E:\\TasDyn\\teamspeak3-server_win64"/>
            <button class="action secondary" id="ts3-browse">Browse...</button>
          </div>
        </div>
        <div class="field-grid">
          <div class="field"><label>Voice port (UDP)</label><input type="number" id="ts3-voice" value="${ss.ts3VoicePort}"/></div>
          <div class="field"><label>ServerQuery port (this PC only)</label><input type="number" id="ts3-query" value="${ss.ts3QueryPort}"/></div>
        </div>
        <div class="field row">
          <input type="checkbox" id="ts3-licence" ${ss.ts3LicenseAccepted ? 'checked' : ''}/>
          <label for="ts3-licence">I've read and accept the TeamSpeak 3 Server licence (license.txt in the server folder)</label>
        </div>
        <div class="field">
          <label>Admin privilege key (copy it from here into the TeamSpeak client)</label>
          <div class="input-with-button">
            <input type="text" id="ts3-key" readonly value="${escapeHtml(ss.ts3PrivilegeKey)}" placeholder="None yet: click New key (TeamSpeak must be running)"/>
            <button class="action secondary" id="ts3-copy">Copy</button>
            <button class="action secondary" id="ts3-newkey">New key</button>
          </div>
          <p class="hint">This box only shows a key; you don't type here. Copy it, then in the TeamSpeak client go to
            <strong>Permissions → Use Privilege Key</strong> and paste it there. Each key works once, so click New key for another.</p>
        </div>
        <div class="button-row"><button class="action" id="ts3-save">Save TeamSpeak setup</button></div>
        <div id="ts3-msg" class="hint"></div>
      </div>

      <div class="card">
        <h2>Website and Discord bot</h2>
        <p class="hint">Both run from the website source and are rebuilt on every start, so a source change goes live on the next restart.
          The website runs as <code>-mode web</code> and the bot as <code>-mode bot</code>, each in its own process. Their settings
          come from <code>.env</code> in the website folder; the bot needs <code>DISCORD_BOT_TOKEN</code> there.</p>
        <div class="field">
          <label>Website folder (blank = ${escapeHtml(ss.websiteDirDefault || 'src/website in this repo')})</label>
          <div class="input-with-button">
            <input type="text" id="web-dir" value="${escapeHtml(ss.websiteDir)}"/>
            <button class="action secondary" id="web-browse">Browse...</button>
          </div>
        </div>
        <div class="button-row"><button class="action secondary" id="web-save">Save</button></div>
        <div id="web-msg" class="hint"></div>
      </div>
    `;
    built = true;

    panel.querySelectorAll<HTMLButtonElement>('[data-svc-act]').forEach((btn) => {
        btn.addEventListener('click', () => serviceAction(btn.dataset.svc!, btn.dataset.svcAct!));
    });
    panel.querySelectorAll<HTMLInputElement>('[data-svc-opt]').forEach((box) => {
        box.addEventListener('change', async () => {
            const key = box.dataset.svc!;
            const auto = (panel.querySelector(`[data-svc-opt="auto"][data-svc="${key}"]`) as HTMLInputElement).checked;
            const crash = (panel.querySelector(`[data-svc-opt="crash"][data-svc="${key}"]`) as HTMLInputElement).checked;
            try { await SetServiceOptions(key, auto, crash); } catch (e) { setMsg('svc-msg', `Couldn't save: ${e}`); }
        });
    });
    document.getElementById('svc-start-all')!.addEventListener('click', async () => {
        setMsg('svc-msg', 'Starting everything...');
        const errs = await StartAllServices();
        setMsg('svc-msg', errs && errs.length ? errs.join(' · ') : '');
        refreshServices();
    });
    document.getElementById('svc-stop-all')!.addEventListener('click', async () => {
        setMsg('svc-msg', 'Stopping everything...');
        await StopAllServices();
        setMsg('svc-msg', '');
        refreshServices();
    });

    const logSelect = document.getElementById('svc-log-select') as HTMLSelectElement;
    logSelect.value = logService;
    logSelect.addEventListener('change', () => { logService = logSelect.value; refreshServiceLog(true); });
    document.getElementById('svc-log-folder')!.addEventListener('click', () => OpenServiceLogs().catch((e) => setMsg('svc-msg', String(e))));

    document.getElementById('ts3-browse')!.addEventListener('click', async () => {
        const p = await BrowseForFolder('Select the TeamSpeak 3 server folder (with ts3server.exe)');
        if (p) input('ts3-path').value = p;
    });
    document.getElementById('web-browse')!.addEventListener('click', async () => {
        const p = await BrowseForFolder('Select the website folder (src/website)');
        if (p) input('web-dir').value = p;
    });
    const save = async (msgId: string) => {
        try {
            const cur = await GetServiceSettings();
            await SaveServiceSettings(main.ServiceSettings.createFrom({
                ...cur,
                ts3Path: input('ts3-path').value.trim(),
                ts3VoicePort: parseInt(input('ts3-voice').value, 10) || 0,
                ts3QueryPort: parseInt(input('ts3-query').value, 10) || 0,
                ts3LicenseAccepted: input('ts3-licence').checked,
                websiteDir: input('web-dir').value.trim(),
            }));
            setMsg(msgId, 'Saved.');
        } catch (e) {
            setMsg(msgId, `Save failed: ${e}`);
        }
    };
    document.getElementById('ts3-save')!.addEventListener('click', () => save('ts3-msg'));
    document.getElementById('web-save')!.addEventListener('click', () => save('web-msg'));
    document.getElementById('ts3-copy')!.addEventListener('click', async () => {
        const v = input('ts3-key').value;
        if (!v) return;
        await navigator.clipboard.writeText(v).catch(() => {});
        setMsg('ts3-msg', 'Key copied.');
    });
    document.getElementById('ts3-newkey')!.addEventListener('click', async () => {
        try {
            input('ts3-key').value = await CreateTS3AdminKey();
            setMsg('ts3-msg', 'New admin key made.');
        } catch (e) {
            setMsg('ts3-msg', `Couldn't make a key: ${e}`);
        }
    });

    applyServices(list);
    refreshServiceHealth();
    refreshServiceLog(true);
}

async function serviceAction(key: string, act: string) {
    if (act === 'logs') {
        logService = key;
        const sel = document.getElementById('svc-log-select') as HTMLSelectElement | null;
        if (sel) sel.value = key;
        refreshServiceLog(true);
        document.getElementById('svc-log')?.scrollIntoView({ behavior: 'smooth', block: 'center' });
        return;
    }
    document.getElementById(`svc-card-${key}`)?.querySelectorAll('button').forEach((b) => (b.disabled = true));
    if (act === 'start' || act === 'restart') {
        const pill = document.getElementById(`svc-state-${key}`);
        if (pill) { pill.className = 'state-pill state-starting'; pill.innerText = key === 'website' || key === 'bot' ? 'Building...' : 'Starting...'; }
    }
    try {
        let warnings: string[] | null = null;
        if (act === 'start') warnings = await StartService(key);
        if (act === 'stop') await StopService(key);
        if (act === 'restart') warnings = await RestartService(key);
        setMsg('svc-msg', warnings && warnings.length ? warnings.join(' · ') : '');
    } catch (e) {
        setMsg('svc-msg', String(e));
    }
    refreshServices();
}

// Sidebar: "3 / 4 services running".
export function applyServicesSummary(list: main.ServiceStatus[]) {
    const dot = document.getElementById('sidebar-status-dot');
    const text = document.getElementById('sidebar-status-text');
    const up = list.filter((s) => s.state === 'running').length;
    const trouble = list.some((s) => s.state === 'crash-loop' || s.state === 'crashed');
    if (dot) {
        dot.classList.toggle('running', up > 0 && !trouble);
        dot.classList.toggle('warn', trouble);
    }
    if (text) text.innerText = trouble ? `${up} / ${list.length} running · needs attention` : `${up} / ${list.length} services running`;
}

function applyServices(list: main.ServiceStatus[]) {
    applyServicesSummary(list);
    if (!built) return;
    for (const s of list) {
        const pill = document.getElementById(`svc-state-${s.key}`);
        const card = document.getElementById(`svc-card-${s.key}`);
        if (!pill || !card) continue;
        pill.className = `state-pill state-${s.state}`;
        pill.innerText = STATE_LABELS[s.state] ?? s.state;
        const meta: string[] = [];
        if (s.pid) meta.push(`PID ${s.pid}`);
        if (s.state === 'running' && s.uptimeSeconds) meta.push(`up ${formatUptime(s.uptimeSeconds)}`);
        if (s.recentCrashes) meta.push(`${s.recentCrashes} crash${s.recentCrashes === 1 ? '' : 'es'} in 5 min`);
        if (s.restartAt) meta.push(`restarting at ${new Date(s.restartAt).toLocaleTimeString()}`);
        document.getElementById(`svc-meta-${s.key}`)!.innerText = meta.join(' · ') || ' ';
        document.getElementById(`svc-health-${s.key}`)!.innerText = s.state === 'running' ? (serviceHealth[s.key] || 'Checking...') : '';
        document.getElementById(`svc-error-${s.key}`)!.innerText = [s.lastError, ...(s.warnings || [])].filter(Boolean).join(' · ');
        const running = s.state === 'running', busy = s.state === 'starting' || s.state === 'stopping';
        (card.querySelector('[data-svc-act="start"]') as HTMLButtonElement).disabled = running || busy;
        (card.querySelector('[data-svc-act="stop"]') as HTMLButtonElement).disabled = !(running || s.state === 'crashed');
        (card.querySelector('[data-svc-act="restart"]') as HTMLButtonElement).disabled = busy;
        (card.querySelector('[data-svc-act="logs"]') as HTMLButtonElement).disabled = false;
    }
}

export async function refreshServices() {
    try {
        applyServices(await GetServices());
    } catch (e) {
        console.error(e);
    }
}

async function refreshServiceHealth() {
    let list: main.ServiceStatus[];
    try { list = await GetServices(); } catch { return; }
    await Promise.all(list.filter((s) => s.state === 'running').map(async (s) => {
        try { serviceHealth[s.key] = await GetServiceHealth(s.key); } catch { /* stays "Checking..." */ }
    }));
    applyServices(list);
}

async function refreshServiceLog(force = false) {
    const el = document.getElementById('svc-log');
    if (!el) return;
    const follow = (document.getElementById('svc-log-follow') as HTMLInputElement | null)?.checked ?? true;
    if (!force && !follow) return;
    try {
        const lines = await GetServiceLog(logService, 400);
        el.innerText = lines && lines.length ? lines.join('\n') : '';
        el.scrollTop = el.scrollHeight;
    } catch (e) {
        el.innerText = `Couldn't read the log: ${e}`;
    }
}

EventsOn('services:changed', () => refreshServices());
setInterval(refreshServices, 2000);
// Health checks hit the services themselves (an HTTP request to the site,
// a ServerQuery login), so only while you're looking at this tab.
const servicesTabOpen = () => document.getElementById('panel-services')?.classList.contains('active') ?? false;
setInterval(() => { if (servicesTabOpen()) refreshServiceHealth(); }, 10000);
setInterval(() => { if (servicesTabOpen()) refreshServiceLog(); }, 2000);
