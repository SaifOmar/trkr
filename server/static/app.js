const API = '/api/v1';
const POLL_MS = 3000;

const COLORS = [
    '#58a6ff', '#3fb950', '#d2a8ff', '#f0883e',
    '#f778ba', '#79c0ff', '#ffa657', '#ff7b72',
];
const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
const DAYS_FULL = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

const SYSTEM_PROCS_LINUX = new Set([
  'systemd', 'pipewire', 'pipewire-pulse', 'wireplumber',
  'dbus-daemon', 'dbus-broker', 'dbus-broker-launch',
  'xdg-dbus-proxy', 'xdg-desktop-portal', 'xdg-desktop-portal-gtk',
  'xdg-document-portal', 'xdg-permission-store',
  'gvfsd', 'gvfsd-fuse', 'gvfsd-trash', 'gvfsd-metadata',
  'pulseaudio', 'rtkit-daemon', 'at-spi-bus-launcher', 'at-spi2-registryd',
  'gnome-shell', 'gnome-session', 'gnome-session-binary',
  'ssh-agent', 'gdm-session-worker', 'Xorg',
  'upowerd', 'NetworkManager', 'bluetoothd', 'wpa_supplicant',
  'accounts-daemon', 'packagekitd', 'snapd', 'udisksd', 'fwupd',
  'goa-daemon', 'evolution-source-registry',
  'colord', 'geoclue', 'dnsmasq', 'boltd',
]);

const SYSTEM_PROCS_WINDOWS = new Set([
  'svchost', 'System', 'smss', 'csrss', 'wininit', 'services',
  'lsass', 'winlogon', 'conhost', 'RuntimeBroker',
  'SearchIndexer', 'WmiPrvSE', 'SgrmBroker',
]);

// ── DOM ──
const $ = (id) => document.getElementById(id);
const dom = {
    processSearch: $('process-search'),
    processList: $('process-list'),
    processSkeleton: $('process-skeleton'),
    processRetry: $('process-retry'),
    processCount: $('process-count'),
    summaryTotal: $('summary-total-value'),
    statToday: $('stat-today'),
    statTodayLabel: $('stat-today-label'),
    statAvg: $('stat-avg'),
    statAvgLabel: $('stat-avg-label'),
    statActive: $('stat-active'),
    statActiveLabel: $('stat-active-label'),
    activeSessionsList: $('active-sessions-list'),
    activeSessionsSkeleton: $('active-sessions-skeleton'),
    activeSessionsCount: $('active-sessions-count'),
    buildingChart: $('building-chart'),
    donutRing: $('donut-ring'),
    donutValue: $('donut-value'),
    donutLabel: $('donut-label'),
    machineInfo: $('machine-info'),
    watchlistInput: $('watchlist-input'),
    watchlistAddBtn: $('watchlist-add-btn'),
    watchlistSuggestions: $('watchlist-suggestions'),
    watchlistList: $('watchlist-list'),
    watchlistCount: $('watchlist-count'),
    snackbar: $('snackbar'),
    liveIndicator: $('live-indicator'),
    offlineBanner: $('offline-banner'),
    detailPanel: $('detail-panel'),
    detailBackdrop: $('detail-backdrop'),
    detailClose: $('detail-close'),
    detailTitle: $('detail-title'),
    detailBody: $('detail-body'),
    navLinks: $('nav-links'),
    processesFilter: $('processes-filter'),
    processesSearch: $('processes-search'),
    processesSort: $('processes-sort'),
    processesTbody: $('processes-tbody'),
    processesEmpty: $('processes-empty'),
    historyList: $('history-list'),
    historySearch: $('history-search'),
    historyFilter: $('history-filter'),
    historySort: $('history-sort'),
    historyLoadMoreWrap: $('history-load-more-wrap'),
    historyLoadMore: $('history-load-more'),
    processesExpandBtn: $('processes-expand-btn'),
};

// ── Store ──
const store = {
    _data: { activeProcesses: [], activeSessions: [], sessions: [], autoWatch: [] },
    _errors: { activeProcesses: false, activeSessions: false, sessions: false, autoWatch: false },
    _listeners: [],
    get(key) { const v = this._data[key]; return Array.isArray(v) ? v : []; },
    set(key, val) { this._data[key] = Array.isArray(val) ? val : []; this._errors[key] = false; this._notify(key); },
    err(key) { return !!this._errors[key]; },
    markErr(key) { this._errors[key] = true; this._notify(key); },
    _notify(key) { this._listeners.forEach(fn => fn(key, this._data[key])); },
    on(fn) { this._listeners.push(fn); },
};

// ── API Module ──
let consecutiveFails = 0;

async function apiFetch(path, opts = {}) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 6000);
    try {
        const r = await fetch(`${API}${path}`, { ...opts, signal: controller.signal });
        if (!r.ok) { const text = await r.text().catch(() => ''); throw new Error(text || r.statusText); }
        consecutiveFails = 0;
        setOffline(false);
        return await r.json();
    } catch (e) {
        consecutiveFails++;
        if (consecutiveFails >= 2) setOffline(true);
        throw e;
    } finally {
        clearTimeout(timer);
    }
}

async function fetchActive() {
    try { store.set('activeProcesses', await apiFetch('/active/processes')); } catch { store.markErr('activeProcesses'); return null; }
}

async function fetchActiveSessions() {
    try { store.set('activeSessions', await apiFetch('/active/sessions')); } catch { store.markErr('activeSessions'); return null; }
}

async function stopActiveSession(pid, ppid, name) {
    const body = {};
    if (pid > 0) body.pid = pid;
    if (ppid > 0) body.ppid = ppid;
    body.name = name;
    try {
        await apiFetch('/active/sessions/stop', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
        });
        await fetchActiveSessions();
        renderDashboardActiveSessions();
        await fetchSessions();
        renderDashboardSummary();
        renderDashboardBuilding();
        renderDashboardDonut();
        toast('Stopped ' + name, 'success');
    } catch {
        toast('Failed to stop ' + name, 'error');
    }
}

async function startWatching(pid, extra = {}) {
    let label = 'process ' + pid;
    try {
        await apiFetch('/active/processes/watch', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ pid, ...extra }),
        });
        await fetchActiveSessions();
        renderProcessesView();
        toast('Watching ' + label, 'success');
    } catch {
        toast('Failed to watch ' + label, 'error');
    }
}

async function fetchSessions() {
    try { store.set('sessions', await apiFetch('/store/sessions')); } catch { store.markErr('sessions'); return null; }
}

async function fetchAutoWatch() {
    try { store.set('autoWatch', await apiFetch('/store/autowatch')); } catch { store.markErr('autoWatch'); return null; }
}

async function fetchAll() {
    await Promise.all([fetchActive(), fetchActiveSessions(), fetchSessions(), fetchAutoWatch()]);
}

async function addAutoWatch(name) {
    try {
        await apiFetch('/store/autowatch', {
            method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name }),
        });
        await fetchAutoWatch();
        renderDashboardWatchlist();
        toast('Added ' + name, 'success');
    } catch (e) {
        if (e.message && e.message.includes('already exists')) {
            toast('Already watching ' + name, 'error');
        } else {
            toast('Failed to add', 'error');
        }
    }
}

async function removeAutoWatch(name) {
    try {
        await apiFetch(`/store/autowatch/${encodeURIComponent(name)}`, { method: 'DELETE' });
        await fetchAutoWatch();
        renderDashboardWatchlist();
        toast('Removed ' + name, 'success');
    } catch {
        toast('Failed to remove', 'error');
    }
}

// ── Offline Detection ──
function setOffline(offline) {
    dom.offlineBanner.classList.toggle('visible', offline);
}

window.addEventListener('online', () => { consecutiveFails = 0; setOffline(false); });
window.addEventListener('offline', () => setOffline(true));

