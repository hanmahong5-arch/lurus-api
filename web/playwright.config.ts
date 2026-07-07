import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright config.
 * Default host = STAGE; override for local: E2E_BASE_URL=http://localhost:5173.
 *
 * Auth model: a single `setup` project logs in ONCE via the Layer C bridge and
 * saves storageState (cookie + localStorage). All specs reuse it — the bridge is
 * rate-limited to 5 logins/60s (BootstrapRateLimit), so per-test login 429s.
 *
 * Not in PR-blocking CI; runs nightly / locally before merging.
 */

const baseURL = process.env.E2E_BASE_URL || 'https://test-newhub.lurus.cn';

export const STORAGE_STATE = 'tests/e2e/.auth/state.json';

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
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
      name: 'setup',
      testMatch: /global\.setup\.ts/,
    },
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'], storageState: STORAGE_STATE },
      dependencies: ['setup'],
    },
  ],
});
