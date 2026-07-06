const API_BASE = 'http://127.0.0.1:8080';
const POLL_INTERVAL = 2000;

let processes = [];
let selectedPid = null;
let focusedIndex = -1;

const searchInput = document.getElementById('search');
const processList = document.getElementById('process-list');
const processCount = document.getElementById('process-count');
const selectedName = document.getElementById('selected-name');
const selectedPidEl = document.getElementById('selected-pid');
const trackBtn = document.getElementById('track-btn');
const snackbar = document.getElementById('snackbar');

let pollTimer = null;
let snackbarTimer = null;

document.addEventListener('keydown', onGlobalKey);

fetchProcesses();
startPolling();

async function fetchProcesses() {
    try {
        const res = await fetch(`${API_BASE}/api/processes`);
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const data = await res.json();
        processes = data.processes || [];
        render();
    } catch (err) {
        showSnackbar('Failed to fetch processes', 'error');
    }
}

function startPolling() {
    stopPolling();
    pollTimer = setInterval(fetchProcesses, POLL_INTERVAL);
}

function stopPolling() {
    if (pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
    }
}

function render() {
    const query = searchInput.value.toLowerCase().trim();
    const filtered = query
        ? processes.filter(p => p.name.toLowerCase().includes(query))
        : processes;

    processCount.textContent = filtered.length;

    if (filtered.length === 0) {
        processList.innerHTML = `
      <div class="empty-state">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <path d="M10 2v4M6 10H2M18 10h-4M14 2v4"/>
          <path d="M6 16a6 6 0 0 1 12 0v4a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2v-4Z"/>
          <circle cx="12" cy="12" r="3"/>
        </svg>
        <p>${query ? 'No matching processes' : 'No processes found'}</p>
      </div>`;
        focusedIndex = -1;
        return;
    }

    focusedIndex = Math.min(focusedIndex, filtered.length - 1);
    if (focusedIndex < 0 && selectedPid) {
        const idx = filtered.findIndex(p => p.pid === selectedPid);
        if (idx >= 0) focusedIndex = idx;
    }

    let html = '';
    for (let i = 0; i < filtered.length; i++) {
        const p = filtered[i];
        const isSelected = p.pid === selectedPid;
        const isFocused = i === focusedIndex;
        const firstLetter = p.name.charAt(0).toUpperCase();

        html += `
      <div class="process-card${isSelected ? ' selected' : ''}"
           role="option"
           aria-selected="${isSelected}"
           tabindex="${isFocused ? 0 : -1}"
           data-index="${i}"
           data-pid="${p.pid}">
        <div class="process-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/>
            <line x1="8" y1="21" x2="16" y2="21"/>
            <line x1="12" y1="17" x2="12" y2="21"/>
          </svg>
        </div>
        <div class="process-info">
          <div class="process-name">${escapeHtml(p.name)}</div>
          <div class="process-meta">
            <span>PID <span class="pid-value">${p.pid}</span></span>
            <span>PPID ${p.ppid}</span>
          </div>
        </div>
      </div>`;
    }

    processList.innerHTML = html;

    const cards = processList.querySelectorAll('.process-card');
    cards.forEach(card => {
        card.addEventListener('click', () => {
            const pid = parseInt(card.dataset.pid, 10);
            selectProcess(pid);
        });
        card.addEventListener('mouseenter', () => {
            const idx = parseInt(card.dataset.index, 10);
            focusedIndex = idx;
            updateFocus();
        });
    });

    if (focusedIndex >= 0 && focusedIndex < cards.length) {
        cards[focusedIndex].focus({ preventScroll: false });
    }

    const matchedPid = selectedPid && filtered.some(p => p.pid === selectedPid);
    if (!matchedPid) {
        clearSelection();
    }
}

function selectProcess(pid) {
    selectedPid = pid;
    const proc = processes.find(p => p.pid === pid);
    if (proc) {
        selectedName.textContent = proc.name;
        selectedPidEl.textContent = `PID ${proc.pid}`;
        trackBtn.disabled = false;
        saveRecentSelection(pid);
    }
    const idx = getFilteredList().findIndex(p => p.pid === pid);
    if (idx >= 0) focusedIndex = idx;
    render();
}

function clearSelection() {
    selectedPid = null;
    selectedName.textContent = '—';
    selectedPidEl.textContent = '';
    trackBtn.disabled = true;
}

function getFilteredList() {
    const query = searchInput.value.toLowerCase().trim();
    return query
        ? processes.filter(p => p.name.toLowerCase().includes(query))
        : processes;
}

function updateFocus() {
    const cards = processList.querySelectorAll('.process-card');
    cards.forEach((card, i) => {
        card.tabIndex = i === focusedIndex ? 0 : -1;
        if (i === focusedIndex) card.focus({ preventScroll: false });
    });
}

async function trackProcess() {
    if (!selectedPid || trackBtn.disabled) return;

    trackBtn.classList.add('loading');
    trackBtn.textContent = 'Tracking...';

    try {
        const res = await fetch(`${API_BASE}/api/track`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ pid: selectedPid }),
        });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        showSnackbar(`Now tracking PID ${selectedPid}`, 'success');
    } catch (err) {
        showSnackbar('Failed to start tracking', 'error');
    } finally {
        trackBtn.classList.remove('loading');
        trackBtn.innerHTML = `
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3"/>
      </svg>
      Track This`;
    }
}

function showSnackbar(message, type) {
    if (snackbarTimer) clearTimeout(snackbarTimer);
    snackbar.textContent = message;
    snackbar.className = `snackbar ${type} visible`;
    snackbarTimer = setTimeout(() => {
        snackbar.classList.remove('visible');
    }, 3000);
}

function onGlobalKey(e) {
    if (e.key === '/' && document.activeElement !== searchInput) {
        e.preventDefault();
        searchInput.focus();
        return;
    }

    if (e.key === 'Escape' && document.activeElement === searchInput) {
        searchInput.value = '';
        searchInput.blur();
        render();
        return;
    }

    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        const list = getFilteredList();
        if (list.length === 0) return;

        if (e.key === 'ArrowDown') {
            focusedIndex = (focusedIndex + 1) % list.length;
        } else {
            focusedIndex = (focusedIndex - 1 + list.length) % list.length;
        }

        const pid = list[focusedIndex].pid;
        selectProcess(pid);
        return;
    }

    if (e.key === 'Enter') {
        if (document.activeElement?.classList.contains('process-card')) {
            const pid = parseInt(document.activeElement.dataset.pid, 10);
            selectProcess(pid);
            return;
        }
        if (focusedIndex >= 0 && !trackBtn.disabled && document.activeElement !== searchInput) {
            trackProcess();
        }
        return;
    }
}

searchInput.addEventListener('input', () => {
    focusedIndex = -1;
    render();
});

searchInput.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        const list = getFilteredList();
        if (list.length === 0) return;

        if (e.key === 'ArrowDown') {
            focusedIndex = 0;
        } else {
            focusedIndex = list.length - 1;
        }

        const pid = list[focusedIndex].pid;
        selectProcess(pid);
        searchInput.blur();
    }
});

trackBtn.addEventListener('click', trackProcess);

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function saveRecentSelection(pid) {
    try {
        localStorage.setItem('trkr_last_pid', String(pid));
    } catch { }
}

function restoreSelection() {
    try {
        const last = localStorage.getItem('trkr_last_pid');
        if (last && processes.some(p => p.pid === parseInt(last, 10))) {
            selectProcess(parseInt(last, 10));
        }
    } catch { }
}

const origRender = render;
render = function () {
    origRender();
    if (!selectedPid) restoreSelection();
};