// ── Router ──
function initRouter() {
    function route() {
        const hash = location.hash || '#dashboard';
        document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));
        const view = document.getElementById('view-' + hash.slice(1));
        if (view) view.classList.add('active');
        dom.navLinks.querySelectorAll('a').forEach(a => a.classList.toggle('active', a.getAttribute('href') === hash));
        if (hash !== '#dashboard' && dashboardTimer) {
            clearInterval(dashboardTimer);
            dashboardTimer = null;
        }
        if (hash === '#processes') renderProcessesView();
        if (hash === '#history') renderHistoryView();
    }
    window.addEventListener('hashchange', route);
    route();
}

// ── Polling ──
let pollTimer = null;

function startPoll() {
    stopPoll();
    pollTimer = setInterval(async () => {
        await Promise.all([fetchActive(), fetchActiveSessions(), fetchSessions()]);
        const route = location.hash || '#dashboard';
        if (route === '#dashboard') {
            renderDashboardProcesses();
            if (typeof renderDashboardActiveSessions === 'function') renderDashboardActiveSessions();
            renderDashboardWatchlist();
            renderDashboardSummary();
            renderDashboardBuilding();
            renderDashboardDonut();
        }
        if (route === '#processes') renderProcessesView();
        if (route === '#history') renderHistoryView();

        if (!dom.detailPanel.hidden && dom.detailPanel.dataset.mode === 'process') {
            openDetail(dom.detailTitle.textContent);
        }
        updateActiveTimers();
    }, POLL_MS);
}

function stopPoll() {
    if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
    if (dashboardTimer) { clearInterval(dashboardTimer); dashboardTimer = null; }
}

document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
        stopPoll();
    } else {
        fetchAll().then(() => {
            const route = location.hash || '#dashboard';
            if (route === '#dashboard') renderDashboard();
            if (route === '#processes') renderProcessesView();
            if (route === '#history') renderHistoryView();
        });
        startPoll();
    }
});

// ── Time Helpers ──
function fmtDur(nanos) {
    if (!nanos || nanos <= 0) return '—';
    const m = Math.floor(nanos / 6e10);
    const h = Math.floor(m / 60);
    const min = m % 60;
    if (h === 0 && min === 0) return '0mins';
    if (h === 0) return min + 'mins';
    if (min === 0) return h + 'hr';
    return h + 'hr ' + min + 'mins';
}

function fmtDurShort(nanos) {
    if (!nanos || nanos <= 0) return '—';
    const m = Math.floor(nanos / 6e10);
    const h = Math.floor(m / 60);
    const min = m % 60;
    if (h === 0) return min + 'm';
    if (min === 0) return h + 'h';
    return h + 'h ' + min + 'm';
}

function fmtLiveDur(ms) {
    if (ms < 0) ms = 0;
    const totalSecs = Math.floor(ms / 1000);
    const h = Math.floor(totalSecs / 3600);
    const m = Math.floor((totalSecs % 3600) / 60);
    const s = totalSecs % 60;

    if (h > 0) return `${h}h ${m}m ${s}s`;
    if (m > 0) return `${m}m ${s}s`;
    return `${s}s`;
}

function fmtTime(iso) {
    if (!iso) return '—';
    const d = new Date(iso);
    return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}

function fmtDate(iso) {
    if (!iso) return '—';
    const d = new Date(iso);
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
}

function isToday(d) {
    const t = new Date(d), n = new Date();
    return t.getFullYear() === n.getFullYear() && t.getMonth() === n.getMonth() && t.getDate() === n.getDate();
}

function daysAgo(n) {
    const d = new Date(); d.setDate(d.getDate() - n); d.setHours(0, 0, 0, 0); return d;
}

// ── Computation ──
function weekSessions() {
    const cutoff = daysAgo(7);
    const ended = store.get('sessions').filter(s => new Date(s.start_time) >= cutoff);
    const active = store.get('activeSessions').filter(s => new Date(s.start_time) >= cutoff);
    return [...ended, ...active];
}

function mergeIntervals(sessions) {
    if (!sessions.length) return 0;
    const now = Date.now();
    const ranges = sessions
        .filter(s => s.start_time)
        .map(s => ({
            start: new Date(s.start_time).getTime(),
            end: s.end_time ? new Date(s.end_time).getTime() : now,
        }))
        .filter(r => r.end > r.start)
        .sort((a, b) => a.start - b.start);

    if (!ranges.length) return 0;

    let totalMs = 0;
    let curStart = ranges[0].start;
    let curEnd = ranges[0].end;

    for (let i = 1; i < ranges.length; i++) {
        const r = ranges[i];
        if (r.start <= curEnd) {
            if (r.end > curEnd) curEnd = r.end;
        } else {
            totalMs += curEnd - curStart;
            curStart = r.start;
            curEnd = r.end;
        }
    }
    totalMs += curEnd - curStart;
    return totalMs * 1e6;
}

function computeStats() {
    const ws = weekSessions();
    const totalDur = mergeIntervals(ws);
    const todayDur = mergeIntervals(ws.filter(s => isToday(s.start_time)));
    const daySet = new Set();
    ws.forEach(s => { const d = new Date(s.start_time); daySet.add(d.toDateString()); });
    const numDays = Math.max(daySet.size, 1);
    const avg = totalDur / numDays;
    const dayMap = {};
    ws.forEach(s => { const d = new Date(s.start_time).getDay(); dayMap[d] = (dayMap[d] || 0) + (s.duration || 0); });
    let bestDay = 0, bestVal = 0;
    for (const [d, v] of Object.entries(dayMap)) { if (v > bestVal) { bestVal = v; bestDay = +d; } }
    return { total: totalDur, today: todayDur, avg, bestDay: DAYS[bestDay], bestDayFull: DAYS_FULL[bestDay], numDays };
}

function computeDaily() {
    const now = new Date();
    const days = [];
    for (let i = 6; i >= 0; i--) {
        const d = new Date(now); d.setDate(d.getDate() - i);
        const key = d.toISOString().split('T')[0];
        days.push({ key, day: DAYS[d.getDay()], date: d, procs: {} });
    }
    weekSessions().forEach(s => {
        const key = new Date(s.start_time).toISOString().split('T')[0];
        const bucket = days.find(d => d.key === key);
        if (!bucket) return;
        const name = s.proc?.name || 'PID ' + s.process_id;
        bucket.procs[name] = (bucket.procs[name] || 0) + (s.duration || 0);
    });
    days.forEach(d => {
        const daySessions = weekSessions().filter(s => {
            const sk = new Date(s.start_time).toISOString().split('T')[0];
            return sk === d.key;
        });
        d.totalMerged = mergeIntervals(daySessions);
    });
    return days;
}

// ════════════════════════════════════════
//  DASHBOARD RENDERERS
// ════════════════════════════════════════

let dashboardTimer = null;

function renderDashboard() {
    renderDashboardProcesses();
    renderDashboardActiveSessions();
    renderDashboardSummary();
    renderDashboardBuilding();
    renderDashboardDonut();
    renderDashboardWatchlist();

    if (!dashboardTimer) {
        dashboardTimer = setInterval(updateActiveTimers, 1000);
    }
    updateActiveTimers();
}

