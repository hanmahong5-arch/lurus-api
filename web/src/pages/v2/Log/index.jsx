import React, { useCallback, useEffect, useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';
import { API, showError } from '../../../helpers';

/*
 * v2 Log page — wired to GET /api/v2/:tenant_slug/logs.
 * TTFT and upstream channel are not returned by the API; displayed as —.
 */

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

const fmtTime = (unixSec) => {
  if (!unixSec) return '—';
  const d = new Date(unixSec * 1000);
  return d.toLocaleTimeString('en-GB', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    fractionalSecondDigits: 3,
  });
};

const fmtTok = (prompt, completion) => {
  const p = prompt ? `${(prompt / 1000).toFixed(1)}k` : '0';
  const c = completion ? `${(completion / 1000).toFixed(1)}k` : '—';
  return `${p}→${c}`;
};

const fmtCost = (quota) => {
  if (!quota) return '—';
  const usd = quota / QUOTA_PER_USD;
  return `$${usd.toFixed(4)}`;
};

// Static data kept for cluster/live tabs (no backend endpoints for these yet)
const CLUSTERS = [
  {
    count: 142,
    sig: 'upstream_timeout',
    model: 'gpt-4o',
    up: 'openai/main',
    last: '12s',
    trend: [3, 5, 8, 9, 12, 18, 22, 28, 35, 42, 38, 40, 36, 34],
    tenants: 6,
  },
  {
    count: 38,
    sig: 'rate_limit_429',
    model: 'claude-3-opus',
    up: 'anthropic/eu',
    last: '1m',
    trend: [1, 2, 2, 3, 4, 4, 5, 6, 5, 4, 4, 3, 3, 3],
    tenants: 2,
  },
  {
    count: 17,
    sig: 'invalid_api_key',
    model: 'gemini-1.5-pro',
    up: 'vertex/asia',
    last: '4m',
    trend: [0, 0, 1, 2, 3, 4, 4, 3, 2, 1, 0, 0, 1, 1],
    tenants: 1,
  },
  {
    count: 9,
    sig: 'context_overflow',
    model: 'gpt-4o-mini',
    up: 'openai/main',
    last: '11m',
    trend: [0, 1, 1, 1, 2, 1, 1, 1, 0, 1, 0, 1, 0, 0],
    tenants: 3,
  },
];

const LIVE_ROWS = [
  ['14:02:11.481', '200', 'gpt-4o-mini', '847t', '$0.0042', 'acme'],
  ['14:02:11.122', '200', 'claude-3.5-sonnet', '312t', '$0.0090', 'contoso'],
  ['14:02:10.844', '429', 'claude-3-opus', '—', '—', 'initech'],
  ['14:02:10.512', '200', 'gemini-1.5-pro', '621t', '$0.0031', 'globex'],
  ['14:02:10.001', '504', 'gpt-4o', '—', '—', 'acme'],
  ['14:02:09.770', '200', 'gpt-4o-mini', '210t', '$0.0011', 'acme'],
  ['14:02:09.422', '200', 'claude-3.5-sonnet', '480t', '$0.0072', 'contoso'],
  ['14:02:08.991', '200', 'gemini-1.5-flash', '112t', '$0.0004', 'globex'],
];

const PAGE_SIZE = 50;

