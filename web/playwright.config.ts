import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright config — runs against STAGE by default.
 * Override host via env: E2E_BASE_URL=https://test-newhub.lurus.cn bun run test:e2e
 *
 * Note: e2e is NOT in PR-blocking CI (Lane A plan §3). It runs:
 *   - Nightly via GHA workflow_dispatch
 *   - Locally before merging Story 11-2 DoD revisions
 */

const baseURL = process.env.E2E_BASE_URL || 'https://test-newhub.lurus.cn';

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [
    ['list'],
    ['html', { open: 'never', outputFolder: 'playwright-report' }],
    ['json', { outputFile: 'playwright-report/results.json' }],
  ],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