// ── Processes ──
function renderDashboardProcesses() {
    const q = (dom.processSearch?.value || '').toLowerCase().trim();
    const all = store.get('activeProcesses');
    const list = q ? all.filter(p => p.name && p.name.toLowerCase().includes(q)) : all;

    dom.processCount.textContent = all.length + ' running';

    dom.processRetry.hidden = !store.err('activeProcesses');
    if (store.err('activeProcesses')) {
        dom.processList.innerHTML = '';
        return;
    }

    try { dom.processSkeleton.remove(); } catch (e) { /* already gone */ }

    if (!list.length && !all.length) {
        dom.processList.innerHTML = '<div class="empty-state"><p>' + (q ? 'No matching processes' : 'No active processes') + '</p></div>';
        return;
    }

    if (!list.length && all.length) {
        dom.processList.innerHTML = '<div class="empty-state"><p>No matching processes</p></div>';
        return;
    }

    dom.processList.innerHTML = list.map(p => {
        const c = COLORS[hashCode(p.name) % COLORS.length];
        return '<div class="process-item" data-name="' + esc(p.name) + '" data-pid="' + p.pid + '" tabindex="0" role="button" aria-label="' + esc(p.name) + '">' +
            '<span class="process-dot" style="background:' + c + ';box-shadow:0 0 6px ' + c + '44"></span>' +
            '<div class="process-item-info">' +
            '<span class="process-item-name">' + esc(p.name) + '</span>' +
            '<span class="process-item-meta">PID ' + p.pid + ' · PPID ' + p.ppid + '</span>' +
            '<span class="process-item-duration" data-start="' + p.start_time + '">0s</span>' +
            '</div></div>';
    }).join('');
}

// ── Active Sessions ──

function renderDashboardActiveSessions() {
    const list = store.get('activeSessions');

    if (dom.activeSessionsCount) dom.activeSessionsCount.textContent = list.length + ' tracking';

    if (store.err('activeSessions')) {
        if (dom.activeSessionsList) dom.activeSessionsList.innerHTML = errorState('Failed to load active sessions', 'activeSessions');
        return;
    }

    try { dom.activeSessionsSkeleton?.remove(); } catch (e) { }

    if (!list.length) {
        if (dom.activeSessionsList) dom.activeSessionsList.innerHTML = '<div class="empty-state"><p>No tracked sessions</p></div>';
        return;
    }

    if (dom.activeSessionsList) dom.activeSessionsList.innerHTML = list.map(s => {
        const name = s.proc?.name || 'PID ' + s.process_id;
        const pid = s.proc?.pid || 0;
        const ppid = s.proc?.ppid || 0;
        return '<div class="process-item active-session-item" data-pid="' + pid + '" data-ppid="' + ppid + '" data-name="' + esc(name) + '">' +
            '<span class="live-dot" style="flex-shrink:0;margin-right:4px;"></span>' +
            '<div class="process-item-info">' +
            '<span class="process-item-name">' + esc(name) + '</span>' +
            '<span class="process-item-meta">Started ' + fmtTime(s.start_time) + '</span>' +
            '</div>' +
            '<span class="process-time active-time" data-start="' + s.start_time + '">0s</span>' +
            '<button class="stop-btn" data-pid="' + pid + '" data-ppid="' + ppid + '" data-name="' + esc(name) + '" title="Stop watching" aria-label="Stop watching ' + esc(name) + '">Stop</button>' +
            '</div>';
    }).join('');
}

function updateActiveTimers() {
    document.querySelectorAll('.active-time, .process-item-duration, .proc-runtime').forEach(el => {
        const start = new Date(el.dataset.start);
        const diffMs = Date.now() - start.getTime();
        el.textContent = fmtLiveDur(diffMs);
    });
}

// ── Summary ──
function renderDashboardSummary() {
    const sessions = store.get('sessions');
    if (!sessions.length) {
        if (dom.summaryTotal) dom.summaryTotal.textContent = '—';
        if (dom.statToday) dom.statToday.textContent = '—';
        if (dom.statAvg) dom.statAvg.textContent = '—';
        if (dom.statActive) dom.statActive.textContent = '—';
        return;
    }
    try {
        const s = computeStats();
        if (dom.summaryTotal) dom.summaryTotal.textContent = fmtDur(s.total);
        if (dom.statToday) dom.statToday.textContent = fmtDur(s.today);
        if (dom.statTodayLabel) dom.statTodayLabel.textContent = 'today';
        if (dom.statAvg) dom.statAvg.textContent = fmtDur(s.avg);
        if (dom.statAvgLabel) dom.statAvgLabel.textContent = 'over ' + s.numDays + ' days';
        if (dom.statActive) dom.statActive.textContent = s.bestDay;
        if (dom.statActiveLabel) dom.statActiveLabel.textContent = 'top day';
    } catch (e) {
        console.error('trkr summary error:', e);
    }
}

// ── Building ──
function renderDashboardBuilding() {
    const sessions = store.get('sessions');
    if (store.err('sessions')) {
        dom.buildingChart.innerHTML = errorState('Failed to load session data', 'sessions');
        return;
    }
    if (!sessions.length) {
        dom.buildingChart.innerHTML = '<div class="empty-state"><p>No session data yet</p></div>';
        return;
    }
    const days = computeDaily();
    const allProcs = new Set();
    days.forEach(d => Object.keys(d.procs).forEach(p => allProcs.add(p)));
    const names = [...allProcs];
    const colorMap = {}; names.forEach((n, i) => colorMap[n] = COLORS[i % COLORS.length]);
    const maxT = Math.max(...days.map(d => d.totalMerged), 1);

    dom.buildingChart.innerHTML = days.map((d, i) => {
        const total = d.totalMerged;
        const h = Math.round((total / maxT) * 100) || 2;
        const floors = names.filter(n => d.procs[n]).map(n => {
            const fh = Math.round((d.procs[n] / total) * 100) || 1;
            return '<div class="floor" style="--h:' + fh + '%;--color:' + colorMap[n] + '"></div>';
        }).join('');
        const rows = names.filter(n => d.procs[n]).map(n =>
            '<div class="bt-row"><span class="bt-dot" style="background:' + colorMap[n] + '"></span>' + esc(n) + ' — ' + fmtDur(d.procs[n]) + '</div>'
        ).join('');
        return '<div class="building" style="--h:' + h + '%">' +
            '<div class="building-tooltip"><div class="bt-total">' + fmtDur(total) + ' total</div>' + rows + '</div>' +
            '<div class="building-stack">' + (floors || '<div class="floor" style="--h:2%;--color:var(--text-muted)"></div>') + '</div>' +
            '<span class="building-label">' + d.day + '</span></div>';
    }).join('');
}

// ── Donut ──
function renderDashboardDonut() {
    const sessions = store.get('sessions');
    if (store.err('sessions')) {
        dom.donutRing.innerHTML = '';
        dom.donutValue.textContent = '—';
        dom.donutLabel.textContent = '';
        dom.machineInfo.innerHTML = errorState('Failed to load session data', 'sessions');
        return;
    }
    if (!sessions.length) {
        dom.donutRing.innerHTML = '';
        dom.donutValue.textContent = '—';
        dom.donutLabel.textContent = '';
        dom.machineInfo.innerHTML = '<div class="machine-name">Process Breakdown</div><div class="machine-stats"><div class="machine-stat"><span class="ms-value">No data yet</span></div></div>';
        return;
    }

    const ws = weekSessions();
    const total = ws.reduce((a, s) => a + (s.duration || 0), 0) || 1;

    // Group by process name
    const procMap = {};
    ws.forEach(s => {
        const name = s.proc?.name || 'PID ' + s.process_id;
        procMap[name] = (procMap[name] || 0) + (s.duration || 0);
    });

    // Sort by duration descending
    const sortedProcs = Object.entries(procMap)
        .sort((a, b) => b[1] - a[1])
        .slice(0, 4); // top 4

    dom.donutValue.textContent = fmtDur(total);
    dom.donutLabel.textContent = 'Total';

    const r = 38, circ = 2 * Math.PI * r;
    const cx = 50, cy = 50;

    let segments = '';
    let offset = 0;

    sortedProcs.forEach(([name, val], i) => {
        const pct = val / total;
        const len = pct * circ;
        const color = COLORS[hashCode(name) % COLORS.length];

        segments += '<circle class="donut-ring-segment" cx="' + cx + '" cy="' + cy + '" r="' + r +
            '" stroke="' + color + '" stroke-dasharray="' + len + ' ' + (circ - len) +
            '" stroke-dashoffset="' + (-offset) + '"><title>' + esc(name) + '</title></circle>';
        offset += len;
    });

    const topTotal = sortedProcs.reduce((a, [_, val]) => a + val, 0);
    if (topTotal < total) {
        const otherVal = total - topTotal;
        const pct = otherVal / total;
        const len = pct * circ;
        segments += '<circle class="donut-ring-segment" cx="' + cx + '" cy="' + cy + '" r="' + r +
            '" stroke="var(--surface-hover)" stroke-dasharray="' + len + ' ' + (circ - len) +
            '" stroke-dashoffset="' + (-offset) + '"><title>Other</title></circle>';
    }

    dom.donutRing.innerHTML = '<svg width="100%" height="100%" viewBox="0 0 100 100">' +
        '<circle class="donut-ring-bg" cx="' + cx + '" cy="' + cy + '" r="' + r + '"></circle>' +
        segments + '</svg>';

    let infoHtml = '<div class="machine-name">Top Processes</div><div class="machine-stats">';

    sortedProcs.slice(0, 3).forEach(([name, val]) => {
        const color = COLORS[hashCode(name) % COLORS.length];
        const pct = Math.round((val / total) * 100);
        infoHtml += `<div class="machine-stat" style="flex-direction:row;justify-content:space-between;align-items:center;">` +
            `<div style="display:flex;align-items:center;gap:6px;">` +
            `<span class="proc-dot" style="background:${color};width:6px;height:6px;border-radius:50%;display:inline-block;"></span>` +
            `<span class="ms-value" style="font-size:0.75rem;">${esc(name)}</span>` +
            `</div>` +
            `<span class="ms-detail">${pct}% (${fmtDur(val)})</span>` +
            `</div>`;
    });

    infoHtml += '</div>';
    dom.machineInfo.innerHTML = infoHtml;
}

