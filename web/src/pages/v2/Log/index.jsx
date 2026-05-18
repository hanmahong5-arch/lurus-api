import React, { useCallback, useEffect, useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';
import WIPBanner from '../../../components/hifi/WIPBanner';
import { API, showError } from '../../../helpers';

/*
 * v2 Log page — wired to GET /api/v2/:tenant_slug/logs.
 * TTFT and upstream channel are not returned by the API; displayed as —.
 * Cluster/Live tail tabs render WIPBanner — backend aggregation/streaming
 * endpoints don't exist yet (see hardening-swarm-2026-05-18-acceptance.md).
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
          ['cluster', 'Error clusters', '—'],
          ['live', 'Live tail', '—'],
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

      {/* ── Cluster tab — no aggregation endpoint yet ── */}
      {tab === 'cluster' && (
        <div style={{ padding: 24 }}>
          <WIPBanner
            reason='Error clusters need a backend aggregation endpoint that groups logs by error signature. None exists.'
            todo='Backend: /api/v2/{slug}/logs/clusters (group by error_signature + tenant counts + trend).'
          />
          <div
            className='panel'
            style={{ marginTop: 14, padding: 24, textAlign: 'center', color: 'var(--hf-ink-3)', fontFamily: 'var(--hf-mono)', fontSize: 12 }}
          >
            No error-cluster data — endpoint not implemented.
          </div>
        </div>
      )}

      {/* ── Live tail tab — no streaming endpoint yet ── */}
      {tab === 'live' && (
        <div style={{ padding: 24 }}>
          <WIPBanner
            reason='Live tail needs either an SSE/WebSocket stream or a high-frequency polling cursor against /logs. Neither is wired.'
            todo='Backend: /api/v2/{slug}/logs/stream (SSE) OR cursor param on /logs; UI wires after.'
          />
          <div
            className='panel'
            style={{ marginTop: 14, padding: 24, textAlign: 'center', color: 'var(--hf-ink-3)', fontFamily: 'var(--hf-mono)', fontSize: 12 }}
          >
            No live-tail data — streaming endpoint not implemented.
          </div>
        </div>
      )}
    </HFShell>
  );
};

export default HFLog;
