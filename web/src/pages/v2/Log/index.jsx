import React, { useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';

/*
 * HiFi 1 — Log Trace + Error Clustering.
 * Ported from design canvas hifi/hf1-log.jsx (2026-05-07).
 * Data is hardcoded — wire to real /api/log endpoints in a follow-up story.
 */

const ROWS = [
  {
    ts: '14:02:11.481',
    dur: 1843,
    ttft: 412,
    model: 'gpt-4o-mini',
    up: 'openai/main',
    tok: '1.2k→847',
    cost: 0.0042,
    status: 200,
    tenant: 'acme',
  },
  {
    ts: '14:02:09.220',
    dur: 922,
    ttft: 188,
    model: 'claude-3.5-sonnet',
    up: 'anthropic/eu',
    tok: '0.8k→312',
    cost: 0.009,
    status: 200,
    tenant: 'contoso',
  },
  {
    ts: '14:02:07.014',
    dur: 4810,
    ttft: null,
    model: 'gpt-4o',
    up: 'openai/main',
    tok: '2.4k→ —',
    cost: 0,
    status: 502,
    tenant: 'acme',
  },
  {
    ts: '14:02:05.901',
    dur: 1102,
    ttft: 295,
    model: 'gemini-1.5-pro',
    up: 'vertex/asia',
    tok: '0.9k→621',
    cost: 0.0031,
    status: 200,
    tenant: 'globex',
  },
  {
    ts: '14:02:03.555',
    dur: 680,
    ttft: 140,
    model: 'gpt-4o-mini',
    up: 'openai/main',
    tok: '0.4k→210',
    cost: 0.0011,
    status: 200,
    tenant: 'acme',
  },
  {
    ts: '14:02:01.118',
    dur: 6021,
    ttft: null,
    model: 'gpt-4o',
    up: 'openai/main',
    tok: '3.1k→ —',
    cost: 0,
    status: 504,
    tenant: 'acme',
  },
  {
    ts: '14:01:58.802',
    dur: 1521,
    ttft: 322,
    model: 'claude-3.5-sonnet',
    up: 'anthropic/eu',
    tok: '1.0k→480',
    cost: 0.0072,
    status: 200,
    tenant: 'contoso',
  },
  {
    ts: '14:01:56.011',
    dur: 411,
    ttft: 98,
    model: 'gemini-1.5-flash',
    up: 'vertex/asia',
    tok: '0.3k→112',
    cost: 0.0004,
    status: 200,
    tenant: 'globex',
  },
  {
    ts: '14:01:54.220',
    dur: 2102,
    ttft: 510,
    model: 'gpt-4o',
    up: 'openai/main',
    tok: '1.8k→680',
    cost: 0.0118,
    status: 429,
    tenant: 'initech',
  },
];

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

const WATERFALL = [
  { l: 'gateway', a: 0, w: 18, c: 'var(--hf-ink-3)', t: '18ms' },
  { l: 'auth', a: 18, w: 8, c: 'var(--hf-ink-3)', t: '8ms' },
  { l: 'router', a: 26, w: 14, c: 'var(--hf-ink-3)', t: '14ms' },
  { l: 'dns', a: 40, w: 32, c: 'var(--hf-info)', t: '32ms' },
  { l: 'tcp+tls', a: 72, w: 88, c: 'var(--hf-info)', t: '88ms' },
  { l: 'upstream', a: 160, w: 410, c: 'var(--hf-err)', t: '410ms · timeout' },
  { l: 'meter+log', a: 570, w: 22, c: 'var(--hf-ink-3)', t: '22ms' },
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

const HFLog = () => {
  const [tab, setTab] = useState('trace');
  const [selRow, setSelRow] = useState(2);
  const [filters, setFilters] = useState(['model:gpt-4o', 'status:5xx']);

  return (
    <HFShell
      active='logs'
      crumbs={['my account', 'usage & logs']}
      actions={
        <>
          <span className='lbl'>
            <span
              className='live-dot'
              style={{
                display: 'inline-block',
                verticalAlign: 'middle',
                marginRight: 6,
              }}
            />
            live · 2s
          </span>
          <button type='button' className='btn'>
            last 1h ▾
          </button>
          <button type='button' className='btn primary'>
            export
          </button>
        </>
      }
    >
      <div className='hf-page-head'>
        <div>
          <div className='lbl' style={{ marginBottom: 6 }}>
            requests
          </div>
          <h1>
            206 errors{' '}
            <span className='muted' style={{ fontWeight: 400 }}>
              across 12,481 requests
            </span>
          </h1>
          <div className='sub'>
            streaming · last 1 hour · meilisearch indexed
          </div>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 28 }}>
          {[
            ['avg ttft', '412ms', 'var(--hf-ok)'],
            ['p95 dur', '2.10s', 'var(--hf-ink)'],
            ['error rate', '1.65%', 'var(--hf-err)'],
            ['hot model', 'gpt-4o', 'var(--hf-ink)'],
          ].map(([l, v, c], i) => (
            <div key={i}>
              <div className='lbl'>{l}</div>
              <div
                className='display'
                style={{ fontSize: 22, color: c, marginTop: 2 }}
              >
                {v}
              </div>
            </div>
          ))}
        </div>
      </div>

      <div
        style={{
          padding: '14px 28px',
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          borderBottom: '1px solid var(--hf-rule)',
          background: 'var(--hf-paper)',
        }}
      >
        <div className='field' style={{ flex: 1, height: 34 }}>
          <span className='faint'>⌕</span>
          {filters.map((f, i) => (
            <span
              key={i}
              className='pill'
              style={{ background: 'var(--hf-sunken)' }}
            >
              {f}
              <button
                type='button'
                onClick={() => setFilters(filters.filter((_, j) => j !== i))}
                style={{
                  border: 0,
                  background: 'transparent',
                  cursor: 'pointer',
                  color: 'var(--hf-ink-3)',
                  padding: 0,
                  marginLeft: 2,
                }}
              >
                ×
              </button>
            </span>
          ))}
          <span style={{ color: 'var(--hf-ink-4)' }}>add filter…</span>
          <span style={{ marginLeft: 'auto', color: 'var(--hf-ink-4)' }}>
            meili · 12ms
          </span>
        </div>
        <button type='button' className='btn'>
          save query
        </button>
      </div>

      <div
        style={{
          display: 'flex',
          padding: '0 28px',
          borderBottom: '1px solid var(--hf-rule)',
          background: 'var(--hf-paper)',
        }}
      >
        {[
          ['trace', 'Trace stream', '12,481'],
          ['cluster', 'Error clusters', '4'],
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

      {tab === 'trace' && (
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '1.5fr 1fr',
            minHeight: 0,
            height: 'calc(100vh - 296px)',
          }}
        >
          <div
            style={{
              borderRight: '1px solid var(--hf-rule)',
              overflow: 'auto',
            }}
          >
            <table className='t'>
              <thead>
                <tr>
                  <th>timestamp</th>
                  <th>dur</th>
                  <th>ttft</th>
                  <th>model</th>
                  <th>upstream</th>
                  <th>tenant</th>
                  <th>tok</th>
                  <th>$</th>
                  <th>code</th>
                </tr>
              </thead>
              <tbody>
                {ROWS.map((r, i) => (
                  <tr
                    key={i}
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
                    <td className='mono muted'>{r.ts}</td>
                    <td className='mono'>
                      {r.dur}
                      <span className='faint'>ms</span>
                    </td>
                    <td className='mono'>{r.ttft ? r.ttft + 'ms' : '—'}</td>
                    <td className='strong'>{r.model}</td>
                    <td className='mono muted'>{r.up}</td>
                    <td className='mono'>{r.tenant}</td>
                    <td className='mono muted'>{r.tok}</td>
                    <td className='mono'>
                      {r.cost ? '$' + r.cost.toFixed(4) : '—'}
                    </td>
                    <td>
                      <span
                        className={
                          'tag ' +
                          (r.status >= 500
                            ? 'err'
                            : r.status >= 400
                              ? 'warn'
                              : 'ok')
                        }
                      >
                        {r.status}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div style={{ overflow: 'auto', background: 'var(--hf-paper)' }}>
            <div
              style={{
                padding: '20px 22px',
                borderBottom: '1px solid var(--hf-rule)',
              }}
            >
              <div className='lbl' style={{ marginBottom: 4 }}>
                request · {ROWS[selRow].ts}
              </div>
              <div className='display' style={{ fontSize: 19 }}>
                POST /v1/chat/completions
              </div>
              <div
                style={{
                  display: 'flex',
                  gap: 8,
                  marginTop: 8,
                  alignItems: 'center',
                }}
              >
                <span
                  className={
                    'tag ' +
                    (ROWS[selRow].status >= 500
                      ? 'err'
                      : ROWS[selRow].status >= 400
                        ? 'warn'
                        : 'ok')
                  }
                >
                  {ROWS[selRow].status}
                </span>
                <span className='pill'>{ROWS[selRow].model}</span>
                <span className='pill'>{ROWS[selRow].dur}ms</span>
                <span className='muted mono' style={{ marginLeft: 'auto' }}>
                  req_1f4a...e90c
                </span>
              </div>
            </div>

            <div style={{ padding: 20 }}>
              <div className='lbl' style={{ marginBottom: 8 }}>
                waterfall
              </div>
              <div className='panel-paper' style={{ padding: 12 }}>
                {WATERFALL.map((b, i) => (
                  <div
                    key={i}
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '74px 1fr 110px',
                      alignItems: 'center',
                      height: 22,
                      fontSize: 10,
                    }}
                  >
                    <span className='mono muted'>{b.l}</span>
                    <span
                      style={{
                        position: 'relative',
                        height: 8,
                        background: 'transparent',
                      }}
                    >
                      <span
                        style={{
                          position: 'absolute',
                          left: 0,
                          right: 0,
                          top: 3,
                          borderTop: '1px dashed var(--hf-rule)',
                        }}
                      />
                      <span
                        style={{
                          position: 'absolute',
                          left: (b.a / 600) * 100 + '%',
                          width: (b.w / 600) * 100 + '%',
                          height: 8,
                          background: b.c,
                          borderRadius: 1,
                        }}
                      />
                    </span>
                    <span className='mono muted' style={{ textAlign: 'right' }}>
                      {b.t}
                    </span>
                  </div>
                ))}
              </div>

              <div className='lbl' style={{ margin: '16px 0 8px' }}>
                request transformation · client → upstream
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
                  <span className='muted'>model:</span> gpt-4o{' '}
                  <span className='faint'>→</span>{' '}
                  <span className='acc'>gpt-4o-2024-08-06</span>{' '}
                  <span className='tag info' style={{ marginLeft: 4 }}>
                    aliased
                  </span>
                </div>
                <div>
                  <span className='muted'>temperature:</span>{' '}
                  <span className='faint'>—</span>{' '}
                  <span className='faint'>→</span> 0.7{' '}
                  <span className='tag warn' style={{ marginLeft: 4 }}>
                    injected
                  </span>
                </div>
                <div>
                  <span className='muted'>max_tokens:</span> 4096
                </div>
                <div>
                  <span className='muted'>messages:</span> 12 entries · 2,408
                  tokens
                </div>
                <div>
                  <span className='muted'>headers:</span>{' '}
                  <span className='faint'>
                    + x-tenant-id, + x-trace-id, − authorization
                  </span>
                </div>
              </div>

              <div className='lbl' style={{ margin: '16px 0 8px' }}>
                error
              </div>
              <pre
                className='panel-paper mono'
                style={{
                  margin: 0,
                  padding: 12,
                  fontSize: 10,
                  color: 'var(--hf-err)',
                  whiteSpace: 'pre-wrap',
                }}
              >
                {`{
  "error": {
    "type": "upstream_timeout",
    "code": 504,
    "message": "upstream did not respond within 60s",
    "upstream": "api.openai.com",
    "trace_id": "1f4a...e90c"
  }
}`}
              </pre>

              <div style={{ display: 'flex', gap: 6, marginTop: 14 }}>
                <button type='button' className='btn'>
                  copy as cURL
                </button>
                <button type='button' className='btn'>
                  replay
                </button>
                <button type='button' className='btn'>
                  open in playground
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {tab === 'cluster' && (
        <div style={{ padding: 24 }}>
          <div className='lbl' style={{ marginBottom: 10 }}>
            206 errors · 4 clusters · last 1h
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
                          <i
                            key={j}
                            style={{ height: Math.max(2, v * 1.0) + 'px' }}
                          />
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

          <div className='lbl' style={{ margin: '24px 0 10px' }}>
            cluster #1 · upstream_timeout · 142 events · 6 tenants affected
          </div>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '1.4fr 1fr',
              gap: 14,
            }}
          >
            <div className='panel' style={{ padding: 16 }}>
              <div className='lbl'>representative payload</div>
              <pre
                className='mono'
                style={{
                  margin: '8px 0 0',
                  fontSize: 10,
                  color: 'var(--hf-ink-2)',
                  whiteSpace: 'pre-wrap',
                }}
              >
                {`HTTP/1.1 504 Gateway Timeout
content-type: application/json
x-trace-id: 1f4a-e90c-…
x-upstream: api.openai.com

{
  "error": {
    "type": "upstream_timeout",
    "message": "upstream did not respond within 60s"
  }
}`}
              </pre>
            </div>
            <div className='panel' style={{ padding: 16 }}>
              <div className='lbl'>impact</div>
              <ul
                style={{
                  margin: '8px 0 0',
                  paddingLeft: 16,
                  fontSize: 12,
                  color: 'var(--hf-ink-2)',
                }}
              >
                <li>
                  tenants: <b>acme</b>, contoso, foobar{' '}
                  <span className='faint'>(+3)</span>
                </li>
                <li>tokens: 23 unique</li>
                <li>
                  first seen: <b>14:01:55</b> · last: <b>14:02:09</b>
                </li>
                <li>
                  retry success:{' '}
                  <span style={{ color: 'var(--hf-ok)' }}>62%</span> on fallback
                  (azure/eu)
                </li>
                <li>
                  est. revenue impact: <b>$2.40</b>
                </li>
              </ul>
              <div
                style={{
                  display: 'flex',
                  gap: 6,
                  marginTop: 16,
                  flexWrap: 'wrap',
                }}
              >
                <button type='button' className='btn acc'>
                  route around
                </button>
                <button type='button' className='btn'>
                  silence 1h
                </button>
                <button type='button' className='btn'>
                  create incident
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {tab === 'live' && (
        <div style={{ padding: 24, height: 'calc(100vh - 296px)' }}>
          <div className='lbl' style={{ marginBottom: 8 }}>
            live tail · streaming
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