// ── Watchlist ──
function renderDashboardWatchlist() {
    const items = store.get('autoWatch');
    dom.watchlistCount.textContent = items.length;
    if (store.err('autoWatch')) {
        dom.watchlistList.innerHTML = errorState('Failed to load watchlist', 'autoWatch');
        return;
    }
    if (!items.length) {
        dom.watchlistList.innerHTML = '<div class="watchlist-empty"><p>No processes watched</p></div>';
        return;
    }
    const active = store.get('activeProcesses');
    dom.watchlistList.innerHTML = items.map(w => {
        const live = active.some(p => p.name === w.name);
        return '<div class="watchlist-item">' +
            '<span class="watchlist-dot' + (live ? ' active' : '') + '"></span>' +
            '<span class="watchlist-name">' + esc(w.name) + '</span>' +
            '<button class="watchlist-remove" data-name="' + esc(w.name) + '" title="Remove" aria-label="Remove ' + esc(w.name) + '">×</button></div>';
    }).join('');
    dom.watchlistList.querySelectorAll('.watchlist-remove').forEach(btn => {
        btn.addEventListener('click', e => { e.stopPropagation(); removeAutoWatch(btn.dataset.name); });
    });
}

// ── Watchlist Suggestions ──
function renderWatchlistSuggestions() {
    const q = (dom.watchlistInput?.value || '').toLowerCase().trim();
    const el = dom.watchlistSuggestions;
    if (!el) return;

    if (!q) { el.hidden = true; return; }

    const active = store.get('activeProcesses');
    const watchNames = new Set(store.get('autoWatch').map(w => w.name.toLowerCase()));
    const seen = new Set();
    const matches = [];
    active.forEach(p => {
        if (!p.name) return;
        const lower = p.name.toLowerCase();
        if (!lower.includes(q)) return;
        if (watchNames.has(lower)) return;
        if (seen.has(lower)) return;
        seen.add(lower);
        matches.push(p.name);
    });

    if (!matches.length) { el.hidden = true; return; }

    el.hidden = false;
    el.innerHTML = matches.map(name => {
        const c = COLORS[hashCode(name) % COLORS.length];
        return '<div class="watchlist-suggestion" data-name="' + esc(name) + '" tabindex="0" role="button">' +
            '<span class="process-dot" style="background:' + c + ';box-shadow:0 0 6px ' + c + '44;width:6px;height:6px"></span>' +
            '<span class="watchlist-suggestion-name">' + esc(name) + '</span>' +
            '<span class="watchlist-add-label">Add</span>' +
            '</div>';
    }).join('');

    el.querySelectorAll('.watchlist-suggestion').forEach(s => {
        s.addEventListener('click', () => {
            addAutoWatch(s.dataset.name);
            dom.watchlistInput.value = '';
            el.hidden = true;
            dom.watchlistInput.focus();
        });
        s.addEventListener('keydown', e => {
            if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                addAutoWatch(s.dataset.name);
                dom.watchlistInput.value = '';
                el.hidden = true;
                dom.watchlistInput.focus();
            }
        });
    });
}

// ════════════════════════════════════════
//  PROCESSES VIEW
// ════════════════════════════════════════

