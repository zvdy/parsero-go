// Captures UI screenshots of a running parserod instance for the README.
//
// Usage:
//   BASE_URL=http://localhost:8080 \
//   IDENTITY=demo@parsero.dev \
//   RESULTS_SCAN_ID=<uuid> \
//   OUT_DIR=assets/screenshots \
//   node scripts/screenshots.mjs
//
// Requires Playwright's chromium. The script is environment-driven so the same
// file runs both locally and in CI (see .github/workflows/screenshots.yml).

import { createRequire } from 'node:module';
import { mkdir } from 'node:fs/promises';

// Resolve playwright via CommonJS so a global install (NODE_PATH) works without
// a local node_modules — handy in CI and ad-hoc runs.
const require = createRequire(import.meta.url);
const { chromium } = require('playwright');

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';
const IDENTITY = process.env.IDENTITY || 'demo@parsero.dev';
const IDENTITY_HEADER = process.env.IDENTITY_HEADER || 'X-Auth-Request-Email';
const RESULTS_SCAN_ID = process.env.RESULTS_SCAN_ID || '';
const OUT_DIR = process.env.OUT_DIR || 'assets/screenshots';
// A monitor is created through the UI so the screenshots are reproducible. The
// target must be publicly resolvable (the SSRF guard rejects private hosts).
const MONITOR_TARGET = process.env.MONITOR_TARGET || 'example.com';
const MONITOR_WEBHOOK = process.env.MONITOR_WEBHOOK || 'https://hooks.slack.com/services/T000/B000/XXXX';

const shots = [];

async function main() {
  await mkdir(OUT_DIR, { recursive: true });

  const browser = await chromium.launch();
  const context = await browser.newContext({
    viewport: { width: 1200, height: 900 },
    deviceScaleFactor: 2, // crisp, retina-quality PNGs
    extraHTTPHeaders: { [IDENTITY_HEADER]: IDENTITY },
  });
  const page = await context.newPage();

  await page.goto(`${BASE_URL}/`, { waitUntil: 'networkidle' });
  await page.waitForSelector('.scan-form');

  // Create a recurring monitor through the form so the home page + monitors
  // card show real data. Best-effort: don't fail the run if seeding hiccups.
  await createMonitor(page);

  // 1. Home: scan form, monitors, and recent history.
  await shot(page, 'home.png', false);

  // 2. Focused shot of just the "Recurring monitors" card.
  const monitorsCard = page.locator('section.card', { hasText: 'Recurring monitors' });
  if (await monitorsCard.count()) {
    await monitorsCard.first().screenshot({ path: `${OUT_DIR}/monitors.png` });
    shots.push(`${OUT_DIR}/monitors.png`);
  }

  // 3. Home with an SSRF rejection inline error (a security highlight).
  await page.fill('.scan-form input[name="target"]', 'localhost');
  await page.click('.scan-form button');
  await page.waitForSelector('.error-banner', { timeout: 5000 });
  await shot(page, 'ssrf-blocked.png', false);

  // 4. A completed scan's results page. Clip to a tall-ish viewport rather than
  // fullPage — real scans have hundreds of rows, which would make an unusably
  // long image for the README.
  if (RESULTS_SCAN_ID) {
    await page.setViewportSize({ width: 1200, height: 1150 });
    await page.goto(`${BASE_URL}/scan/${RESULTS_SCAN_ID}`, { waitUntil: 'networkidle' });
    // Wait for the results table the HTMX partial swaps in.
    await page.waitForSelector('table.results tbody tr', { timeout: 15000 });
    await shot(page, 'scan-results.png', false);
  }

  await browser.close();
  console.log('Saved screenshots:\n  ' + shots.join('\n  '));
}

// createMonitor fills and submits the monitors form, then waits for the row.
async function createMonitor(page) {
  const form = 'form[hx-post="/ui/schedules"]';
  try {
    if (await page.locator(`${form} tbody tr`).count()) return; // already seeded
    await page.fill(`${form} input[name="target"]`, MONITOR_TARGET);
    await page.fill(`${form} input[name="cron"]`, '@daily');
    await page.fill(`${form} input[name="notify_webhook"]`, MONITOR_WEBHOOK);
    await page.click(`${form} button[type="submit"]`);
    await page.waitForSelector('#monitors table tbody tr', { timeout: 8000 });
  } catch (err) {
    console.warn('monitor seeding skipped:', err.message);
  }
}

async function shot(page, name, fullPage) {
  const path = `${OUT_DIR}/${name}`;
  await page.screenshot({ path, fullPage });
  shots.push(path);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
