import { test, expect } from '@playwright/test';
import {
  BRIDGE_TOKEN,
  BRIDGE_SKIP,
  loginViaBridge,
  gotoDashboard,
  v2,
} from './helpers/auth';

/**
 * Story 11-2 DoD §e2e:
 *   "log in via Layer C bridge → land on v2 dashboard → create token →
 *    see it in token list → log out → log back in → token still there."
 *
 * REWRITTEN 2026-06-20: the bridge is POST + required user_id (was GET in the
 * old spec); tenant slug is `lurus` (the seeded default tenant's routable slug,
 * not the bridge's cosmetic "default"). See helpers/auth.ts.
 *
 * Fully local-testable (no external provider needed).
 */
test.describe('Story 11-2 — token persistence across re-login', () => {
  test.skip(!BRIDGE_TOKEN, BRIDGE_SKIP);

  const tokenName = `e2e-persistence-${Date.now()}`;
  let createdTokenId = '';

  test('create → relogin → token persists', async ({ page }) => {
    // Step 1: shared session lands on dashboard.
    await gotoDashboard(page);

    // Step 2: create a token via API (faster than the UI form).
    const createRes = await page.request.post(v2('/tokens'), {
      data: { name: tokenName, remain_quota: 1000 },
    });
    expect(
      createRes.ok(),
      `create token: HTTP ${createRes.status()} ${await createRes.text()}`,
    ).toBeTruthy();
    createdTokenId = String((await createRes.json())?.data?.id ?? '');
    expect(createdTokenId).toBeTruthy();

    // Step 3: confirm in the v2 Token page UI.
    await page.goto('/console/v2/token');
    await expect(
      page.getByText(tokenName, { exact: true }).first(),
    ).toBeVisible({ timeout: 15_000 });

    // Step 4: log out (drop the session cookie).
    await page.context().clearCookies();

    // Step 5: log back in via bridge.
    await loginViaBridge(page);
    await page.goto('/console/v2/dashboard');
    await expect(page).toHaveURL(/\/console\/v2\/dashboard/, {
      timeout: 15_000,
    });

    // Step 6: token still present after re-login.
    await page.goto('/console/v2/token');
    await expect(
      page.getByText(tokenName, { exact: true }).first(),
    ).toBeVisible({ timeout: 15_000 });
  });

  test.afterEach(async ({ page }) => {
    if (createdTokenId) {
      await page.request.delete(v2(`/tokens/${createdTokenId}`));
    }
  });
});