function renderProcessesView() {
    const filter = dom.processesFilter?.value || 'active';
    const activeProcesses = store.get('activeProcesses');
    const watchNames = new Set(store.get('autoWatch').map(w => w.name.toLowerCase()));
    const q = (dom.processesSearch?.value || '').toLowerCase().trim();
    const sort = dom.processesSort?.value || 'duration';
    const expanded = dom.processesExpandBtn?.dataset.expanded === 'true';

    if (store.err('activeProcesses')) {
        dom.processesTbody.innerHTML = '';
        dom.processesEmpty.hidden = false;
        dom.processesEmpty.querySelector('p').textContent = 'Failed to load processes';
        return;
    }

    const matchesSearch = p => {
        if (!q) return true;
        if (p.name && p.name.toLowerCase().includes(q)) return true;
        if (p.pid && String(p.pid).includes(q)) return true;
        if (p.ppid && String(p.ppid).includes(q)) return true;
        return false;
    };

    // Compute session data by name and by PID
    const now = Date.now();
    const sesDurByName = {};
    const sesHasParent = {};
    const sesByPid = {};
    store.get('activeSessions').forEach(s => {
        if (!s.proc?.name) return;
        const name = s.proc.name.toLowerCase();
        if (s.proc.is_parent) sesHasParent[name] = true;
        const elapsedMs = now - new Date(s.start_time).getTime();
        if (elapsedMs > 0) {
            sesDurByName[name] = (sesDurByName[name] || 0) + elapsedMs * 1e6;
        }
        if (s.proc.pid && elapsedMs > 0) {
            sesByPid[s.proc.pid] = { duration: elapsedMs * 1e6, isParent: s.proc.is_parent };
        }
    });

    let raw = activeProcesses.filter(matchesSearch);

    let list;
    if (expanded) {
        // Show individual PIDs, no name aggregation
        list = raw.map(p => ({ ...p, _expanded: true }));
    } else {
        // Aggregate duplicates by name
        const nameMap = new Map();
        raw.forEach(p => {
            const lower = p.name.toLowerCase();
            if (!nameMap.has(lower)) {
                nameMap.set(lower, { ...p, pids: new Set([p.pid]) });
            } else {
                nameMap.get(lower).pids.add(p.pid);
            }
        });
        list = Array.from(nameMap.values()).map(p => ({
            ...p,
            _duration: sesDurByName[p.name.toLowerCase()] || 0
        }));
    }

    if (filter === 'watching') {
        if (expanded) {
            // Show all processes with active sessions
            list = list.filter(p => sesByPid[p.pid] != null);
        } else {
            list = list.filter(p => watchNames.has((p.name || '').toLowerCase()) && p.is_parent);
        }
    }

    if (filter === 'user') {
        const sysProcs = list.length && list[0].os === 'windows' ? SYSTEM_PROCS_WINDOWS : SYSTEM_PROCS_LINUX;
        list = list.filter(p => !sysProcs.has((p.name || '').toLowerCase()));
    }

    if (filter === 'system') {
        const sysProcs = list.length && list[0].os === 'windows' ? SYSTEM_PROCS_WINDOWS : SYSTEM_PROCS_LINUX;
        list = list.filter(p => sysProcs.has((p.name || '').toLowerCase()));
    }

    if (sort === 'duration') list.sort((a, b) => {
        const da = expanded ? (sesByPid[a.pid]?.duration || 0) : (a._duration || 0);
        const db = expanded ? (sesByPid[b.pid]?.duration || 0) : (b._duration || 0);
        return db - da;
    });
    else if (sort === 'name') list.sort((a, b) => (a.name || '').localeCompare(b.name || ''));
    else if (sort === 'pid') list.sort((a, b) => {
        const pa = typeof a.pid === 'number' ? a.pid : 0;
        const pb = typeof b.pid === 'number' ? b.pid : 0;
        return pa - pb;
    });

    if (!list.length) {
        dom.processesTbody.innerHTML = '';
        dom.processesEmpty.hidden = false;
        return;
    }
    dom.processesEmpty.hidden = true;

    dom.processesTbody.innerHTML = list.map(p => {
        const c = COLORS[hashCode(p.name) % COLORS.length];
        const os = p.os || 'Unknown';
        const device = p.device_name || 'Local';
        const isAutoWatched = watchNames.has((p.name || '').toLowerCase());
        const autoBadge = isAutoWatched ? '<span class="auto-badge" title="Auto-watched">A</span>' : '';

        let hasSession, isParentSession, pidDisplay, duration;
        if (expanded) {
            const ses = sesByPid[p.pid];
            hasSession = ses != null;
            isParentSession = ses ? ses.isParent : false;
            pidDisplay = p.pid;
            duration = ses ? ses.duration : 0;
        } else {
            hasSession = (sesDurByName[p.name.toLowerCase()] || 0) > 0;
            isParentSession = sesHasParent[p.name.toLowerCase()] === true;
            pidDisplay = p.pids && p.pids.size > 1 ? p.pids.size + ' procs' : p.pid;
            duration = p._duration || 0;
        }

        let watchBtn;
        if (hasSession) {
            const label = isParentSession ? 'Watching' : 'Watching (child)';
            watchBtn = `<button class="action-btn remove-watch" data-name="${esc(p.name)}" data-pid="${p.pid}" data-ppid="${p.ppid}" title="Stop watching">${label}</button>`;
        } else {
            watchBtn = `<button class="action-btn add-watch" data-pid="${p.pid}" title="Watch">Watch</button>`;
        }

        const watchParent = (!p.is_parent)
            ? `<button class="watch-parent" data-pid="${p.ppid}" title="Watch parent process">Watch parent</button>`
            : '';

        const runtimeStart = p.start_time || '';
        const runtimeStr = runtimeStart ? fmtLiveDur(Date.now() - new Date(runtimeStart).getTime()) : '—';
        return `<tr class="clickable-row" data-name="${esc(p.name)}">` +
            `<td><div class="proc-name"><span class="proc-dot" style="background:${c};box-shadow:0 0 6px ${c}44"></span>${esc(p.name)}${autoBadge}</div></td>` +
            `<td><span class="proc-device">${esc(device)} <span class="proc-os">(${esc(os)})</span></span></td>` +
            `<td><span class="proc-pid">${pidDisplay}</span></td>` +
            `<td><span class="proc-pid">${p.ppid || '—'}</span></td>` +
            `<td><span class="proc-runtime" data-start="${runtimeStart}">${runtimeStr}</span></td>` +
            `<td><span class="proc-duration">${fmtDur(duration)}</span></td>` +
            `<td><span class="proc-status proc-status-active"><span class="live-dot" style="width:5px;height:5px;box-shadow:0 0 6px var(--green-glow)"></span>Active</span></td>` +
            `<td><div class="proc-actions">${watchBtn}${watchParent}</div></td>` +
            '</tr>';
    }).join('');
}

// ════════════════════════════════════════
//  HISTORY VIEW
// ════════════════════════════════════════

let historyLimit = 50;

function renderHistoryView(resetLimit = false) {
    if (resetLimit) historyLimit = 50;

    const sessions = store.get('sessions');
    if (!sessions.length) {
        dom.historyList.innerHTML = '<div class="empty-state"><p>No session history</p></div>';
        if (dom.historyLoadMoreWrap) dom.historyLoadMoreWrap.hidden = true;
        return;
    }

    const q = (dom.historySearch?.value || '').toLowerCase().trim();
    const filter = dom.historyFilter?.value || 'week';
    const sort = dom.historySort?.value || 'newest';

    let filtered = q ? sessions.filter(s => (s.proc?.name || 'PID ' + s.process_id).toLowerCase().includes(q)) : [...sessions];

    filtered = filtered.filter(s => {
        const d = new Date(s.start_time);
        if (filter === 'today') return isToday(s.start_time);
        if (filter === 'week') return d >= daysAgo(7);
        if (filter === 'month') return d >= daysAgo(30);
        return true;
    });

    if (sort === 'newest') {
        filtered.sort((a, b) => new Date(b.start_time) - new Date(a.start_time));
    } else if (sort === 'oldest') {
        filtered.sort((a, b) => new Date(a.start_time) - new Date(b.start_time));
    } else if (sort === 'longest') {
        filtered.sort((a, b) => (b.duration || 0) - (a.duration || 0));
    }

    const paginated = filtered.slice(0, historyLimit);

    if (dom.historyLoadMoreWrap) {
        dom.historyLoadMoreWrap.hidden = paginated.length >= filtered.length;
    }

    if (!paginated.length) {
        dom.historyList.innerHTML = '<div class="empty-state"><p>No sessions match criteria</p></div>';
        return;
    }

    let html = '';
    if (sort === 'longest') {
        html = paginated.map(s => renderHistoryItem(s)).join('');
    } else {
        let lastDate = '';
        paginated.forEach(s => {
            const dateStr = new Date(s.start_time).toDateString();
            if (dateStr !== lastDate) {
                const dateLabel = isToday(s.start_time) ? 'Today' : fmtDate(s.start_time);
                html += `<div class="history-date-header">${dateLabel}</div>`;
                lastDate = dateStr;
            }
            html += renderHistoryItem(s);
        });
    }

    dom.historyList.innerHTML = html;
}

function renderHistoryItem(s) {
    const name = s.proc?.name || 'PID ' + s.process_id;
    const c = COLORS[hashCode(name) % COLORS.length];

    const isOngoing = !s.end_time;
    const startStr = fmtTime(s.start_time);
    const endStr = isOngoing ? 'Active' : fmtTime(s.end_time);

    let trackedNanos = s.duration || 0;
    if (isOngoing && s.start_time) {
        const elapsed = Date.now() - new Date(s.start_time).getTime();
        if (elapsed > 0) trackedNanos = elapsed * 1e6;
    }

    let procRuntimeNanos = 0;
    if (s.proc?.start_time) {
        const procStart = new Date(s.proc.start_time).getTime();
        const end = s.end_time ? new Date(s.end_time).getTime() : Date.now();
        const elapsedMs = end - procStart;
        if (elapsedMs > 0) procRuntimeNanos = elapsedMs * 1e6;
    }

    return '<div class="history-item clickable-row" data-sid="' + s.id + '" tabindex="0" role="button">' +
        '<span class="process-dot" style="background:' + c + ';width:8px;height:8px;box-shadow:0 0 6px ' + c + '44;flex-shrink:0"></span>' +
        '<div class="history-item-info">' +
        '<span class="history-item-name">' + esc(name) + '</span>' +
        '<span class="history-item-meta">' + startStr + ' → ' + endStr + '</span>' +
        '</div>' +
        '<span class="history-item-duration">' +
        '<span class="history-item-label">tracked</span>' +
        '<span class="' + (isOngoing ? 'live-text' : '') + '">' + fmtDur(trackedNanos) + '</span>' +
        '</span>' +
        '<span class="history-item-duration">' +
        '<span class="history-item-label">runtime</span>' +
        '<span>' + fmtDur(procRuntimeNanos) + '</span>' +
        '</span>' +
        '</div>';
}

