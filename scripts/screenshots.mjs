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

  // 1. Home: scan form + recent history.
  await page.goto(`${BASE_URL}/`, { waitUntil: 'networkidle' });
  await page.waitForSelector('.scan-form');
  await shot(page, 'home.png', false);

  // 2. Home with an SSRF rejection inline error (a security highlight).
  await page.fill('input[name="target"]', 'localhost');
  await page.click('.scan-form button');
  await page.waitForSelector('.error-banner', { timeout: 5000 });
  await shot(page, 'ssrf-blocked.png', false);

  // 3. A completed scan's results page. Clip to a tall-ish viewport rather than
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

async function shot(page, name, fullPage) {
  const path = `${OUT_DIR}/${name}`;
  await page.screenshot({ path, fullPage });
  shots.push(path);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
