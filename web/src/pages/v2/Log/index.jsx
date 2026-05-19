/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import React, { useCallback, useEffect, useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';
import WIPBanner from '../../../components/hifi/WIPBanner';
import { API, showError } from '../../../helpers';

/* Wave 2: Cluster tab wired; Live tail SSE deferred to v3 */

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

  // Cluster tab state
  const [clusterBucket, setClusterBucket] = useState('hour');
  const [clusterItems, setClusterItems] = useState([]);
  const [clusterLoading, setClusterLoading] = useState(false);

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

  const fetchLogs = useCallback(
    async (currentPage, model, token, start, end) => {
      setLoading(true);
      try {
        const params = new URLSearchParams({
          page: String(currentPage),
          page_size: String(PAGE_SIZE),
        });
        if (model) params.set('model_name', model);
        if (token) params.set('token_name', token);
        if (start)
          params.set(
            'start_time',
            String(Math.floor(new Date(start).getTime() / 1000)),
          );
        if (end)
          params.set(
            'end_time',
            String(Math.floor(new Date(end).getTime() / 1000)),
          );

        const res = await API.get(
          `/api/v2/${tenantSlug}/logs?${params.toString()}`,
        );
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
    },
    [tenantSlug],
  );

  // Fetch on mount and whenever tenantSlug changes
  useEffect(() => {
    if (tenantSlug)
      fetchLogs(page, filterModel, filterToken, filterStart, filterEnd);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenantSlug]);

  const fetchCluster = useCallback(
    async (bucket) => {
      setClusterLoading(true);
      try {
        const res = await API.get(
          `/api/v2/${tenantSlug}/logs/cluster?bucket=${bucket}`,
        );
        if (res?.data?.success) {
          const items = res.data.data.items ?? [];
          // Sort descending by count (API returns sorted, but guard client-side)
          items.sort((a, b) => b.count - a.count);
          setClusterItems(items);
        } else {
          showError(res?.data?.message || 'Failed to load cluster data');
        }
      } catch (_) {
        // error toast shown by API interceptor
      } finally {
        setClusterLoading(false);
      }
    },
    [tenantSlug],
  );

  // Fetch cluster when switching to cluster tab or bucket changes
  useEffect(() => {
    if (tab === 'cluster' && tenantSlug) {
      fetchCluster(clusterBucket);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab, clusterBucket, tenantSlug]);

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
        <span className='muted' style={{ fontSize: 11 }}>
          →
        </span>
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
                <div
                  className='muted'
                  style={{ padding: '20px 22px', fontSize: 12 }}
                >
                  Loading…
                </div>
              )}

              {!loading && logs.length === 0 && (
                <div
                  className='muted'
                  style={{ padding: '20px 22px', fontSize: 12 }}
                >
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
                          background:
                            selRow === i ? 'var(--hf-sunken)' : undefined,
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
                          {r.Duration != null && (
                            <span className='faint'>ms</span>
                          )}
                        </td>
                        <td className='mono'>—</td>
                        <td className='strong'>{r.ModelName || '—'}</td>
                        <td className='mono muted'>—</td>
                        <td className='mono muted'>{r.TokenName || '—'}</td>
                        <td className='mono muted'>
                          {fmtTok(r.PromptTokens, r.CompletionTokens)}
                        </td>
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
                    <div className='lbl' style={{ marginBottom: 8 }}>
                      details
                    </div>
                    <div
                      className='panel-paper'
                      style={{
                        padding: 12,
                        fontFamily: 'var(--hf-mono)',
                        fontSize: 11,
                        lineHeight: 1.7,
                      }}
                    >
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
                        {selectedLog.Duration != null
                          ? `${selectedLog.Duration}ms`
                          : '—'}
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
                  <div
                    className='muted'
                    style={{ padding: '20px 22px', fontSize: 12 }}
                  >
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

      {/* ── Cluster tab — wired to GET /api/v2/:slug/logs/cluster ── */}
      {/* Wave 2: Cluster tab wired; Live tail SSE deferred to v3 */}
      {tab === 'cluster' && (
        <div style={{ padding: 24 }}>
          {/* Bucket toggle */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              marginBottom: 16,
            }}
          >
            <span
              className='muted'
              style={{ fontSize: 11, fontFamily: 'var(--hf-mono)' }}
            >
              bucket:
            </span>
            {['hour', 'day'].map((b) => (
              <button
                key={b}
                type='button'
                className={clusterBucket === b ? 'btn primary' : 'btn'}
                style={{ fontSize: 11 }}
                onClick={() => setClusterBucket(b)}
              >
                {b}
              </button>
            ))}
            {clusterLoading && (
              <span
                className='muted'
                style={{ fontSize: 11, fontFamily: 'var(--hf-mono)' }}
              >
                loading…
              </span>
            )}
          </div>

          {/* Cluster table */}
          {!clusterLoading && clusterItems.length === 0 && (
            <div className='muted' style={{ padding: '20px 0', fontSize: 12 }}>
              No cluster data for selected period.
            </div>
          )}

          {clusterItems.length > 0 && (
            <table className='t' data-testid='cluster-table'>
              <thead>
                <tr>
                  <th>model</th>
                  <th>error code</th>
                  <th>bucket</th>
                  <th>count</th>
                </tr>
              </thead>
              <tbody>
                {clusterItems.map((row, i) => {
                  const code = row.error_code || '';
                  const is5xx = /^5/.test(code);
                  const is4xx = /^4/.test(code);
                  return (
                    <tr key={i}>
                      <td className='strong'>{row.model_name || '—'}</td>
                      <td>
                        {code ? (
                          <span
                            className={
                              is5xx ? 'tag error' : is4xx ? 'tag warn' : 'tag'
                            }
                          >
                            {code}
                          </span>
                        ) : (
                          <span className='muted'>—</span>
                        )}
                      </td>
                      <td className='mono muted' style={{ fontSize: 10 }}>
                        {row.bucket}
                      </td>
                      <td className='mono'>{row.count}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
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
            style={{
              marginTop: 14,
              padding: 24,
              textAlign: 'center',
              color: 'var(--hf-ink-3)',
              fontFamily: 'var(--hf-mono)',
              fontSize: 12,
            }}
          >
            No live-tail data — streaming endpoint not implemented.
          </div>
        </div>
      )}
    </HFShell>
  );
};

export default HFLog;