// ════════════════════════════════════════
//  DETAIL PANEL
// ════════════════════════════════════════

function openDetail(name) {
    const activeProcs = store.get('activeProcesses');
    const activeSessions = store.get('activeSessions');
    const sessions = store.get('sessions');
    const autoWatch = store.get('autoWatch');

    // Find process info
    let p = activeProcs.find(item => item.name === name);
    if (!p) {
        const activeSes = activeSessions.find(s => s.proc?.name === name);
        if (activeSes) p = activeSes.proc;
    }
    if (!p) {
        const ses = sessions.find(s => s.proc?.name === name);
        if (ses) p = ses.proc;
    }

    const procSessions = sessions.filter(s => (s.proc?.name || 'PID ' + s.process_id) === name);
    const totalDur = procSessions.reduce((a, s) => a + (s.duration || 0), 0);
    const isActive = activeProcs.some(item => item.name === name) || activeSessions.some(s => s.proc?.name === name);
    const isAutoWatched = autoWatch.some(w => w.name.toLowerCase() === (name || '').toLowerCase());
    const activeSes = activeSessions.find(s => s.proc?.name === name);

    dom.detailTitle.textContent = name;

    let html = '<div class="detail-section">' +
        '<div class="detail-section-title">Process Info</div>' +
        (activeSes ? '<div class="detail-current-session"><span class="detail-label">Current Session</span><span class="live-text">' + fmtLiveDur(Date.now() - new Date(activeSes.start_time).getTime()) + '</span></div>' : '') +
        '<div class="detail-row"><span class="detail-label">Status</span><span class="detail-value" style="color:' + (isActive ? 'var(--green)' : 'var(--text-dim)') + '">' + (isActive ? 'Active' : 'Inactive') + '</span></div>' +
        (p ? '<div class="detail-row"><span class="detail-label">Device</span><span class="detail-value">' + esc(p.device_name || 'Local') + ' (' + esc(p.os || 'Unknown') + ')</span></div>' : '') +
        (p ? '<div class="detail-row"><span class="detail-label">PID</span><span class="detail-value">' + p.pid + '</span></div>' : '') +
        (p ? '<div class="detail-row"><span class="detail-label">PPID</span><span class="detail-value">' + p.ppid + '</span></div>' : '') +
        (p && p.ppid > 0 ? '<div class="detail-row"><span class="detail-label">Parent</span><span class="detail-value">' + esc((activeProcs.find(ap => ap.pid === p.ppid) || {}).name || '—') + '</span></div>' : '') +
        (p && p.start_time ? '<div class="detail-row"><span class="detail-label">Runtime</span><span class="detail-value">' + fmtLiveDur(Date.now() - new Date(p.start_time).getTime()) + '</span></div>' : '') +
        '<div class="detail-row"><span class="detail-label">Total Duration</span><span class="detail-value">' + fmtDur(totalDur) + '</span></div>' +
        '<div class="detail-row"><span class="detail-label">Sessions</span><span class="detail-value">' + procSessions.length + '</span></div>' +
        '</div>';

    html += '<div class="detail-section">' +
        '<div class="detail-section-title">Actions & Watchlist</div>' +
        '<div class="detail-actions" style="display:flex;gap:8px;flex-wrap:wrap;">';

    if (activeSes) {
        const pid = activeSes.proc?.pid || 0;
        const ppid = activeSes.proc?.ppid || 0;
        html += `<button class="action-btn remove-watch detail-act-stop" data-pid="${pid}" data-ppid="${ppid}" data-name="${esc(name)}">Stop Session</button>`;
    } else if (p && p.pid) {
        html += `<button class="action-btn add-watch detail-act-start" data-pid="${p.pid}">Watch Process</button>`;
    }

    if (isAutoWatched) {
        html += `<button class="action-btn remove-watch detail-act-unauto" data-name="${esc(name)}">Remove Auto-watch</button>`;
    } else {
        html += `<button class="action-btn add-watch detail-act-auto" data-name="${esc(name)}">Add Auto-watch</button>`;
    }
    html += '</div></div>';

    dom.detailBody.innerHTML = html;
    dom.detailPanel.hidden = false;
    dom.detailPanel.dataset.mode = 'process';
    lastFocusedEl = document.activeElement;
    document.body.style.overflow = 'hidden';
    dom.detailClose?.focus();
    document.addEventListener('keydown', trapFocus);

    // Bind action buttons inside detail panel
    const stopBtn = dom.detailBody.querySelector('.detail-act-stop');
    if (stopBtn) {
        stopBtn.addEventListener('click', async () => {
            await stopActiveSession(parseInt(stopBtn.dataset.pid) || 0, parseInt(stopBtn.dataset.ppid) || 0, name);
            openDetail(name);
        });
    }
    const startBtn = dom.detailBody.querySelector('.detail-act-start');
    if (startBtn) {
        startBtn.addEventListener('click', async () => {
            await startWatching(parseInt(startBtn.dataset.pid) || 0);
            openDetail(name);
        });
    }
    const autoBtn = dom.detailBody.querySelector('.detail-act-auto');
    if (autoBtn) {
        autoBtn.addEventListener('click', async () => {
            await addAutoWatch(name);
            openDetail(name);
        });
    }
    const unautoBtn = dom.detailBody.querySelector('.detail-act-unauto');
    if (unautoBtn) {
        unautoBtn.addEventListener('click', async () => {
            await removeAutoWatch(name);
            openDetail(name);
        });
    }
}

let lastFocusedEl = null;

function trapFocus(e) {
    if (dom.detailPanel.hidden) return;
    const focusable = dom.detailPanel.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
    if (!focusable.length) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (e.key === 'Tab') {
        if (e.shiftKey && document.activeElement === first) {
            e.preventDefault();
            last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
            e.preventDefault();
            first.focus();
        }
    }
}

function closeDetail() {
    dom.detailPanel.hidden = true;
    document.body.style.overflow = '';
    document.removeEventListener('keydown', trapFocus);
    if (lastFocusedEl) {
        lastFocusedEl.focus();
        lastFocusedEl = null;
    }
}

