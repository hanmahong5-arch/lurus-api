import { test, expect } from '@playwright/test';

/**
 * Phase E3 — audit taxonomy + 7y retention + export endpoint
 * (commit acc22131).
 *
 * Spec: trigger a token update → confirm:
 *   - audit_events row with action="token.updated" lands
 *   - /admin/audit/actions returns the 50+ action taxonomy
 *   - /admin/audit/export?format=csv returns CSV with the event
 *   - /admin/audit/export?format=json returns paginated JSON
 *
 * Required env:
 *   E2E_BASE_URL              default https://test-newhub.lurus.cn
 *   E2E_BRIDGE_TOKEN          Layer C bridge auth token
 *   E2E_TENANT_SLUG           default "default"
 */

const BRIDGE_TOKEN = process.env.E2E_BRIDGE_TOKEN;
const TENANT_SLUG = process.env.E2E_TENANT_SLUG || 'default';

test.describe('Phase E3 — audit export round-trip', () => {
  test.skip(
    !BRIDGE_TOKEN,
    'E2E_BRIDGE_TOKEN not set — skipping STAGE e2e.',
  );

  const tokenName = `e2e-audit-export-${Date.now()}`;
  let createdId: string | null = null;

  test('action discovery + CSV/JSON export round-trip', async ({ page, request }) => {
    await page.goto(`/api/v2/bridge/exchange?token=${BRIDGE_TOKEN}`);
    await expect(page).toHaveURL(/\/console\/v2\/dashboard/, { timeout: 10_000 });

    const actionsRes = await request.get(`/api/v2/${TENANT_SLUG}/admin/audit/actions`);
    expect(actionsRes.ok()).toBeTruthy();
    const actionsJson = await actionsRes.json();
    const actions = (actionsJson?.data?.actions ?? []) as string[];
    expect(actions.length).toBeGreaterThanOrEqual(50);
    expect(actions).toContain('token.updated');
    expect(actions).toContain('billing.quota_threshold');

    const createRes = await request.post(`/api/v2/${TENANT_SLUG}/tokens`, {
      data: { name: tokenName, remain_quota: 100 },
    });
    expect(createRes.ok()).toBeTruthy();
    createdId = String((await createRes.json())?.data?.id ?? '');
    expect(createdId).toBeTruthy();

    const updateRes = await request.put(`/api/v2/${TENANT_SLUG}/tokens/${createdId}`, {
      data: { remain_quota: 500 },
    });
    expect(updateRes.ok()).toBeTruthy();

    await new Promise((r) => setTimeout(r, 2_000));

    const csvRes = await request.get(
      `/api/v2/${TENANT_SLUG}/admin/audit/export?format=csv&action=token.updated&limit=5`,
    );
    expect(csvRes.ok()).toBeTruthy();
    expect(csvRes.headers()['content-type']).toMatch(/text\/csv/);
    const csvBody = await csvRes.text();
    expect(csvBody.split('\n')[0]).toMatch(/^id,ts,actor,actor_id,action,resource/);
    expect(csvBody).toContain('token.updated');
    expect(csvBody).toContain(createdId);

    const jsonRes = await request.get(
      `/api/v2/${TENANT_SLUG}/admin/audit/export?format=json&action=token.updated&limit=5`,
    );
    expect(jsonRes.ok()).toBeTruthy();
    const jsonBody = await jsonRes.json();
    expect(Array.isArray(jsonBody?.data?.events)).toBe(true);
    expect(jsonBody?.data?.events?.length).toBeGreaterThan(0);

    const badRes = await request.get(
      `/api/v2/${TENANT_SLUG}/admin/audit/export?format=xml`,
    );
    expect(badRes.status()).toBeGreaterThanOrEqual(400);
  });

  test.afterEach(async ({ request }) => {
    if (createdId) {
      await request.delete(`/api/v2/${TENANT_SLUG}/tokens/${createdId}`);
    }
  });
});
