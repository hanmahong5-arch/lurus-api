import { test, expect } from '@playwright/test';

/**
 * Story 11-2 DoD §e2e:
 *   "log in via Layer C bridge → land on v2 dashboard → create token →
 *    see it in token list → log out → log back in → token still there."
 *
 * Required env (not committed; injected at runtime):
 *   E2E_BASE_URL              default https://test-newhub.lurus.cn
 *   E2E_BRIDGE_TOKEN          Layer C bridge auth token (skip OIDC interactive flow)
 *   E2E_TENANT_SLUG           e.g. "default"
 *
 * When credentials are not set the spec test.skip()s with a clear reason —
 * it does NOT silently pass (avoid §4.1 ⑥ marker-vs-measurement trap).
 */

const BRIDGE_TOKEN = process.env.E2E_BRIDGE_TOKEN;
const TENANT_SLUG = process.env.E2E_TENANT_SLUG || 'default';

test.describe('Story 11-2 — token persistence across re-login', () => {
  test.skip(
    !BRIDGE_TOKEN,
    'E2E_BRIDGE_TOKEN not set — skipping STAGE e2e. ' +
      'Set the Layer C bridge token to run this spec. ' +
      'See _bmad-output/planning-artifacts/story-11-2-v2-wiring-mvp.md',
  );

  const tokenName = `e2e-persistence-${Date.now()}`;
  let createdTokenId: string | null = null;

  test('create → relogin → token persists', async ({ page, request }) => {
    // ── Step 1: log in via Layer C bridge ──────────────────────────────
    await page.goto(`/api/v2/bridge/exchange?token=${BRIDGE_TOKEN}`);

    // Expect redirect to v2 dashboard.
    await expect(page).toHaveURL(/\/console\/v2\/dashboard/, {
      timeout: 10_000,
    });

    // ── Step 2: create a token via API (faster than UI form) ───────────
    const createRes = await request.post(`/api/v2/${TENANT_SLUG}/tokens`, {
      data: { name: tokenName, remain_quota: 1000 },
    });
    expect(createRes.ok()).toBeTruthy();
    const created = await createRes.json();
    createdTokenId = String(created?.data?.id ?? '');
    expect(createdTokenId).toBeTruthy();

    // ── Step 3: confirm in v2 Token page UI ────────────────────────────
    await page.goto('/console/v2/token');
    await expect(page.getByText(tokenName)).toBeVisible({ timeout: 10_000 });

    // ── Step 4: log out ────────────────────────────────────────────────
    await page.context().clearCookies();
    await page.goto('/');

    // ── Step 5: log back in via bridge ─────────────────────────────────
    await page.goto(`/api/v2/bridge/exchange?token=${BRIDGE_TOKEN}`);
    await expect(page).toHaveURL(/\/console\/v2\/dashboard/, {
      timeout: 10_000,
    });

    // ── Step 6: assert token still in list ─────────────────────────────
    await page.goto('/console/v2/token');
    await expect(page.getByText(tokenName)).toBeVisible({ timeout: 10_000 });
  });

  test.afterEach(async ({ request }) => {
    if (createdTokenId) {
      await request.delete(`/api/v2/${TENANT_SLUG}/tokens/${createdTokenId}`);
    }
  });
});