function openSessionDetail(session) {
    const p = session.proc;
    const name = p?.name || 'PID ' + session.process_id;
    const isActive = !session.end_time;
    const now = Date.now();

    const allSessions = store.get('sessions');
    const activeProcs = store.get('activeProcesses');
    const autoWatch = store.get('autoWatch');
    const activeSessions = store.get('activeSessions');

    const procSessions = allSessions.filter(s => (s.proc?.name || 'PID ' + s.process_id) === name);
    const totalDur = procSessions.reduce((a, s) => a + (s.duration || 0), 0);
    const isAutoWatched = autoWatch.some(w => w.name.toLowerCase() === (name || '').toLowerCase());
    const activeSes = activeSessions.find(s => s.proc?.name === name);

    const liveDur = isActive && session.start_time
        ? fmtLiveDur(now - new Date(session.start_time).getTime())
        : fmtDur(session.duration || 0);

    dom.detailTitle.textContent = name;

    let html = '<div class="detail-section">' +
        '<div class="detail-section-title">Session Info</div>' +
        (isActive ? '<div class="detail-current-session"><span class="detail-label">Current Session</span><span class="live-text">' + liveDur + '</span></div>' : '') +
        '<div class="detail-row"><span class="detail-label">Status</span><span class="detail-value" style="color:' + (isActive ? 'var(--green)' : 'var(--text-dim)') + '">' + (isActive ? 'Active' : 'Ended') + '</span></div>' +
        '<div class="detail-row"><span class="detail-label">Duration</span><span class="detail-value' + (isActive ? ' live-text' : '') + '">' + (isActive ? liveDur : fmtDur(session.duration || 0)) + '</span></div>' +
        '<div class="detail-row"><span class="detail-label">Started</span><span class="detail-value">' + fmtTime(session.start_time) + '</span></div>' +
        '<div class="detail-row"><span class="detail-label">Ended</span><span class="detail-value">' + (session.end_time ? fmtTime(session.end_time) : '—') + '</span></div>' +
        '<div class="detail-row"><span class="detail-label">Session ID</span><span class="detail-value">#' + session.id + '</span></div>' +
        '</div>';

    if (p) {
        html += '<div class="detail-section">' +
            '<div class="detail-section-title">Process Info</div>' +
            '<div class="detail-row"><span class="detail-label">Name</span><span class="detail-value">' + esc(p.name) + '</span></div>' +
            '<div class="detail-row"><span class="detail-label">PID</span><span class="detail-value">' + p.pid + '</span></div>' +
            '<div class="detail-row"><span class="detail-label">PPID</span><span class="detail-value">' + p.ppid + '</span></div>' +
            (p.ppid > 0 ? '<div class="detail-row"><span class="detail-label">Parent</span><span class="detail-value">' + esc((activeProcs.find(ap => ap.pid === p.ppid) || {}).name || '—') + '</span></div>' : '') +
            '<div class="detail-row"><span class="detail-label">Device</span><span class="detail-value">' + esc(p.device_name || 'Local') + ' (' + esc(p.os || 'Unknown') + ')</span></div>' +
            (p.start_time ? '<div class="detail-row"><span class="detail-label">Runtime</span><span class="detail-value">' + fmtLiveDur(now - new Date(p.start_time).getTime()) + '</span></div>' : '') +
            '<div class="detail-row"><span class="detail-label">Total Duration</span><span class="detail-value">' + fmtDur(totalDur) + '</span></div>' +
            '<div class="detail-row"><span class="detail-label">Sessions</span><span class="detail-value">' + procSessions.length + '</span></div>' +
            '</div>';
    }

    html += '<div class="detail-section">' +
        '<div class="detail-section-title">Actions & Watchlist</div>' +
        '<div class="detail-actions" style="display:flex;gap:8px;flex-wrap:wrap;">';

    if (activeSes) {
        const pid = activeSes.proc?.pid || 0;
        const ppid = activeSes.proc?.ppid || 0;
        html += '<button class="action-btn remove-watch detail-act-stop" data-pid="' + pid + '" data-ppid="' + ppid + '" data-name="' + esc(name) + '">Stop Session</button>';
    } else if (p && p.pid) {
        html += '<button class="action-btn add-watch detail-act-start" data-pid="' + p.pid + '">Watch Process</button>';
    }

    if (isAutoWatched) {
        html += '<button class="action-btn remove-watch detail-act-unauto" data-name="' + esc(name) + '">Remove Auto-watch</button>';
    } else {
        html += '<button class="action-btn add-watch detail-act-auto" data-name="' + esc(name) + '">Add Auto-watch</button>';
    }
    html += '</div></div>';

    dom.detailBody.innerHTML = html;
    dom.detailPanel.hidden = false;
    dom.detailPanel.dataset.mode = 'session';
    lastFocusedEl = document.activeElement;
    document.body.style.overflow = 'hidden';
    dom.detailClose?.focus();
    document.addEventListener('keydown', trapFocus);

    const stopBtn = dom.detailBody.querySelector('.detail-act-stop');
    if (stopBtn) {
        stopBtn.addEventListener('click', async () => {
            await stopActiveSession(parseInt(stopBtn.dataset.pid) || 0, parseInt(stopBtn.dataset.ppid) || 0, name);
            openDetail(name);
        });
    }
    const startBtn = dom.detailBody.querySelector('.detail-act-start');
    if (startBtn) {
        startBtn.addEventListener('click', async () => {
            await startWatching(parseInt(startBtn.dataset.pid) || 0);
            openDetail(name);
        });
    }
    const autoBtn = dom.detailBody.querySelector('.detail-act-auto');
    if (autoBtn) {
        autoBtn.addEventListener('click', async () => {
            await addAutoWatch(name);
            openDetail(name);
        });
    }
    const unautoBtn = dom.detailBody.querySelector('.detail-act-unauto');
    if (unautoBtn) {
        unautoBtn.addEventListener('click', async () => {
            await removeAutoWatch(name);
            openDetail(name);
        });
    }
}

if (dom.detailBackdrop) dom.detailBackdrop.addEventListener('click', closeDetail);
if (dom.detailClose) dom.detailClose.addEventListener('click', closeDetail);
document.addEventListener('keydown', e => { if (e.key === 'Escape') closeDetail(); });

// ════════════════════════════════════════
//  EVENT HANDLERS
// ════════════════════════════════════════

let debounceDash = null;
let debounceProcs = null;

if (dom.processSearch) {
    dom.processSearch.addEventListener('input', () => {
        clearTimeout(debounceDash);
        debounceDash = setTimeout(renderDashboardProcesses, 150);
    });
}

// Keyboard shortcuts
document.addEventListener('keydown', e => {
    if (e.key === '/' && document.activeElement !== dom.processSearch && document.activeElement !== dom.watchlistInput && document.activeElement !== dom.processesSearch && document.activeElement !== dom.historySearch) {
        e.preventDefault();
        const view = location.hash || '#dashboard';
        if (view === '#dashboard') dom.processSearch?.focus();
        else if (view === '#processes') dom.processesSearch?.focus();
        else if (view === '#history') dom.historySearch?.focus();
    }
    if (e.key === 'Escape' && document.activeElement === dom.processSearch) {
        dom.processSearch.value = ''; dom.processSearch.blur(); renderDashboardProcesses();
    }
    if (e.key === 'Escape' && document.activeElement === dom.processesSearch) {
        dom.processesSearch.value = ''; dom.processesSearch.blur(); renderProcessesView();
    }
    if (e.key === '?' && !e.ctrlKey && !e.metaKey) {
        e.preventDefault();
        toast('Shortcuts: / search · Esc clear · ↑↓ navigate · Enter detail · ? help', 'info');
    }
});

// Dashboard process list keyboard nav
if (dom.processList) {
    dom.processList.addEventListener('keydown', e => {
        const items = dom.processList.querySelectorAll('.process-item');
        if (!items.length) return;
        const idx = Array.from(items).indexOf(document.activeElement);
        if (e.key === 'ArrowDown') { e.preventDefault(); items[Math.min(idx + 1, items.length - 1)]?.focus(); }
        if (e.key === 'ArrowUp') { e.preventDefault(); items[Math.max(idx - 1, 0)]?.focus(); }
    });

    dom.processList?.addEventListener('click', e => {
        const item = e.target.closest('.process-item');
        if (item) {
            document.querySelectorAll('.process-item').forEach(i => i.classList.remove('selected'));
            item.classList.add('selected');
            openDetail(item.dataset.name);
        }
    });

    dom.activeSessionsList?.addEventListener('click', e => {
        const stopBtn = e.target.closest('.stop-btn');
        if (stopBtn) {
            e.stopPropagation();
            stopActiveSession(parseInt(stopBtn.dataset.pid) || 0, parseInt(stopBtn.dataset.ppid) || 0, stopBtn.dataset.name);
            return;
        }
        const item = e.target.closest('.process-item');
        if (item) {
            document.querySelectorAll('.process-item').forEach(i => i.classList.remove('selected'));
            item.classList.add('selected');
            openDetail(item.dataset.name);
        }
    });
}

