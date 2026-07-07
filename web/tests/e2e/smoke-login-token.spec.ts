import { test, expect } from '@playwright/test';
import { BRIDGE_TOKEN, BRIDGE_SKIP, gotoDashboard, v2 } from './helpers/auth';

/**
 * Harness smoke test — proves the local stack + bridge login + tenant routing
 * work end to end. Gateway for the rest of the matrix.
 */
test.describe('smoke — bridge login + token CRUD', () => {
  test.skip(!BRIDGE_TOKEN, BRIDGE_SKIP);

  const tokenName = `e2e-smoke-${Date.now()}`;
  let tokenId = '';

  test('login → dashboard → token create/list/delete (API + UI)', async ({
    page,
  }) => {
    // 1. Shared session (storageState) lands on dashboard (PrivateRoute admits us).
    await gotoDashboard(page);

    // 2. Create a token via API (slug = lurus).
    const createRes = await page.request.post(v2('/tokens'), {
      data: { name: tokenName, remain_quota: 1000 },
    });
    expect(
      createRes.ok(),
      `create token: HTTP ${createRes.status()} ${await createRes.text()}`,
    ).toBeTruthy();
    const created = await createRes.json();
    tokenId = String(created?.data?.id ?? '');
    expect(tokenId).toBeTruthy();

    // 3. List contains it.
    const listRes = await page.request.get(v2('/tokens?p=1&size=100'));
    expect(listRes.ok()).toBeTruthy();
    const listBody = await listRes.json();
    const items =
      listBody?.data?.items ?? listBody?.data?.records ?? listBody?.data ?? [];
    const names = JSON.stringify(items);
    expect(names).toContain(tokenName);

    // 4. UI: token page shows it (name appears in both a title span and a
    //    sub-label, so scope to the exact-text match to satisfy strict mode).
    await page.goto('/console/v2/token');
    await expect(
      page.getByText(tokenName, { exact: true }).first(),
    ).toBeVisible({ timeout: 15_000 });
  });

  test.afterEach(async ({ page }) => {
    if (tokenId) {
      await page.request.delete(v2(`/tokens/${tokenId}`));
    }
  });
});
