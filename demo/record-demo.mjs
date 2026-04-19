// HealthOps Demo Video Recorder
// Uses Playwright to navigate the app and record a smooth demo video.
//
// Usage:  node record-demo.mjs
// Output: ./recordings/healthops-demo.webm

import { chromium } from 'playwright';
import { mkdir } from 'fs/promises';

const BASE = 'http://localhost:8080';
const VIEWPORT = { width: 1440, height: 900 };

// Timing helpers
const wait = (ms) => new Promise((r) => setTimeout(r, ms));

async function run() {
  await mkdir('recordings', { recursive: true });

  const browser = await chromium.launch({ headless: false });
  const context = await browser.newContext({
    viewport: VIEWPORT,
    recordVideo: {
      dir: 'recordings/',
      size: VIEWPORT,
    },
    colorScheme: 'light',
  });

  const page = await context.newPage();

  // Smooth scroll helper
  const smoothScroll = async (y, duration = 800) => {
    await page.evaluate(
      ([scrollY, dur]) => {
        return new Promise((resolve) => {
          const start = window.scrollY;
          const distance = scrollY - start;
          const startTime = performance.now();
          function step(currentTime) {
            const elapsed = currentTime - startTime;
            const progress = Math.min(elapsed / dur, 1);
            // easeInOutCubic
            const ease =
              progress < 0.5
                ? 4 * progress * progress * progress
                : 1 - Math.pow(-2 * progress + 2, 3) / 2;
            window.scrollTo(0, start + distance * ease);
            if (progress < 1) requestAnimationFrame(step);
            else resolve();
          }
          requestAnimationFrame(step);
        });
      },
      [y, duration]
    );
  };

  console.log('🎬 Recording demo...');

  // ── Scene 1: Dashboard ──────────────────────────────────
  console.log('  → Dashboard');
  await page.goto(BASE, { waitUntil: 'domcontentloaded', timeout: 60000 });
  await wait(2500); // admire the dashboard

  // Scroll down to show response times chart & check status
  await smoothScroll(300);
  await wait(2000);

  // Scroll further to active incidents & latest results
  await smoothScroll(700);
  await wait(2000);

  // Back to top
  await smoothScroll(0);
  await wait(1000);

  // ── Scene 2: Click "Run checks" button ──────────────────
  console.log('  → Run checks');
  const runBtn = page.getByRole('button', { name: /run checks/i });
  if (await runBtn.isVisible()) {
    await runBtn.click();
    await wait(3000); // watch the checks run
  }

  // ── Scene 3: Checks page ───────────────────────────────
  console.log('  → Checks');
  await page.click('text=Checks');
  await page.waitForLoadState('domcontentloaded');
  await wait(2000);

  // Click into a check detail
  const firstCheck = page.locator('table tbody tr').first();
  if (await firstCheck.isVisible()) {
    await firstCheck.click();
    await page.waitForLoadState('domcontentloaded');
    await wait(2500);

    // Scroll down to see results
    await smoothScroll(400);
    await wait(1500);
    await smoothScroll(0);
    await wait(500);
  }

  // ── Scene 4: Servers ───────────────────────────────────
  console.log('  → Servers');
  await page.click('text=Servers');
  await page.waitForLoadState('domcontentloaded');
  await wait(2000);

  // ── Scene 5: Incidents ─────────────────────────────────
  console.log('  → Incidents');
  await page.click('text=Incidents');
  await page.waitForLoadState('domcontentloaded');
  await wait(2000);

  // ── Scene 6: Analytics ─────────────────────────────────
  console.log('  → Analytics');
  await page.click('text=Analytics');
  await page.waitForLoadState('domcontentloaded');
  await wait(2000);

  // Scroll to see all charts
  await smoothScroll(400);
  await wait(1500);
  await smoothScroll(800);
  await wait(1500);
  await smoothScroll(0);
  await wait(1000);

  // ── Scene 7: AI Analysis ──────────────────────────────
  console.log('  → AI Analysis');
  await page.click('text=AI Analysis');
  await page.waitForLoadState('domcontentloaded');
  await wait(2500);

  // ── Scene 8: MySQL ────────────────────────────────────
  console.log('  → MySQL');
  await page.click('text=MySQL');
  await page.waitForLoadState('domcontentloaded');
  await wait(2000);

  // ── Scene 9: Settings ─────────────────────────────────
  console.log('  → Settings');
  await page.click('text=Settings');
  await page.waitForLoadState('domcontentloaded');
  await wait(2000);

  // Scroll to show all settings
  await smoothScroll(500);
  await wait(1500);
  await smoothScroll(0);
  await wait(1000);

  // ── Scene 10: Toggle dark mode ────────────────────────
  console.log('  → Dark mode toggle');
  // Look for theme toggle button (usually a sun/moon icon)
  const themeBtn = page.locator('button[aria-label*="theme"], button[aria-label*="dark"], button[aria-label*="mode"]');
  if (await themeBtn.count() > 0) {
    await themeBtn.first().click();
    await wait(1500);
  } else {
    // Try the header icon buttons (rightmost is usually dark mode)
    const headerBtns = page.locator('header button, nav button').last();
    if (await headerBtns.isVisible()) {
      await headerBtns.click();
      await wait(1500);
    }
  }

  // ── Scene 11: Dashboard in dark mode ──────────────────
  console.log('  → Dashboard (dark mode)');
  await page.click('text=Dashboard');
  await page.waitForLoadState('domcontentloaded');
  await wait(2500);

  // Scroll to show charts in dark mode
  await smoothScroll(400);
  await wait(2000);
  await smoothScroll(0);
  await wait(1500);

  // ── Finish ────────────────────────────────────────────
  console.log('  → Wrapping up...');
  await wait(1000);

  // Close context to finalize the video
  await context.close();
  await browser.close();

  console.log('✅ Demo recorded! Check recordings/ directory for the .webm file.');
}

run().catch((err) => {
  console.error('Recording failed:', err);
  process.exit(1);
});