// Processes view search/sort
if (dom.processesSearch) {
    dom.processesSearch.addEventListener('input', () => {
        clearTimeout(debounceProcs);
        debounceProcs = setTimeout(renderProcessesView, 150);
    });
}
if (dom.processesSort) dom.processesSort.addEventListener('change', renderProcessesView);
if (dom.processesFilter) dom.processesFilter.addEventListener('change', renderProcessesView);
if (dom.processesExpandBtn) {
    dom.processesExpandBtn.addEventListener('click', () => {
        const expanded = dom.processesExpandBtn.dataset.expanded === 'true';
        dom.processesExpandBtn.dataset.expanded = expanded ? 'false' : 'true';
        dom.processesExpandBtn.textContent = expanded ? 'Expand' : 'Compact';
        renderProcessesView();
    });
}

// Process table keyboard nav
document.addEventListener('keydown', e => {
    if (e.key === 'Enter' || e.key === ' ') {
        const row = e.target.closest('.clickable-row, .history-item, .process-item');
        if (row) {
            e.preventDefault();
            if (row.dataset.name) {
                openDetail(row.dataset.name);
            } else if (row.dataset.sid) {
                const sid = parseInt(row.dataset.sid);
                const session = store.get('sessions').find(s => s.id === sid);
                if (session) openSessionDetail(session);
            }
        }
    }
});

// Process table click events
dom.processesTbody?.addEventListener('click', e => {
    // Handle watch-parent buttons
    const watchParent = e.target.closest('.watch-parent');
    if (watchParent) {
        e.stopPropagation();
        const pid = parseInt(watchParent.dataset.pid);
        if (pid > 0) startWatching(pid, { watch_parent: true });
        return;
    }
    // Handle watch/unwatch buttons
    const watchBtn = e.target.closest('.add-watch');
    if (watchBtn) {
        e.stopPropagation();
        const pid = parseInt(watchBtn.dataset.pid);
        if (pid > 0) startWatching(pid);
        return;
    }
    const removeBtn = e.target.closest('.remove-watch');
    if (removeBtn) {
        e.stopPropagation();
        stopActiveSession(parseInt(removeBtn.dataset.pid) || 0, parseInt(removeBtn.dataset.ppid) || 0, removeBtn.dataset.name);
        return;
    }

    // Handle row clicks
    const row = e.target.closest('.clickable-row');
    if (row) {
        openDetail(row.dataset.name);
    }
});

// History view events
let debounceHist = null;
if (dom.historySearch) {
    dom.historySearch.addEventListener('input', () => {
        clearTimeout(debounceHist);
        debounceHist = setTimeout(() => renderHistoryView(true), 150);
    });
}
if (dom.historyFilter) dom.historyFilter.addEventListener('change', () => renderHistoryView(true));
if (dom.historySort) dom.historySort.addEventListener('change', () => renderHistoryView(true));

// History item click → open session detail
dom.historyList?.addEventListener('click', e => {
    const item = e.target.closest('.history-item');
    if (item) {
        const sid = parseInt(item.dataset.sid);
        const sessions = store.get('sessions');
        const session = sessions.find(s => s.id === sid);
        if (session) openSessionDetail(session);
    }
});
if (dom.historyLoadMore) {
    dom.historyLoadMore.addEventListener('click', () => {
        historyLimit += 50;
        renderHistoryView();
    });
}

// Watchlist
if (dom.watchlistAddBtn) {
    dom.watchlistAddBtn.addEventListener('click', () => {
        const v = dom.watchlistInput?.value?.trim();
        if (v) { addAutoWatch(v); dom.watchlistInput.value = ''; }
        if (dom.watchlistSuggestions) dom.watchlistSuggestions.hidden = true;
    });
}
if (dom.watchlistInput) {
    dom.watchlistInput.addEventListener('input', renderWatchlistSuggestions);
    dom.watchlistInput.addEventListener('keydown', e => {
        if (e.key === 'Enter') {
            const v = dom.watchlistInput.value.trim();
            if (v) { addAutoWatch(v); dom.watchlistInput.value = ''; }
            if (dom.watchlistSuggestions) dom.watchlistSuggestions.hidden = true;
        }
        if (e.key === 'Escape') {
            if (dom.watchlistSuggestions) dom.watchlistSuggestions.hidden = true;
            dom.watchlistInput.blur();
        }
    });
}

document.addEventListener('click', e => {
    if (dom.watchlistSuggestions && !e.target.closest('.watchlist-panel, .watchlist-suggestion')) {
        dom.watchlistSuggestions.hidden = true;
    }
});

// Retry
dom.processRetry?.querySelector('.retry-btn')?.addEventListener('click', async () => {
    dom.processRetry.hidden = true;
    try { dom.processSkeleton.remove(); } catch (e) { /* ok */ }
    await fetchActive();
    renderDashboardProcesses();
});

document.addEventListener('click', async e => {
    const btn = e.target.closest('.error-retry');
    if (!btn) return;
    const key = btn.dataset.key;
    if (key === 'activeProcesses') { await fetchActive(); renderDashboardProcesses(); }
    else if (key === 'activeSessions') { await fetchActiveSessions(); renderDashboardActiveSessions(); }
    else if (key === 'sessions') { await fetchSessions(); renderDashboardSummary(); renderDashboardBuilding(); renderDashboardDonut(); }
    else if (key === 'autoWatch') { await fetchAutoWatch(); renderDashboardWatchlist(); }
    if (location.hash === '#processes') renderProcessesView();
    if (location.hash === '#history') renderHistoryView();
});

// ════════════════════════════════════════
//  HELPERS
// ════════════════════════════════════════

function esc(s) { const d = document.createElement('div'); d.textContent = s == null ? '' : s; return d.innerHTML; }
function hashCode(s) { if (s == null) return 0; let h = 0; for (let i = 0; i < s.length; i++) { h = ((h << 5) - h) + s.charCodeAt(i); h |= 0; } return Math.abs(h); }

function errorState(msg, retryFn) {
    return '<div class="error-state"><span>' + esc(msg) + '</span><button class="retry-btn error-retry" data-key="' + esc(retryFn) + '">Retry</button></div>';
}

let snackbarTimer = null;
function toast(msg, type) {
    if (!dom.snackbar) return;
    if (snackbarTimer) clearTimeout(snackbarTimer);
    dom.snackbar.textContent = msg;
    dom.snackbar.className = 'snackbar ' + (type || '') + ' visible';
    snackbarTimer = setTimeout(() => dom.snackbar.classList.remove('visible'), 3000);
}

// ── Mobile Nav ──
const menuToggle = document.getElementById('menu-toggle');
const navLinks = document.getElementById('nav-links');
if (menuToggle && navLinks) {
    menuToggle.setAttribute('aria-controls', 'nav-links');
    menuToggle.setAttribute('aria-expanded', 'false');
    menuToggle.addEventListener('click', () => {
        const open = navLinks.classList.toggle('open');
        menuToggle.classList.toggle('open');
        menuToggle.setAttribute('aria-expanded', String(open));
        if (open) {
            navLinks.querySelector('a')?.focus();
        } else {
            menuToggle.focus();
        }
    });
    navLinks.addEventListener('click', e => {
        if (e.target.closest('a')) {
            navLinks.classList.remove('open');
            menuToggle.classList.remove('open');
            menuToggle.setAttribute('aria-expanded', 'false');
            menuToggle.focus();
        }
    });
    document.addEventListener('keydown', e => {
        if (e.key === 'Escape' && navLinks.classList.contains('open')) {
            navLinks.classList.remove('open');
            menuToggle.classList.remove('open');
            menuToggle.setAttribute('aria-expanded', 'false');
            menuToggle.focus();
        }
    });
    document.addEventListener('click', e => {
        if (navLinks.classList.contains('open') && !e.target.closest('nav')) {
            navLinks.classList.remove('open');
            menuToggle.classList.remove('open');
            menuToggle.setAttribute('aria-expanded', 'false');
        }
    });
}

// ════════════════════════════════════════
//  INIT
// ════════════════════════════════════════

(async function init() {
    try {
        await fetchAll();
        renderDashboard();
        initRouter();
        startPoll();
    } catch (e) {
        toast('Failed to initialize: ' + e.message, 'error');
    }
})();
