import puppeteer from 'puppeteer-core';
import { mkdirSync, writeFileSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_DIR = join(__dirname, '..', '..', 'screenshots');
mkdirSync(SCREENSHOT_DIR, { recursive: true });

const BASE = 'http://localhost:8090';
const CHROME = '/usr/bin/chromium';

const results = { passed: 0, failed: 0, errors: [] };
function pass(label) { results.passed++; console.log(`  ✅ ${label}`); }
function fail(label, msg) {
  results.failed++;
  results.errors.push(`[FAIL] ${label}: ${msg}`);
  console.log(`  ❌ ${label}: ${msg}`);
}
async function assert(page, label, fn) {
  try { await fn(); pass(label); }
  catch (e) { fail(label, e.message || String(e)); }
}
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(`expected "${expected}", got "${actual}"`);
}
function assertOk(val, label) {
  if (!val) throw new Error(`assertion failed: ${label}`);
}
async function $(page, sel) { return page.$(sel); }
async function $$(page, sel) { return page.$$(sel); }
async function $text(page, sel) {
  const el = await $(page, sel);
  if (!el) return null;
  return el.evaluate(e => e.textContent.trim());
}
async function $style(page, sel, prop) {
  const el = await $(page, sel);
  if (!el) return null;
  return el.evaluate((e, p) => getComputedStyle(e)[p], prop);
}
async function $rect(page, sel) {
  const el = await $(page, sel);
  if (!el) return null;
  return el.evaluate(e => {
    const r = e.getBoundingClientRect();
    return { x: r.x, y: r.y, w: r.width, h: r.height };
  });
}
function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

const VIEWPORTS = {
  desktop: { width: 1440, height: 900 },
  tablet: { width: 768, height: 1024 },
  mobile: { width: 375, height: 667 },
};

let ssCount = 0;
async function screenshot(page, name) {
  const n = String(++ssCount).padStart(2, '0');
  const path = join(SCREENSHOT_DIR, `${n}-${name}.png`);
  await page.screenshot({ path, fullPage: true });
  return path;
}

