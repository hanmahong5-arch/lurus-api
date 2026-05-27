import { test, expect } from '@playwright/test';

/**
 * Phase E4 — quota threshold audit events (commit 066c5356).
 *
 * Spec: verify that the audit endpoint surfaces billing.quota_threshold
 * events whenever a user's used_quota crosses one of the configured
 * percentage rungs (default 50/80/95/100). This spec does NOT burn real
 * relay quota — it asserts the action exists in the discovery taxonomy
 * and that historical crossings (seeded by prior runs or live traffic)
 * are queryable + return the canonical details shape.
 *
 * Required env:
 *   E2E_BASE_URL              default https://test-newhub.lurus.cn
 *   E2E_BRIDGE_TOKEN          Layer C bridge auth token
 *   E2E_TENANT_SLUG           default "default"
 *
 * If STAGE has no historical billing.quota_threshold rows yet (fresh
 * cluster), the threshold-row assertion soft-skips with an inline note
 * rather than failing — quota crossings require live LLM traffic to
 * accumulate, which is out of this spec's scope.
 */

const BRIDGE_TOKEN = process.env.E2E_BRIDGE_TOKEN;
const TENANT_SLUG = process.env.E2E_TENANT_SLUG || 'default';

test.describe('Phase E4 — quota threshold audit', () => {
  test.skip(
    !BRIDGE_TOKEN,
    'E2E_BRIDGE_TOKEN not set — skipping STAGE e2e.',
  );

  test('action present in taxonomy + historical rows queryable', async ({
    page,
    request,
  }) => {
    await page.goto(`/api/v2/bridge/exchange?token=${BRIDGE_TOKEN}`);
    await expect(page).toHaveURL(/\/console\/v2\/dashboard/, { timeout: 10_000 });

    const actionsRes = await request.get(`/api/v2/${TENANT_SLUG}/admin/audit/actions`);
    expect(actionsRes.ok()).toBeTruthy();
    const actions = ((await actionsRes.json())?.data?.actions ?? []) as string[];
    expect(actions).toContain('billing.quota_threshold');

    const eventsRes = await request.get(
      `/api/v2/${TENANT_SLUG}/admin/audit/events?action=billing.quota_threshold&limit=10`,
    );
    expect(eventsRes.ok()).toBeTruthy();
    const eventsJson = await eventsRes.json();
    const events = (eventsJson?.data?.events ?? []) as any[];

    test.skip(
      events.length === 0,
      'no billing.quota_threshold rows in STAGE yet — populate by ' +
        'running synthetic relay traffic against a low-quota token, ' +
        'then re-run. See internal/app/quota_threshold.go for the ladder.',
    );

    const sample = events[0];
    expect(sample?.action).toBe('billing.quota_threshold');
    expect(sample?.actor).toBe('system');
    expect(sample?.resource).toBe('user');

    const details = JSON.parse(String(sample?.details ?? '{}'));
    expect(details).toMatchObject({
      threshold_pct: expect.any(Number),
      used_tokens: expect.any(Number),
      limit_tokens: expect.any(Number),
      usage_percent: expect.any(Number),
      account_id: expect.any(Number),
    });
    expect([50, 80, 95, 100]).toContain(details.threshold_pct);
    expect(details.usage_percent).toBeGreaterThanOrEqual(details.threshold_pct);

    const csvRes = await request.get(
      `/api/v2/${TENANT_SLUG}/admin/audit/export?format=csv&action=billing.quota_threshold&limit=10`,
    );
    expect(csvRes.ok()).toBeTruthy();
    const csv = await csvRes.text();
    expect(csv).toContain('billing.quota_threshold');
    expect(csv).toMatch(/threshold_pct/);
  });
});