const HFLog = () => {
  const tenantSlug = useTenantSlug();

  const [tab, setTab] = useState('trace');
  const [selRow, setSelRow] = useState(0);

  // Logs state
  const [logs, setLogs] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);

  // Filter state
  const [filterModel, setFilterModel] = useState('');
  const [filterToken, setFilterToken] = useState('');
  const [filterStart, setFilterStart] = useState('');
  const [filterEnd, setFilterEnd] = useState('');

  const fetchLogs = useCallback(async (currentPage, model, token, start, end) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        page: String(currentPage),
        page_size: String(PAGE_SIZE),
      });
      if (model) params.set('model_name', model);
      if (token) params.set('token_name', token);
      if (start) params.set('start_time', String(Math.floor(new Date(start).getTime() / 1000)));
      if (end) params.set('end_time', String(Math.floor(new Date(end).getTime() / 1000)));

      const res = await API.get(`/api/v2/${tenantSlug}/logs?${params.toString()}`);
      if (res?.data?.success) {
        const d = res.data.data;
        setLogs(d.logs ?? []);
        setTotal(d.total ?? 0);
        setSelRow(0);
      } else {
        showError(res?.data?.message || 'Failed to load logs');
      }
    } catch (_) {
      // error toast shown by API interceptor
    } finally {
      setLoading(false);
    }
  }, [tenantSlug]);

  // Fetch on mount and whenever tenantSlug changes
  useEffect(() => {
    if (tenantSlug) fetchLogs(page, filterModel, filterToken, filterStart, filterEnd);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenantSlug]);

  const applyFilters = () => {
    setPage(1);
    fetchLogs(1, filterModel, filterToken, filterStart, filterEnd);
  };

  const goPage = (next) => {
    setPage(next);
    fetchLogs(next, filterModel, filterToken, filterStart, filterEnd);
  };

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const selectedLog = logs[selRow];

  const inputStyle = {
    fontFamily: 'var(--hf-mono)',
    fontSize: 11,
    padding: '4px 8px',
    border: '1px solid var(--hf-rule)',
    background: 'var(--hf-sunken)',
    color: 'var(--hf-ink)',
    borderRadius: 2,
    outline: 'none',
  };

  return (
    <HFShell
      active='logs'
      crumbs={['my account', 'usage & logs']}
      actions={
        <>
          <span className='muted mono' style={{ fontSize: 11 }}>
            {loading ? 'loading…' : `${total} requests`}
          </span>
        </>
      }
    >
      {/* Filter bar */}
      <div
        style={{
          padding: '10px 28px',
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          borderBottom: '1px solid var(--hf-rule)',
          background: 'var(--hf-paper)',
          flexWrap: 'wrap',
        }}
      >
        <input
          style={{ ...inputStyle, width: 160 }}
          placeholder='model name…'
          value={filterModel}
          onChange={(e) => setFilterModel(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && applyFilters()}
        />
        <input
          style={{ ...inputStyle, width: 140 }}
          placeholder='token name…'
          value={filterToken}
          onChange={(e) => setFilterToken(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && applyFilters()}
        />
        <input
          style={{ ...inputStyle, width: 160 }}
          type='datetime-local'
          title='start time'
          value={filterStart}
          onChange={(e) => setFilterStart(e.target.value)}
        />
        <span className='muted' style={{ fontSize: 11 }}>→</span>
        <input
          style={{ ...inputStyle, width: 160 }}
          type='datetime-local'
          title='end time'
          value={filterEnd}
          onChange={(e) => setFilterEnd(e.target.value)}
        />
        <button type='button' className='btn primary' onClick={applyFilters}>
          search
        </button>
        <button
          type='button'
          className='btn ghost'
          onClick={() => {
            setFilterModel('');
            setFilterToken('');
            setFilterStart('');
            setFilterEnd('');
            setPage(1);
            fetchLogs(1, '', '', '', '');
          }}
        >
          clear
        </button>
      </div>

      {/* Tabs */}
      <div
        style={{
          display: 'flex',
          padding: '0 28px',
          borderBottom: '1px solid var(--hf-rule)',
          background: 'var(--hf-paper)',
        }}
      >
        {[
          ['trace', 'Requests', total || ''],
          ['cluster', 'Error clusters', CLUSTERS.length],
          ['live', 'Live tail', '⏵'],
        ].map(([k, l, c]) => (
          <button
            key={k}
            type='button'
            onClick={() => setTab(k)}
            style={{
              padding: '12px 16px',
              background: 'transparent',
              border: 0,
              cursor: 'pointer',
              fontFamily: 'var(--hf-mono)',
              fontSize: 11,
              color: tab === k ? 'var(--hf-ink)' : 'var(--hf-ink-3)',
              borderBottom:
                tab === k
                  ? '2px solid var(--hf-accent)'
                  : '2px solid transparent',
              marginBottom: -1,
              display: 'flex',
              gap: 8,
              alignItems: 'center',
            }}
          >
            {l}
            <span style={{ fontSize: 9, color: 'var(--hf-ink-4)' }}>{c}</span>
          </button>
        ))}
      </div>

      {/* ── Trace tab ── */}
      {tab === 'trace' && (
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            height: 'calc(100vh - 270px)',
          }}
        >
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '1.5fr 1fr',
              minHeight: 0,
              flex: 1,
            }}
          >
            {/* Log table */}
            <div
              style={{
                borderRight: '1px solid var(--hf-rule)',
                overflow: 'auto',
              }}
            >
              {loading && (
                <div className='muted' style={{ padding: '20px 22px', fontSize: 12 }}>
                  Loading…
                </div>
              )}

              {!loading && logs.length === 0 && (
                <div className='muted' style={{ padding: '20px 22px', fontSize: 12 }}>
                  No logs found.
                </div>
              )}

              {!loading && logs.length > 0 && (
                <table className='t'>
                  <thead>
                    <tr>
                      <th>timestamp</th>
                      <th>dur</th>
                      <th>ttft</th>
                      <th>model</th>
                      <th>upstream</th>
                      <th>token</th>
                      <th>tok</th>
                      <th>$</th>
                      <th>code</th>
                    </tr>
                  </thead>
                  <tbody>
                    {logs.map((r, i) => (
                      <tr
                        key={r.Id ?? i}
                        onClick={() => setSelRow(i)}
                        style={{
                          background: selRow === i ? 'var(--hf-sunken)' : undefined,
                          cursor: 'pointer',
                          borderLeft:
                            selRow === i
                              ? '2px solid var(--hf-accent)'
                              : '2px solid transparent',
                        }}
                      >
                        <td className='mono muted'>{fmtTime(r.CreatedAt)}</td>
                        <td className='mono'>
                          {r.Duration ?? '—'}
                          {r.Duration != null && <span className='faint'>ms</span>}
                        </td>
                        <td className='mono'>—</td>
                        <td className='strong'>{r.ModelName || '—'}</td>
                        <td className='mono muted'>—</td>
                        <td className='mono muted'>{r.TokenName || '—'}</td>
                        <td className='mono muted'>{fmtTok(r.PromptTokens, r.CompletionTokens)}</td>
                        <td className='mono'>{fmtCost(r.Quota)}</td>
                        <td>
                          <span className='tag ok'>200</span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>

            {/* Detail panel */}
            <div style={{ overflow: 'auto', background: 'var(--hf-paper)' }}>
              {selectedLog ? (
                <>
                  <div
                    style={{
                      padding: '20px 22px',
                      borderBottom: '1px solid var(--hf-rule)',
                    }}
                  >
                    <div className='lbl' style={{ marginBottom: 4 }}>
                      request · {fmtTime(selectedLog.CreatedAt)}
                    </div>
                    <div className='display' style={{ fontSize: 19 }}>
                      {selectedLog.ModelName || '—'}
                    </div>
                    <div
                      style={{
                        display: 'flex',
                        gap: 8,
                        marginTop: 8,
                        alignItems: 'center',
                        flexWrap: 'wrap',
                      }}
                    >
                      <span className='tag ok'>200</span>
                      {selectedLog.ModelName && (
                        <span className='pill'>{selectedLog.ModelName}</span>
                      )}
                      {selectedLog.Duration != null && (
                        <span className='pill'>{selectedLog.Duration}ms</span>
                      )}
                      {selectedLog.IsStream && (
                        <span className='pill'>stream</span>
                      )}
                    </div>
                  </div>

                  <div style={{ padding: 20 }}>
                    <div className='lbl' style={{ marginBottom: 8 }}>details</div>
                    <div className='panel-paper' style={{ padding: 12, fontFamily: 'var(--hf-mono)', fontSize: 11, lineHeight: 1.7 }}>
                      <div>
                        <span className='muted'>model:</span>{' '}
                        {selectedLog.ModelName || '—'}
                      </div>
                      <div>
                        <span className='muted'>token name:</span>{' '}
                        {selectedLog.TokenName || '—'}
                      </div>
                      <div>
                        <span className='muted'>prompt tokens:</span>{' '}
                        {selectedLog.PromptTokens ?? '—'}
                      </div>
                      <div>
                        <span className='muted'>completion tokens:</span>{' '}
                        {selectedLog.CompletionTokens ?? '—'}
                      </div>
                      <div>
                        <span className='muted'>cost:</span>{' '}
                        {fmtCost(selectedLog.Quota)}
                      </div>
                      <div>
                        <span className='muted'>duration:</span>{' '}
                        {selectedLog.Duration != null ? `${selectedLog.Duration}ms` : '—'}
                      </div>
                      <div>
                        <span className='muted'>streaming:</span>{' '}
                        {selectedLog.IsStream ? 'yes' : 'no'}
                      </div>
                      {selectedLog.Content && (
                        <div>
                          <span className='muted'>note:</span>{' '}
                          {selectedLog.Content}
                        </div>
                      )}
                    </div>
                  </div>
                </>
              ) : (
                !loading && (
                  <div className='muted' style={{ padding: '20px 22px', fontSize: 12 }}>
                    Select a row to view details.
                  </div>
                )
              )}
            </div>
          </div>

          {/* Pagination */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 12,
              padding: '10px 28px',
              borderTop: '1px solid var(--hf-rule)',
              background: 'var(--hf-paper)',
              fontSize: 12,
            }}
          >
            <button
              type='button'
              className='btn'
              disabled={page <= 1 || loading}
              onClick={() => goPage(page - 1)}
            >
              ← prev
            </button>
            <span className='mono muted'>
              page {page} of {totalPages}
            </span>
            <button
              type='button'
              className='btn'
              disabled={page >= totalPages || loading}
              onClick={() => goPage(page + 1)}
            >
              next →
            </button>
            <span className='muted' style={{ marginLeft: 'auto' }}>
              {total} total · {PAGE_SIZE} per page
            </span>
          </div>
        </div>
      )}

      {/* ── Cluster tab (static mockup) ── */}
      {tab === 'cluster' && (
        <div style={{ padding: 24 }}>
          <div className='lbl' style={{ marginBottom: 10 }}>
            error clusters · demo data
          </div>
          <div className='panel'>
            <table className='t'>
              <thead>
                <tr>
                  <th>count</th>
                  <th>signature</th>
                  <th>model</th>
                  <th>upstream</th>
                  <th>tenants</th>
                  <th>trend</th>
                  <th>last seen</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {CLUSTERS.map((c, i) => (
                  <tr key={i}>
                    <td>
                      <span
                        className='display'
                        style={{
                          fontSize: 22,
                          color: i === 0 ? 'var(--hf-err)' : 'var(--hf-ink)',
                        }}
                      >
                        {c.count}
                      </span>
                    </td>
                    <td>
                      <span className='tag err'>{c.sig}</span>
                    </td>
                    <td className='strong'>{c.model}</td>
                    <td className='mono muted'>{c.up}</td>
                    <td className='mono'>{c.tenants}</td>
                    <td>
                      <span className='spark err' style={{ height: 28 }}>
                        {c.trend.map((v, j) => (
                          <i key={j} style={{ height: Math.max(2, v * 1.0) + 'px' }} />
                        ))}
                      </span>
                    </td>
                    <td className='muted'>{c.last} ago</td>
                    <td>
                      <button type='button' className='btn ghost'>
                        inspect →
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* ── Live tail tab (static mockup) ── */}
      {tab === 'live' && (
        <div style={{ padding: 24, height: 'calc(100vh - 270px)' }}>
          <div className='lbl' style={{ marginBottom: 8 }}>
            live tail · demo data
          </div>
          <div
            style={{
              background: '#0a0908',
              color: '#e8e3d4',
              padding: 18,
              height: '100%',
              overflow: 'auto',
              fontFamily: 'var(--hf-mono)',
              fontSize: 11,
              lineHeight: 1.65,
              border: '1px solid var(--hf-rule-strong)',
            }}
          >
            {LIVE_ROWS.map((r, i) => (
              <div key={i}>
                <span style={{ color: '#807a6a' }}>{r[0]}</span>
                {'  '}
                <span
                  style={{
                    color: r[1].startsWith('2')
                      ? '#5acc92'
                      : r[1].startsWith('4')
                        ? '#e0a040'
                        : '#ee6f5e',
                  }}
                >
                  {r[1]}
                </span>
                {'  '}
                <span
                  style={{
                    color: '#f4f1e8',
                    display: 'inline-block',
                    width: 180,
                  }}
                >
                  {r[2]}
                </span>
                <span
                  style={{
                    color: '#cfcbbd',
                    display: 'inline-block',
                    width: 60,
                    textAlign: 'right',
                  }}
                >
                  {r[3]}
                </span>
                {'  '}
                <span
                  style={{
                    color: '#807a6a',
                    display: 'inline-block',
                    width: 80,
                  }}
                >
                  {r[4]}
                </span>
                {'  '}
                <span style={{ color: '#a89c80' }}>{r[5]}</span>
              </div>
            ))}
            <div style={{ color: '#ff7a3a' }}>▌</div>
          </div>
        </div>
      )}
    </HFShell>
  );
};

export default HFLog;