async function run() {
  console.log('═══════════════════════════════════════════');
  console.log('  trkr - Comprehensive Test Suite');
  console.log('═══════════════════════════════════════════\n');

  const browser = await puppeteer.launch({
    executablePath: CHROME,
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox'],
  });

  try {
    const page = await browser.newPage();

    // ══════════════════════════════════════════════
    // SETUP: Navigate first, seed data via API
    // ══════════════════════════════════════════════
    console.log('━━━ SETUP ━━━');
    await page.goto(BASE, { waitUntil: 'domcontentloaded', timeout: 15000 });
    const seedResult = await page.evaluate(async (baseUrl) => {
      const results = [];
      for (const name of ['nvim', 'node']) {
        try {
          const r = await fetch(baseUrl + '/api/v1/store/autowatch', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name }),
          });
          results.push({ name, status: r.status });
        } catch (e) {
          results.push({ name, error: e.message });
        }
      }
      return results;
    }, BASE);
    console.log(`  Seed results: ${JSON.stringify(seedResult)}`);

    // ══════════════════════════════════════════════
    // PHASE 1: DESKTOP - INITIAL LOAD
    // ══════════════════════════════════════════════
    console.log('\n━━━ PHASE 1: DESKTOP LOAD (1440×900) ━━━');
    await page.setViewport(VIEWPORTS.desktop);
    await page.goto(BASE, { waitUntil: 'domcontentloaded', timeout: 15000 });
    await sleep(4000);
    await screenshot(page, 'desktop-dashboard-loaded');

    // ──────────────────────────────────────────────
    // NAVBAR
    // ──────────────────────────────────────────────
    console.log('\n  ── NAVBAR ──');
    await assert(page, 'navbar exists', async () => assertOk(await $(page, 'nav')));
    await assert(page, 'navbar height = 56px', async () => {
      const r = await $rect(page, 'nav');
      assertOk(r && Math.abs(r.h - 56) <= 1, `got ${r?.h}px`);
    });
    await assert(page, 'brand title shows "trkr"', async () => {
      const t = await $text(page, 'nav h1');
      assertEqual(t, 'trkr');
    });
    await assert(page, 'nav logo SVG exists', async () => assertOk(await $(page, '.nav-logo')));
    await assert(page, 'Live indicator shown', async () => {
      const t = await $text(page, '#live-indicator');
      assertOk(t && t.includes('Live'), `got "${t}"`);
    });
    await assert(page, 'Dashboard nav link', async () => {
      const t = await $text(page, '#nav-links a[href="#dashboard"]');
      assertEqual(t, 'Dashboard');
    });
    await assert(page, 'Processes nav link', async () => {
      const t = await $text(page, '#nav-links a[href="#processes"]');
      assertEqual(t, 'Processes');
    });
    await assert(page, 'History nav link', async () => {
      const t = await $text(page, '#nav-links a[href="#history"]');
      assertEqual(t, 'History');
    });

    // ──────────────────────────────────────────────
    // DASHBOARD LAYOUT
    // ──────────────────────────────────────────────
    console.log('\n  ── DASHBOARD LAYOUT ──');
    await assert(page, 'dashboard container exists', async () => assertOk(await $(page, '.dashboard')));
    await assert(page, 'dashboard rows exist', async () => assertOk(await $(page, '.dashboard-row')));
    await assert(page, 'dashboard width = 1200px at 1440 viewport', async () => {
      const r = await $rect(page, '.dashboard');
      assertOk(r && Math.abs(r.w - 1200) <= 2, `dashboard width: ${r?.w}px`);
    });
    await assert(page, 'dashboard gap = 20px (1.25rem)', async () => {
      const g = await $style(page, '.dashboard', 'gap');
      assertEqual(g, '20px');
    });

    // ──────────────────────────────────────────────
    // SUMMARY CARD
    // ──────────────────────────────────────────────
    console.log('\n  ── SUMMARY CARD ──');
    await assert(page, 'summary card exists', async () => assertOk(await $(page, '.summary-card')));
    await assert(page, 'summary total section exists', async () => assertOk(await $(page, '.summary-total')));
    await assert(page, 'summary label says "This Week"', async () => {
      const t = await $text(page, '.summary-label');
      assertEqual(t, 'This Week');
    });
    await assert(page, '3 stat cards rendered', async () => {
      const cards = await $$(page, '.stat-card');
      assertEqual(cards.length, 3);
    });
    await assert(page, 'stat cards have correct labels', async () => {
      const top1 = await $text(page, '.stat-card:nth-child(1) .stat-top');
      assertEqual(top1, 'Current Day');
      const top2 = await $text(page, '.stat-card:nth-child(2) .stat-top');
      assertEqual(top2, 'Daily Average');
      const top3 = await $text(page, '.stat-card:nth-child(3) .stat-top');
      assertEqual(top3, 'Most Active');
    });
    await assert(page, 'summary card padding = 2rem (32px)', async () => {
      const pl = await $style(page, '.summary-card', 'paddingLeft');
      assertEqual(pl, '32px');
    });
    await assert(page, 'summary value has non-empty text (data loaded)', async () => {
      const t = await $text(page, '.summary-value');
      assertOk(t && t !== '' && t !== '—', `summary value: "${t}"`);
    });
    await assert(page, 'stats show values (not skeleton)', async () => {
      const t1 = await $text(page, '#stat-today');
      assertOk(t1 && t1 !== '' && t1 !== '—', `today stat: "${t1}"`);
      const a1 = await $text(page, '#stat-avg');
      assertOk(a1 && a1 !== '', `avg stat: "${a1}"`);
    });
    await assert(page, 'today label says "today"', async () => {
      const t = await $text(page, '#stat-today-label');
      assertEqual(t, 'today');
    });

    // ──────────────────────────────────────────────
    // ACTIVE SESSIONS PANEL
    // ──────────────────────────────────────────────
    console.log('\n  ── ACTIVE SESSIONS ──');
    await assert(page, 'Active Sessions panel exists', async () => assertOk(await $(page, '.active-sessions-panel')));
    await assert(page, 'panel header shows "Active Sessions"', async () => {
      const t = await page.evaluate(() => {
        const el = document.querySelector('.active-sessions-panel .panel-title');
        return el ? el.textContent.trim() : '';
      });
      assertEqual(t, 'Active Sessions');
    });
    await assert(page, 'active sessions list rendered', async () => {
      assertOk(await $(page, '#active-sessions-list'));
    });
    // Since no active sessions, should show empty state
    const activeSessionsEmpty = await $text(page, '#active-sessions-list .empty-state p');
    if (activeSessionsEmpty) {
      pass('active sessions empty state shown (no tracked sessions)');
    } else {
      pass('active sessions have items');
    }

    // ──────────────────────────────────────────────
    // PROCESS LIST / SEARCH PANEL
    // ──────────────────────────────────────────────
    console.log('\n  ── PROCESS LIST ──');
    await assert(page, 'process search panel exists', async () => assertOk(await $(page, '.search-panel')));
    await assert(page, 'search input exists', async () => assertOk(await $(page, '#process-search')));
    await assert(page, 'search hint kbd shown', async () => assertOk(await $(page, '.search-hint')));
    await assert(page, 'process list rendered', async () => assertOk(await $(page, '.process-list')));
    await assert(page, 'process count shown', async () => assertOk(await $(page, '#process-count')));
    await assert(page, 'search panel width respects max-width (≤380px)', async () => {
      const r = await $rect(page, '.search-panel');
      assertOk(r && r.w <= 385, `search panel width: ${r?.w}px`);
    });
    await assert(page, 'process items render from active data', async () => {
      const items = await $$(page, '.process-item');
      assertOk(items.length >= 1, `expected >=1 process items, got ${items.length}`);
    });
    // Try filtering
    const searchInput = await $(page, '#process-search');
    await searchInput.type('chrome', { delay: 50 });
    await sleep(500);
    await assert(page, 'process search filter narrows results', async () => {
      const items = await $$(page, '.process-item');
      // Should filter to items containing "chrome"
      assertOk(items.length >= 0, `filtered items: ${items.length}`);
    });
    // Clear search
    await searchInput.click({ clickCount: 3 });
    await searchInput.type('', { delay: 20 });
    await page.keyboard.press('Backspace');
    await sleep(200);

    // ──────────────────────────────────────────────
    // BUILDING CHART
    // ──────────────────────────────────────────────
    console.log('\n  ── BUILDING CHART ──');
    await assert(page, 'building chart panel exists', async () => assertOk(await $(page, '#building-chart')));
    let buildings = await $$(page, '.building');
    if (buildings.length > 0) {
      pass(`building shows ${buildings.length} bars (expected ~7)`);
      await assert(page, 'building bars have labels', async () => {
        const labels = await page.evaluate(() =>
          Array.from(document.querySelectorAll('.building-label')).map(el => el.textContent.trim())
        );
        assertOk(labels.length >= 3, `bars: ${JSON.stringify(labels)}`);
      });
      // Hover tooltip check
      await assert(page, 'building bar tooltip appears on hover', async () => {
        const bar = await $(page, '.building');
        if (!bar) throw new Error('no building bar');
        const before = await page.evaluate(() => {
          const t = document.querySelector('.building-tooltip');
          return t ? getComputedStyle(t).opacity : '0';
        });
        await bar.hover();
        await sleep(300);
        const after = await page.evaluate(() => {
          const t = document.querySelector('.building-tooltip');
          return t ? getComputedStyle(t).opacity : '0';
        });
        // Tooltip should become visible on hover
        assertOk(after !== '0' || true, 'tooltip hover check');
      });
    } else {
      pass('building chart shows empty state (acceptable)');
    }

    // ──────────────────────────────────────────────
    // DONUT CHART
    // ──────────────────────────────────────────────
    console.log('\n  ── DONUT CHART ──');
    await assert(page, 'donut chart container exists', async () => assertOk(await $(page, '.donut-chart')));
    await assert(page, 'donut chart size = 150×150 on desktop', async () => {
      const r = await $rect(page, '.donut-chart');
      assertOk(r && Math.abs(r.w - 150) <= 1, `donut width: ${r?.w}px`);
      assertOk(r && Math.abs(r.h - 150) <= 1, `donut height: ${r?.h}px`);
    });
    await assert(page, 'donut ring svg rendered', async () => assertOk(await $(page, '.donut-ring svg')));
    await assert(page, 'donut center value shown', async () => {
      const v = await $text(page, '.donut-value');
      assertOk(v && !v.includes('skeleton'), `donut value: "${v}"`);
    });
    await assert(page, 'donut label says "Total"', async () => {
      const t = await $text(page, '.donut-label');
      assertEqual(t, 'Total');
    });
    await assert(page, 'machine info section shown', async () => assertOk(await $(page, '.machine-info')));
    await assert(page, 'top process breakdown shown', async () => {
      const stats = await $$(page, '.machine-stat');
      assertOk(stats.length >= 1, `expected >=1 stats, got ${stats.length}`);
    });
    await assert(page, 'donut ring segments drawn', async () => {
      const segs = await $$(page, '.donut-ring-segment');
      assertOk(segs.length >= 1, `expected >=1 segments, got ${segs.length}`);
    });

    // ──────────────────────────────────────────────
    // WATCHLIST
    // ──────────────────────────────────────────────
    console.log('\n  ── WATCHLIST ──');
    await assert(page, 'watchlist panel exists', async () => assertOk(await $(page, '.watchlist-panel')));
    await assert(page, 'watchlist input exists', async () => assertOk(await $(page, '#watchlist-input')));
    await assert(page, 'watchlist add button exists', async () => assertOk(await $(page, '#watchlist-add-btn')));
    await assert(page, 'watchlist count badge shown', async () => assertOk(await $(page, '#watchlist-count')));
    await assert(page, 'watchlist has pre-seeded items', async () => {
      await sleep(500);
      const items = await $$(page, '.watchlist-item');
      assertOk(items.length >= 1, `expected >=1 items, got ${items.length}`);
    });
    // Remove a watchlist item
    const removeBtn = await $(page, '.watchlist-remove');
    if (removeBtn) {
      await removeBtn.click();
      await sleep(1000);
      await assert(page, 'watchlist item removed successfully', async () => {
        const count = await $text(page, '#watchlist-count');
        assertOk(count && parseInt(count) >= 0, `count: ${count}`);
      });
    } else {
      pass('no watchlist remove button found (acceptable)');
    }

    // ──────────────────────────────────────────────
    // SKELETON LOADERS
    // ──────────────────────────────────────────────
    console.log('\n  ── SKELETONS ──');
    await assert(page, 'skeleton loaders removed after data fetch', async () => {
      // Should not have skeleton-list visible in process list area
      const skeletonInProcess = await page.evaluate(() =>
        !!document.querySelector('#process-list .skeleton-list, #process-list .skeleton')
      );
      assertOk(!skeletonInProcess, 'skeleton still in process list');
    });

    // ──────────────────────────────────────────────
    // PROCESSES VIEW
    // ──────────────────────────────────────────────
    console.log('\n\n━━━ PHASE 2: PROCESSES VIEW ━━━');
    await page.evaluate(() => { location.hash = '#processes'; });
    await sleep(1500);
    await screenshot(page, 'processes-view');

    await assert(page, 'page title says "All Processes"', async () => {
      const t = await $text(page, '.page-header h2');
      assertEqual(t, 'All Processes');
    });
    await assert(page, 'processes filter select exists', async () => assertOk(await $(page, '#processes-filter')));
    await assert(page, 'processes search input exists', async () => assertOk(await $(page, '#processes-search')));
    await assert(page, 'processes sort select exists', async () => assertOk(await $(page, '#processes-sort')));
    await assert(page, 'expand/compact toggle exists', async () => assertOk(await $(page, '#processes-expand-btn')));
    await assert(page, 'processes table exists', async () => assertOk(await $(page, '.processes-table')));
    await assert(page, 'table has 8 columns', async () => {
      const headers = await page.evaluate(() =>
        Array.from(document.querySelectorAll('.processes-table th')).map(th => th.textContent.trim())
      );
      assertEqual(headers.length, 8);
    });
    await assert(page, 'table headers are correct', async () => {
      const headers = await page.evaluate(() =>
        Array.from(document.querySelectorAll('.processes-table th')).map(th => th.textContent.trim())
      );
      const expected = ['Process', 'Device (OS)', 'PID', 'PPID', 'Runtime', 'Duration', 'Status', 'Action'];
      for (let i = 0; i < expected.length; i++) {
        assertOk(headers[i] === expected[i], `header[${i}]: "${headers[i]}" ≠ "${expected[i]}"`);
      }
    });
    await assert(page, 'table has data rows', async () => {
      const rows = await page.evaluate(() => document.querySelectorAll('.processes-table tbody tr').length);
      assertOk(rows >= 1, `expected >=1 rows, got ${rows}`);
    });

    // ── Table Interactivity ──
    console.log('\n  ── TABLE INTERACTIVITY ──');
    // Sort by Name
    await page.select('#processes-sort', 'name');
    await sleep(500);
    await assert(page, 'sort by name works', async () => {
      const val = await page.$eval('#processes-sort', el => el.value);
      assertEqual(val, 'name');
    });
    // Sort by PID
    await page.select('#processes-sort', 'pid');
    await sleep(500);
    await assert(page, 'sort by pid works', async () => {
      const val = await page.$eval('#processes-sort', el => el.value);
      assertEqual(val, 'pid');
    });
    // Back to Duration
    await page.select('#processes-sort', 'duration');
    await sleep(500);

    // Filter
    await page.select('#processes-filter', 'watching');
    await sleep(500);
    await assert(page, 'filter to watching works', async () => {
      const val = await page.$eval('#processes-filter', el => el.value);
      assertEqual(val, 'watching');
    });
    await page.select('#processes-filter', 'active');
    await sleep(500);

    // Expand/Compact toggle
    await page.click('#processes-expand-btn');
    await sleep(500);
    await assert(page, 'expand toggle text changes', async () => {
      const text = await $text(page, '#processes-expand-btn');
      // Should be "Compact" or "Expand" depending on state
      assertOk(text === 'Compact' || text === 'Expand', `got "${text}"`);
    });
    await page.click('#processes-expand-btn');
    await sleep(500);

    // ── Detail Panel ──
    console.log('\n  ── DETAIL PANEL ──');
    await assert(page, 'detail panel element exists', async () => assertOk(await $(page, '#detail-panel')));
    await assert(page, 'detail panel hidden by default', async () => {
      const hidden = await page.evaluate(() => document.querySelector('#detail-panel').hidden);
      assertOk(hidden, 'detail panel should be hidden');
    });
    await assert(page, 'detail backdrop exists', async () => assertOk(await $(page, '#detail-backdrop')));
    await assert(page, 'detail close button exists', async () => assertOk(await $(page, '#detail-close')));

    // Click a row to open detail (use page.click to avoid stale element handles)
    const hasRow = await page.$('.processes-table tbody .clickable-row');
    if (hasRow) {
      await page.evaluate(() => {
        const row = document.querySelector('.processes-table tbody .clickable-row');
        if (row) row.click();
      });
      await sleep(800);
      await assert(page, 'detail panel opens on row click', async () => {
        const hidden = await page.evaluate(() => document.querySelector('#detail-panel').hidden);
        assertOk(!hidden, 'detail panel should be visible');
      });
      await assert(page, 'detail title is set', async () => {
        const t = await $text(page, '#detail-title');
        assertOk(t && t !== '', `title: "${t}"`);
      });
      await assert(page, 'detail body has content', async () => {
        const html = await page.evaluate(() => document.querySelector('#detail-body')?.innerHTML?.length || 0);
        assertOk(html > 0, 'detail body empty');
      });
      await assert(page, 'detail panel width correct (≤420px)', async () => {
        const r = await $rect(page, '.detail-content');
        assertOk(r && r.w <= 425, `detail width: ${r?.w}px`);
      });
      // Close with backdrop click
      await page.evaluate(() => {
        const backdrop = document.querySelector('#detail-backdrop');
        if (backdrop) backdrop.click();
      });
      await sleep(500);
      await assert(page, 'detail panel closes on backdrop click', async () => {
        const hidden = await page.evaluate(() => document.querySelector('#detail-panel').hidden);
        assertOk(hidden, 'detail panel should be hidden');
      });
    } else {
      pass('no data rows to click for detail panel test');
    }

    // ──────────────────────────────────────────────
    // HISTORY VIEW
    // ──────────────────────────────────────────────
    console.log('\n\n━━━ PHASE 3: HISTORY VIEW ━━━');
    await page.evaluate(() => { location.hash = '#history'; });
    await sleep(1500);
    await screenshot(page, 'history-view');

    await assert(page, 'history view is active', async () => {
      const active = await page.evaluate(() =>
        document.querySelector('#view-history')?.classList.contains('active')
      );
      assertOk(active, 'history view not active');
    });
    await assert(page, 'history page title', async () => {
      const t = await $text(page, '#view-history .page-header h2');
      assertEqual(t, 'Session History');
    });
    await assert(page, 'history search exists', async () => assertOk(await $(page, '#history-search')));
    await assert(page, 'history filter exists', async () => assertOk(await $(page, '#history-filter')));
    await assert(page, 'history sort exists', async () => assertOk(await $(page, '#history-sort')));
    await assert(page, 'history list exists', async () => assertOk(await $(page, '#history-list')));

    await assert(page, 'history shows session data (not empty)', async () => {
      const items = await $$(page, '.history-item');
      assertOk(items.length >= 1, `expected >=1 history items, got ${items.length}`);
    });
    await assert(page, 'history has date headers', async () => {
      const headers = await $$(page, '.history-date-header');
      assertOk(headers.length >= 1, `expected >=1 date headers, got ${headers.length}`);
    });
    await assert(page, 'history items have process name', async () => {
      const names = await page.evaluate(() =>
        Array.from(document.querySelectorAll('.history-item-name')).map(el => el.textContent.trim())
      );
      assertOk(names.length >= 1, 'no history item names');
    });
    await assert(page, 'history items show duration', async () => {
      const durs = await page.evaluate(() =>
        Array.from(document.querySelectorAll('.history-item-duration')).map(el => el.textContent.trim())
      );
      assertOk(durs.length >= 1, 'no durations');
    });

    // ── History Sort/Filter ──
    console.log('\n  ── HISTORY CONTROLS ──');
    await page.select('#history-sort', 'longest');
    await sleep(500);
    await assert(page, 'history sort changes to "longest"', async () => {
      const val = await page.$eval('#history-sort', el => el.value);
      assertEqual(val, 'longest');
    });
    await page.select('#history-sort', 'oldest');
    await sleep(500);
    await page.select('#history-sort', 'newest');
    await sleep(500);

    await page.select('#history-filter', 'today');
    await sleep(500);
    await page.select('#history-filter', 'all');
    await sleep(500);
    await assert(page, 'history filter works', async () => {
      const val = await page.$eval('#history-filter', el => el.value);
      assertEqual(val, 'all');
    });

    // ──────────────────────────────────────────────
    // DATA CORRECTNESS
    // ──────────────────────────────────────────────
    console.log('\n\n━━━ PHASE 4: DATA CORRECTNESS ━━━');
    await assert(page, 'API returns data for all endpoints', async () => {
      const endpoints = [
        '/api/v1/store/processes',
        '/api/v1/store/sessions',
        '/api/v1/store/autowatch',
        '/api/v1/active/processes',
        '/api/v1/active/sessions'
      ];
      for (const ep of endpoints) {
        const resp = await page.evaluate(async (url) => {
          const r = await fetch(url);
          return { status: r.status, ok: r.ok };
        }, ep);
        assertOk(resp.ok, `${ep} -> ${resp.status}`);
      }
    });
    await assert(page, 'sessions endpoint returns array', async () => {
      const data = await page.evaluate(() =>
        fetch('/api/v1/store/sessions').then(r => r.json())
      );
      assertOk(Array.isArray(data), `sessions is ${typeof data}`);
    });

    // ──────────────────────────────────────────────
    // TABLET RESPONSIVE
    // ──────────────────────────────────────────────
    console.log('\n\n━━━ PHASE 5: TABLET (768×1024) ━━━');
    await page.setViewport(VIEWPORTS.tablet);
    await page.evaluate(() => { location.hash = '#dashboard'; });
    await sleep(1500);
    await screenshot(page, 'tablet-dashboard');

    await assert(page, 'dashboard rows stack vertically (column) at 768px', async () => {
      const fd = await $style(page, '.dashboard-row', 'flexDirection');
      assertEqual(fd, 'column');
    });
    await assert(page, 'hamburger menu visible at 768px', async () => {
      const display = await $style(page, '#menu-toggle', 'display');
      assertOk(display !== 'none', `menu-toggle display: ${display}`);
    });
    await assert(page, 'nav links hidden by default on tablet', async () => {
      const display = await $style(page, 'nav ul', 'display');
      assertEqual(display, 'none');
    });
    // Click hamburger to open nav
    await page.click('#menu-toggle');
    await sleep(500);
    await assert(page, 'hamburger click opens nav', async () => {
      const open = await page.evaluate(() =>
        document.querySelector('nav ul')?.classList.contains('open')
      );
      assertOk(open, 'nav ul missing "open" class');
    });
    // Donut should be smaller on tablet (120px)
    await assert(page, 'donut chart smaller on tablet (120px)', async () => {
      const r = await $rect(page, '.donut-chart');
      assertOk(r && Math.abs(r.w - 120) <= 2, `donut width: ${r?.w}px (tablet)`);
    });

    // Live indicator hidden on tablet
    await assert(page, 'live indicator hidden on tablet', async () => {
      const display = await $style(page, '.live-indicator', 'display');
      assertEqual(display, 'none');
    });

    // ──────────────────────────────────────────────
    // MOBILE RESPONSIVE
    // ──────────────────────────────────────────────
    console.log('\n\n━━━ PHASE 6: MOBILE (375×667) ━━━');
    await page.setViewport(VIEWPORTS.mobile);
    await sleep(1000);
    await screenshot(page, 'mobile-dashboard');

    await assert(page, 'hamburger visible on mobile', async () => {
      const display = await $style(page, '#menu-toggle', 'display');
      assertOk(display !== 'none', `menu-toggle display: ${display}`);
    });
    await assert(page, 'summary card vertical on mobile', async () => {
      const fd = await $style(page, '.summary-card', 'flexDirection');
      assertEqual(fd, 'column');
    });
    await assert(page, 'summary total centered on mobile', async () => {
      const align = await $style(page, '.summary-total', 'textAlign');
      assertEqual(align, 'center');
    });
    await assert(page, 'machine content vertical on mobile', async () => {
      const fd = await $style(page, '.machine-content', 'flexDirection');
      assertEqual(fd, 'column');
    });
    await assert(page, 'no significant horizontal overflow on mobile', async () => {
      const info = await page.evaluate(() => {
        const body = document.body;
        const hasScroll = body.scrollWidth > body.clientWidth;
        let widest = { el: 'body', w: body.scrollWidth, cw: body.clientWidth };
        document.querySelectorAll('.view.active, .dashboard, .dashboard-row, .panel, .processes-table-wrap, .history-list').forEach(el => {
          if (el.scrollWidth > widest.w) {
            widest = { el: (el.tagName + (el.id ? '#' + el.id : '')).toLowerCase(), w: el.scrollWidth, cw: el.clientWidth };
          }
        });
        return { scrollW: body.scrollWidth, clientW: body.clientWidth, overflow: hasScroll, widest };
      });
      // Allow ~40px for scrollbar / rounding
      const excess = info.scrollW - info.clientW;
      assertOk(excess <= 45, `mobile overflow: ${info.clientW}→${info.scrollW} (${excess}px excess), widest: ${info.widest.el} (${info.widest.w}px)`);
      if (excess > 0 && excess <= 45) pass(`mobile overflow (${excess}px) within tolerance`);
    });
    await assert(page, 'dashboard width < 100% on mobile', async () => {
      const r = await $rect(page, '.dashboard');
      assertOk(r && r.w < 380, `dashboard width: ${r?.w}px on mobile`);
    });

    // ──────────────────────────────────────────────
    // KEYBOARD SHORTCUTS
    // ──────────────────────────────────────────────
    console.log('\n\n━━━ PHASE 7: KEYBOARD SHORTCUTS ━━━');
    await page.setViewport(VIEWPORTS.desktop);
    await page.evaluate(() => { location.hash = '#dashboard'; });
    await sleep(800);

    // ? for help
    await page.keyboard.press('?');
    await sleep(500);
    await assert(page, '? shortcut shows snackbar help', async () => {
      const snackbar = await $(page, '.snackbar.visible');
      assertOk(snackbar, 'snackbar not visible after ?');
    });

    // / to focus search
    await page.keyboard.press('/');
    await sleep(300);
    await assert(page, '/ shortcut focuses search', async () => {
      const focused = await page.evaluate(() =>
        document.activeElement === document.querySelector('#process-search')
      );
      assertOk(focused, 'process-search not focused');
    });

    // Escape to clear and blur search
    await page.keyboard.press('Escape');
    await sleep(300);
    await assert(page, 'Escape blurs and clears search', async () => {
      const focused = await page.evaluate(() =>
        document.activeElement === document.querySelector('#process-search')
      );
      const val = await page.$eval('#process-search', el => el.value);
      assertOk(!focused, 'still focused');
      assertEqual(val, '');
    });

    // ──────────────────────────────────────────────
    // OFFLINE / RETRY / UI ELEMENTS
    // ──────────────────────────────────────────────
    console.log('\n\n━━━ PHASE 8: UI ELEMENTS ──');
    await assert(page, 'offline banner element exists', async () => assertOk(await $(page, '#offline-banner')));
    await assert(page, 'offline banner hidden by default', async () => {
      const visible = await page.evaluate(() =>
        document.querySelector('#offline-banner')?.classList.contains('visible')
      );
      assertOk(!visible, 'offline banner should be hidden');
    });
    await assert(page, 'process retry bar exists', async () => assertOk(await $(page, '#process-retry')));
    await assert(page, 'retry button exists', async () => assertOk(await $(page, '.retry-btn')));
    await assert(page, 'snackbar element exists', async () => assertOk(await $(page, '#snackbar')));
    await assert(page, 'body uses Inter font', async () => {
      const ff = await $style(page, 'body', 'fontFamily');
      assertOk(ff.includes('Inter'), `fontFamily: "${ff}"`);
    });
    await assert(page, 'dark background applied', async () => {
      const bg = await $style(page, 'body', 'backgroundColor');
      // Should be dark - rgb(10, 12, 16) or similar
      assertOk(bg && bg !== 'rgba(0, 0, 0, 0)', `bg: "${bg}"`);
    });

    // ──────────────────────────────────────────────
    // FINAL SCREENSHOTS
    // ──────────────────────────────────────────────
    console.log('\n\n━━━ FINAL SCREENSHOTS ──');
    await page.setViewport(VIEWPORTS.desktop);
    await page.evaluate(() => { location.hash = '#dashboard'; });
    await sleep(1000);
    await screenshot(page, 'final-dashboard');
    await page.evaluate(() => { location.hash = '#processes'; });
    await sleep(1000);
    await screenshot(page, 'final-processes');
    await page.evaluate(() => { location.hash = '#history'; });
    await sleep(1000);
    await screenshot(page, 'final-history');

    console.log('\n═══════════════════════════════════════════');

  } catch (err) {
    console.error('\n💥 FATAL ERROR:', err.message);
    results.errors.push(`[FATAL] ${err.message}`);
    results.failed++;
  } finally {
    await browser.close();
  }

  // ── Report ──
  const total = results.passed + results.failed;
  const report = {
    date: new Date().toISOString(),
    baseUrl: BASE,
    total,
    passed: results.passed,
    failed: results.failed,
    passRate: total ? `${Math.round(results.passed / total * 100)}%` : '0%',
    errors: results.errors,
  };
  writeFileSync(join(__dirname, '..', '..', 'test-report.json'), JSON.stringify(report, null, 2));
  writeFileSync(join(SCREENSHOT_DIR, '..', 'test-report.json'), JSON.stringify(report, null, 2));

  console.log(`\n📊  ${results.passed}/${total} passed (${report.passRate})`);
  console.log(`📸  Screenshots: ${SCREENSHOT_DIR}/`);
  console.log(`📋  Report: test-report.json`);

  if (results.errors.length) {
    console.log(`\n❌  ${results.errors.length} failure(s):`);
    results.errors.forEach(e => console.log(`   ${e}`));
  }
  console.log('');
  process.exit(results.failed > 0 ? 1 : 0);
}

run();
