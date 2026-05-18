import React, { useCallback, useEffect, useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';
import { API, showError } from '../../../helpers';
import {
  computeQPS,
  computeLatencyP50,
  computeLatencyP95,
  computeLatencyP99,
  computeErrorRate,
  computeCostByModel,
  pickRecent,
  formatQPS,
  formatLatencyMs,
  formatErrorRate,
  DASHBOARD_REALTIME_WINDOW_SECONDS,
} from './kpis';

const QUOTA_PER_USD = 500_000;

const useTenantSlug = () => {
  const [slug, setSlug] = useState('default');
  useEffect(() => {
    try {
      const s = localStorage.getItem('tenant_slug');
      if (s) setSlug(s);
    } catch (_) {}
  }, []);
  return slug;
};

const quotaToUSD = (q) => (q / QUOTA_PER_USD).toFixed(2);

const fmtTs = (ts) => {
  if (!ts) return '—';
  try {
    return new Date(ts).toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch (_) {
    return ts;
  }
};

// Realtime KPI tiles derived from the last DASHBOARD_REALTIME_WINDOW_SECONDS
// window of /api/v2/{slug}/logs. No dedicated metrics endpoint exists yet;
// see _bmad-output/planning-artifacts/hardening-swarm-2026-05-18-acceptance.md.

// Shown when the user has zero tokens — Reseller-MVP onboarding TTFT lift,
// modelled on OpenRouter / Anthropic quickstart pattern.
const RELAY_BASE_URL = 'https://api.lurus.cn/v1';

const OnboardingCurlBlock = ({ username, tenantSlug }) => {
  const navigateToTokens = (e) => {
    e.preventDefault();
    window.location.href = '/console/v2/token';
  };
  const curlExample = `curl ${RELAY_BASE_URL}/chat/completions \\
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello from ${username || 'lurus-hub'}!"}]
  }'`;
  return (
    <div
      role='region'
      aria-label='get started'
      style={{
        margin: '14px 24px 0',
        padding: '18px 22px',
        border: '1px solid var(--hf-accent, #c97a3e)',
        borderRadius: 2,
        background: 'rgba(201,122,62,0.06)',
      }}
    >
      <div
        className='lbl'
        style={{ fontSize: 11, marginBottom: 8, color: 'var(--hf-accent)' }}
      >
        get started — first relay call
      </div>
      <div
        className='display'
        style={{ fontSize: 18, marginBottom: 10 }}
      >
        You have <strong>0 tokens</strong> in tenant <code>{tenantSlug}</code>.
        Create one to make your first call.
      </div>
      <div
        style={{
          display: 'flex',
          gap: 10,
          marginBottom: 12,
          flexWrap: 'wrap',
        }}
      >
        <a
          href='/console/v2/token'
          onClick={navigateToTokens}
          className='btn primary'
          style={{ textDecoration: 'none', padding: '6px 14px' }}
        >
          + create token
        </a>
        <span className='mono muted' style={{ fontSize: 11, alignSelf: 'center' }}>
          then paste it into the snippet below
        </span>
      </div>
      <pre
        style={{
          background: '#0a0908',
          color: '#e8e3d4',
          padding: 14,
          margin: 0,
          fontSize: 11,
          lineHeight: 1.55,
          fontFamily: 'var(--hf-mono)',
          overflow: 'auto',
          border: '1px solid var(--hf-rule-strong)',
        }}
      >
        {curlExample}
      </pre>
    </div>
  );
};

// ─── Main page ────────────────────────────────────────────────────────────────

const HFDashboard = () => {
  const tenantSlug = useTenantSlug();

  const [me, setMe] = useState(null);
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(true);

  const fetchData = useCallback(async () => {
    if (!tenantSlug) return;
    setLoading(true);
    const startTime =
      Math.floor(Date.now() / 1000) - DASHBOARD_REALTIME_WINDOW_SECONDS;
    try {
      const [meRes, logsRes] = await Promise.all([
        API.get(`/api/v2/${tenantSlug}/user/me`),
        API.get(
          `/api/v2/${tenantSlug}/logs?page=1&page_size=200&start_time=${startTime}`,
        ),
      ]);
      if (meRes?.data?.success) setMe(meRes.data.data);
      if (logsRes?.data?.success) {
        const items = logsRes.data.data?.items ?? logsRes.data.data ?? [];
        setLogs(Array.isArray(items) ? items : []);
      }
    } catch (e) {
      showError('Failed to load dashboard data');
    } finally {
      setLoading(false);
    }
  }, [tenantSlug]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Derive spend KPI from user/me
  const spendUSD = me ? parseFloat(quotaToUSD(me.used_quota ?? 0)) : null;
  const remainUSD =
    me && me.remaining_quota != null
      ? me.remaining_quota < 0
        ? '∞'
        : `$${quotaToUSD(me.remaining_quota)}`
      : null;

  // Realtime KPIs derived from the fetched 5-minute window.
  const qps = computeQPS(logs, DASHBOARD_REALTIME_WINDOW_SECONDS);
  const p50 = computeLatencyP50(logs);
  const p95 = computeLatencyP95(logs);
  const p99 = computeLatencyP99(logs);
  const errorRate = computeErrorRate(logs);
  const costByModel = computeCostByModel(logs).slice(0, 6);
  const hasRealtimeData = logs.length > 0;
  const showOnboarding = !loading && me && (me.token_count ?? 0) === 0;

  // Activity table uses the most-recent slice only.
  const recentLogs = pickRecent(logs, 5);

  return (
    <HFShell
      active='dashboard'
      crumbs={['workspace', 'dashboard']}
      actions={
        <>
          <span className='muted mono' style={{ fontSize: 11 }}>
            {loading
              ? 'loading…'
              : me
              ? `${me.request_count ?? 0} requests · $${spendUSD?.toFixed(2) ?? '—'} spent`
              : ''}
          </span>
          <button type='button' className='btn' onClick={fetchData}>
            refresh
          </button>
        </>
      }
    >
      {showOnboarding && (
        <OnboardingCurlBlock username={me?.username} tenantSlug={tenantSlug} />
      )}
      <div className='hf-page-head'>
        <div>
          <div className='lbl' style={{ marginBottom: 6 }}>
            at a glance
          </div>
          <h1>
            {loading ? (
              'Loading…'
            ) : me ? (
              <>
                {me.display_name || me.username || 'Your workspace'}{' '}
                <span className='muted' style={{ fontWeight: 400 }}>
                  · {remainUSD} remaining
                </span>
              </>
            ) : (
              'Dashboard'
            )}
          </h1>
          <div className='sub'>
            {me
              ? `${me.token_count ?? 0} active tokens · ${me.request_count ?? 0} total requests`
              : 'Usage overview for your workspace'}
          </div>
        </div>
      </div>

      <div
        style={{
          padding: 24,
          display: 'grid',
          gridTemplateColumns: 'repeat(12, 1fr)',
          gap: 14,
        }}
      >
        {/* ── KPI: Total spend (real) ── */}
        <div className='panel' style={{ gridColumn: 'span 3', padding: 18 }}>
          <div className='lbl'>total spend</div>
          <div className='display' style={{ fontSize: 32, marginTop: 4 }}>
            {loading ? '…' : me ? `$${spendUSD.toFixed(2)}` : '—'}
          </div>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'flex-end',
              marginTop: 8,
            }}
          >
            <span className='mono muted' style={{ fontSize: 10 }}>
              all time · quota units
            </span>
          </div>
        </div>

        {/* ── KPI: Remaining quota (real) ── */}
        <div className='panel' style={{ gridColumn: 'span 3', padding: 18 }}>
          <div className='lbl'>remaining quota</div>
          <div className='display' style={{ fontSize: 32, marginTop: 4 }}>
            {loading ? '…' : remainUSD ?? '—'}
          </div>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'flex-end',
              marginTop: 8,
            }}
          >
            <span className='mono muted' style={{ fontSize: 10 }}>
              {me && me.remaining_quota >= 0 ? 'until top-up' : 'unlimited plan'}
            </span>
          </div>
        </div>

        {/* ── KPI: Total requests (real) ── */}
        <div className='panel' style={{ gridColumn: 'span 3', padding: 18 }}>
          <div className='lbl'>total requests</div>
          <div className='display' style={{ fontSize: 32, marginTop: 4 }}>
            {loading ? '…' : me ? (me.request_count ?? 0).toLocaleString() : '—'}
          </div>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'flex-end',
              marginTop: 8,
            }}
          >
            <span className='mono muted' style={{ fontSize: 10 }}>
              all time
            </span>
          </div>
        </div>

        {/* ── KPI: Active tokens (real) ── */}
        <div className='panel' style={{ gridColumn: 'span 3', padding: 18 }}>
          <div className='lbl'>active tokens</div>
          <div className='display' style={{ fontSize: 32, marginTop: 4 }}>
            {loading ? '…' : me ? (me.token_count ?? 0) : '—'}
          </div>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'flex-end',
              marginTop: 8,
            }}
          >
            <span className='mono muted' style={{ fontSize: 10 }}>
              in this workspace
            </span>
          </div>
        </div>

        {/* ── KPI: QPS (derived from last 5min of logs) ── */}
        <div className='panel' style={{ gridColumn: 'span 4', padding: 18 }}>
          <div className='lbl'>qps</div>
          <div
            className='display'
            style={{
              fontSize: 32,
              marginTop: 4,
              color: hasRealtimeData ? 'var(--hf-accent)' : 'var(--hf-ink-3)',
            }}
          >
            {loading ? '…' : hasRealtimeData ? formatQPS(qps) : '—'}
          </div>
          <div
            style={{
              marginTop: 8,
              fontSize: 10,
              color: 'var(--hf-ink-3)',
              fontFamily: 'var(--hf-mono)',
            }}
          >
            {hasRealtimeData ? 'last 5 min · req/s' : 'no traffic in last 5 min'}
          </div>
        </div>

        {/* ── KPI: Latency P50/P95/P99 (P99 anchors the SLO) ── */}
        <div className='panel' style={{ gridColumn: 'span 4', padding: 18 }}>
          <div className='lbl'>latency · ms</div>
          <div
            style={{
              marginTop: 6,
              display: 'grid',
              gridTemplateColumns: '1fr 1fr 1fr',
              gap: 8,
              alignItems: 'baseline',
            }}
          >
            {[
              ['p50', p50, 'var(--hf-ok)'],
              ['p95', p95, 'var(--hf-warn, #c08a3e)'],
              ['p99', p99, 'var(--hf-err)'],
            ].map(([label, val, color]) => (
              <div key={label}>
                <div
                  className='display'
                  style={{
                    fontSize: 22,
                    color: val != null ? color : 'var(--hf-ink-3)',
                  }}
                >
                  {loading ? '…' : val != null ? formatLatencyMs(val) : '—'}
                </div>
                <div
                  className='mono'
                  style={{ fontSize: 9, color: 'var(--hf-ink-3)', marginTop: 2 }}
                >
                  {label}
                </div>
              </div>
            ))}
          </div>
          <div
            style={{
              marginTop: 8,
              fontSize: 10,
              color: 'var(--hf-ink-3)',
              fontFamily: 'var(--hf-mono)',
            }}
          >
            {p99 != null
              ? 'percentiles · last 5 min · p99 anchors SLO'
              : 'awaiting requests with latency data'}
          </div>
        </div>

        {/* ── KPI: Error rate (derived from log type 5 share) ── */}
        <div className='panel' style={{ gridColumn: 'span 4', padding: 18 }}>
          <div className='lbl'>error rate</div>
          <div
            className='display'
            style={{
              fontSize: 32,
              marginTop: 4,
              color:
                !hasRealtimeData
                  ? 'var(--hf-ink-3)'
                  : errorRate > 0.05
                    ? 'var(--hf-err)'
                    : 'var(--hf-ok)',
            }}
          >
            {loading ? '…' : hasRealtimeData ? formatErrorRate(errorRate) : '—'}
          </div>
          <div
            style={{
              marginTop: 8,
              fontSize: 10,
              color: 'var(--hf-ink-3)',
              fontFamily: 'var(--hf-mono)',
            }}
          >
            {hasRealtimeData ? 'last 5 min · err / consume+err' : 'no traffic in last 5 min'}
          </div>
        </div>

        {/* ── Cost by model · last 5 min (derived from /logs aggregation) ── */}
        <div className='panel' style={{ gridColumn: 'span 7', padding: 18 }}>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'baseline',
              marginBottom: 12,
            }}
          >
            <div>
              <div className='lbl'>cost by model · last 5 min</div>
              <div className='display' style={{ fontSize: 18, marginTop: 2 }}>
                {costByModel.length > 0
                  ? `${costByModel.length} model${costByModel.length === 1 ? '' : 's'} active`
                  : 'No consume traffic yet'}
              </div>
            </div>
            <span className='faint mono' style={{ fontSize: 10 }}>
              {costByModel.length > 0
                ? 'derived from /logs'
                : 'awaiting traffic'}
            </span>
          </div>
          {costByModel.length === 0 && (
            <div
              className='muted'
              style={{
                fontSize: 11,
                fontFamily: 'var(--hf-mono)',
                padding: '24px 0',
                textAlign: 'center',
              }}
            >
              once a relay call is consumed, the model breakdown lands here.
            </div>
          )}
          {costByModel.length > 0 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {(() => {
                const maxQuota = costByModel[0].totalQuota || 1;
                return costByModel.map((row, i) => {
                  const pct = (row.totalQuota / maxQuota) * 100;
                  const usd = (row.totalQuota / QUOTA_PER_USD).toFixed(4);
                  return (
                    <div key={row.model}>
                      <div
                        style={{
                          display: 'flex',
                          justifyContent: 'space-between',
                          marginBottom: 3,
                          fontSize: 11,
                        }}
                      >
                        <span
                          className='mono'
                          style={{
                            color: 'var(--hf-ink)',
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                            maxWidth: '60%',
                          }}
                        >
                          {row.model}
                        </span>
                        <span className='mono muted' style={{ fontSize: 10 }}>
                          ${usd} · {row.requestCount} req
                        </span>
                      </div>
                      <div
                        style={{
                          height: 6,
                          background: 'var(--hf-sunken)',
                          borderRadius: 1,
                          overflow: 'hidden',
                        }}
                      >
                        <div
                          style={{
                            height: '100%',
                            width: pct + '%',
                            background:
                              i === 0
                                ? 'var(--hf-accent)'
                                : 'var(--hf-info, #2c5fb0)',
                            transition: 'width 0.4s ease',
                          }}
                        />
                      </div>
                    </div>
                  );
                });
              })()}
            </div>
          )}
        </div>

        {/* ── Recent activity table ── */}
        <div className='panel' style={{ gridColumn: 'span 5', padding: 18 }}>
          <div className='lbl' style={{ marginBottom: 10 }}>
            recent activity
          </div>
          {loading && (
            <div className='muted' style={{ fontSize: 12 }}>
              Loading…
            </div>
          )}
          {!loading && recentLogs.length === 0 && (
            <div className='muted' style={{ fontSize: 12 }}>
              No recent requests found.
            </div>
          )}
          {!loading && recentLogs.length > 0 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: '1fr 1fr auto',
                  padding: '4px 0 6px',
                  borderBottom: '1px solid var(--hf-rule)',
                  marginBottom: 4,
                }}
              >
                <span className='lbl' style={{ fontSize: 10 }}>time</span>
                <span className='lbl' style={{ fontSize: 10 }}>model</span>
                <span className='lbl' style={{ fontSize: 10, textAlign: 'right' }}>cost</span>
              </div>
              {recentLogs.map((log, i) => {
                const model = log.model || log.ModelName || log.channel_name || '—';
                const cost = log.quota != null ? `$${quotaToUSD(log.quota)}` : '—';
                const ts = fmtTs(log.created_at || log.CreatedAt || null);
                return (
                  <div
                    key={i}
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '1fr 1fr auto',
                      padding: '7px 0',
                      borderBottom: i < recentLogs.length - 1 ? '1px dashed var(--hf-rule)' : 0,
                      fontSize: 11,
                      alignItems: 'center',
                    }}
                  >
                    <span className='mono muted' style={{ fontSize: 10 }}>
                      {ts}
                    </span>
                    <span
                      className='mono'
                      style={{
                        fontSize: 10,
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                        paddingRight: 6,
                      }}
                    >
                      {model}
                    </span>
                    <span className='mono strong' style={{ fontSize: 10 }}>
                      {cost}
                    </span>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </HFShell>
  );
};

export default HFDashboard;
