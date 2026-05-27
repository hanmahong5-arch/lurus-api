import { test, expect } from '@playwright/test';

/**
 * Phase E2 — per-token relay scope allowlist (commit ebb646f7).
 *
 * Spec: a token with scopes=["chat"] should:
 *   - succeed on /v1/chat/completions
 *   - return 403 on /v1/embeddings
 *   - record an auth.scope_rejected audit row with the rejected scope name
 *
 * Required env:
 *   E2E_BASE_URL              default https://test-newhub.lurus.cn
 *   E2E_BRIDGE_TOKEN          Layer C bridge auth token
 *   E2E_TENANT_SLUG           default "default"
 *
 * Skips with a clear reason if env missing — never silently passes
 * (avoids §4.1 ⑥ marker-vs-measurement trap).
 */

const BRIDGE_TOKEN = process.env.E2E_BRIDGE_TOKEN;
const TENANT_SLUG = process.env.E2E_TENANT_SLUG || 'default';

test.describe('Phase E2 — token scope rejection', () => {
  test.skip(
    !BRIDGE_TOKEN,
    'E2E_BRIDGE_TOKEN not set — skipping STAGE e2e. ' +
      'Configure STAGING_KUBECONFIG + bridge token to run. ' +
      'See _bmad-output/planning-artifacts/story-11-2-v2-wiring-mvp.md',
  );

  const tokenName = `e2e-scope-${Date.now()}`;
  let scopedToken: { id: string; key: string } | null = null;

  test('chat-scoped token: chat allowed, embeddings rejected with 403 + audit', async ({
    page,
    request,
  }) => {
    await page.goto(`/api/v2/bridge/exchange?token=${BRIDGE_TOKEN}`);
    await expect(page).toHaveURL(/\/console\/v2\/dashboard/, { timeout: 10_000 });

    const createRes = await request.post(`/api/v2/${TENANT_SLUG}/tokens`, {
      data: {
        name: tokenName,
        remain_quota: 1_000_000,
        scopes: ['chat'],
      },
    });
    expect(createRes.ok()).toBeTruthy();
    const created = await createRes.json();
    scopedToken = {
      id: String(created?.data?.id ?? ''),
      key: String(created?.data?.key ?? ''),
    };
    expect(scopedToken.id).toBeTruthy();
    expect(scopedToken.key).toBeTruthy();

    const auditBefore = await request.get(
      `/api/v2/${TENANT_SLUG}/admin/audit/events?action=auth.scope_rejected&limit=1`,
    );
    const auditBeforeJson = await auditBefore.json();
    const baselineCount = auditBeforeJson?.data?.total ?? 0;

    const chatRes = await request.post('/v1/chat/completions', {
      headers: { Authorization: `Bearer ${scopedToken.key}` },
      data: {
        model: 'gpt-3.5-turbo',
        messages: [{ role: 'user', content: 'ping' }],
        max_tokens: 1,
      },
    });
    expect([200, 502, 503]).toContain(chatRes.status());

    const embedRes = await request.post('/v1/embeddings', {
      headers: { Authorization: `Bearer ${scopedToken.key}` },
      data: { model: 'text-embedding-3-small', input: 'foo' },
    });
    expect(embedRes.status()).toBe(403);
    const embedBody = await embedRes.json();
    expect(JSON.stringify(embedBody)).toMatch(/scope/i);

    await new Promise((r) => setTimeout(r, 2_000));
    const auditAfter = await request.get(
      `/api/v2/${TENANT_SLUG}/admin/audit/events?action=auth.scope_rejected&limit=10`,
    );
    expect(auditAfter.ok()).toBeTruthy();
    const auditAfterJson = await auditAfter.json();
    expect(auditAfterJson?.data?.total ?? 0).toBeGreaterThan(baselineCount);

    const recent = (auditAfterJson?.data?.events ?? []).find((e: any) =>
      String(e?.details ?? '').includes('embedding'),
    );
    expect(recent, 'audit row with rejected scope=embedding should exist').toBeTruthy();
  });

  test.afterEach(async ({ request }) => {
    if (scopedToken?.id) {
      await request.delete(`/api/v2/${TENANT_SLUG}/tokens/${scopedToken.id}`);
    }
  });
});
