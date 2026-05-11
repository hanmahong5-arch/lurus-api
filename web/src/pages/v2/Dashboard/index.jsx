import React, { Fragment, useCallback, useEffect, useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';
import { API, showError } from '../../../helpers';

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

// KPI cards that don't have real-time data yet — shown with a placeholder value.
const STATIC_KPIS = [
  { l: 'qps', c: 'var(--hf-accent)' },
  { l: 'ttft p50', c: 'var(--hf-ok)' },
  { l: 'error rate', c: 'var(--hf-err)' },
];

// ─── Main page ────────────────────────────────────────────────────────────────

const HFDashboard = () => {
  const tenantSlug = useTenantSlug();

  const [me, setMe] = useState(null);
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(true);

  const fetchData = useCallback(async () => {
    if (!tenantSlug) return;
    setLoading(true);
    try {
      const [meRes, logsRes] = await Promise.all([
        API.get(`/api/v2/${tenantSlug}/user/me`),
        API.get(`/api/v2/${tenantSlug}/logs?page=1&page_size=5`),
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

  // Unique models from recent logs for the bubble chart
  const modelSet = Array.from(
    new Set(logs.map((l) => l.model || l.ModelName || l.channel_name).filter(Boolean))
  ).slice(0, 8);

  // Spread bubbles evenly in a predictable layout
  const BUBBLE_COLORS = [
    'var(--hf-ok)',
    'var(--hf-accent)',
    'var(--hf-info)',
    'var(--hf-ok)',
    'var(--hf-ink-2)',
    'var(--hf-accent)',
    'var(--hf-info)',
    'var(--hf-err)',
  ];
  const bubbles = modelSet.map((m, i) => {
    const angle = (i / Math.max(modelSet.length, 1)) * Math.PI * 2;
    const rx = 0.3 + 0.22 * Math.cos(angle);
    const ry = 0.3 + 0.22 * Math.sin(angle);
    return { m, x: Math.max(0.05, Math.min(0.9, rx)), y: Math.max(0.05, Math.min(0.9, ry)), r: 12, c: BUBBLE_COLORS[i % BUBBLE_COLORS.length] };
  });

  // Fallback static bubbles when no log data
  const FALLBACK_BUBBLES = [
    { m: 'gpt-4o-mini', x: 0.18, y: 0.2, r: 14, c: 'var(--hf-ok)' },
    { m: 'claude-3.5-sonnet', x: 0.55, y: 0.65, r: 20, c: 'var(--hf-accent)' },
    { m: 'gemini-1.5-pro', x: 0.52, y: 0.58, r: 11, c: 'var(--hf-info)' },
  ];
  const displayBubbles = bubbles.length > 0 ? bubbles : FALLBACK_BUBBLES;

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

        {/* ── Static KPI cards (no real-time data yet) ── */}
        {STATIC_KPIS.map((k, i) => (
          <div
            key={i}
            className='panel'
            style={{ gridColumn: 'span 4', padding: 18, opacity: 0.65 }}
          >
            <div className='lbl'>{k.l}</div>
            <div className='display' style={{ fontSize: 32, marginTop: 4, color: 'var(--hf-ink-3)' }}>
              —
            </div>
            <div
              style={{
                marginTop: 8,
                fontSize: 10,
                color: 'var(--hf-ink-3)',
                fontFamily: 'var(--hf-mono)',
              }}
            >
              realtime metrics coming soon
            </div>
          </div>
        ))}

        {/* ── Model bubble chart ── */}
        <div className='panel' style={{ gridColumn: 'span 7', padding: 18 }}>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'baseline',
            }}
          >
            <div>
              <div className='lbl'>model usage · recent activity</div>
              <div className='display' style={{ fontSize: 18, marginTop: 2 }}>
                {logs.length > 0 ? 'Models from your last 5 requests' : 'No recent requests'}
              </div>
            </div>
            <span className='faint mono' style={{ fontSize: 10 }}>
              {logs.length > 0 ? 'from recent logs' : 'sample layout'}
            </span>
          </div>
          <div
            style={{
              position: 'relative',
              height: 260,
              marginTop: 18,
              borderLeft: '1px solid var(--hf-rule)',
              borderBottom: '1px solid var(--hf-rule)',
            }}
          >
            {[0.25, 0.5, 0.75].map((p, i) => (
              <Fragment key={i}>
                <div
                  style={{
                    position: 'absolute',
                    left: p * 100 + '%',
                    top: 0,
                    bottom: 0,
                    borderLeft: '1px dashed var(--hf-rule)',
                  }}
                />
                <div
                  style={{
                    position: 'absolute',
                    bottom: p * 100 + '%',
                    left: 0,
                    right: 0,
                    borderTop: '1px dashed var(--hf-rule)',
                  }}
                />
              </Fragment>
            ))}
            {displayBubbles.map((b, i) => (
              <div
                key={i}
                style={{
                  position: 'absolute',
                  left: b.x * 100 + '%',
                  bottom: b.y * 100 + '%',
                  width: b.r * 2,
                  height: b.r * 2,
                  marginLeft: -b.r,
                  marginBottom: -b.r,
                  borderRadius: '50%',
                  background: b.c,
                  opacity: 0.4,
                  border: '1.5px solid ' + b.c,
                }}
              />
            ))}
            {displayBubbles.map((b, i) => (
              <div
                key={i}
                className='mono'
                style={{
                  position: 'absolute',
                  left: 'calc(' + b.x * 100 + '% + ' + (b.r + 6) + 'px)',
                  bottom: b.y * 100 + '%',
                  fontSize: 10,
                  color: 'var(--hf-ink-2)',
                  whiteSpace: 'nowrap',
                }}
              >
                {b.m}
              </div>
            ))}
          </div>
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
          {!loading && logs.length === 0 && (
            <div className='muted' style={{ fontSize: 12 }}>
              No recent requests found.
            </div>
          )}
          {!loading && logs.length > 0 && (
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
              {logs.map((log, i) => {
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
                      borderBottom: i < logs.length - 1 ? '1px dashed var(--hf-rule)' : 0,
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
