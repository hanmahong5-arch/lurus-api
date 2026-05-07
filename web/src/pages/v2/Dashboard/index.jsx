import React, { Fragment, useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';

/*
 * HiFi 3 — Dashboard real-time + cost drilldown.
 * Ported from design canvas hifi/hf3-dashboard.jsx (2026-05-07).
 */

const BUBBLES = [
  { m: 'gpt-4o-mini', x: 0.18, y: 0.2, r: 14, c: 'var(--hf-ok)' },
  { m: 'claude-3.5-haiku', x: 0.22, y: 0.3, r: 8, c: 'var(--hf-ok)' },
  { m: 'gemini-1.5-flash', x: 0.14, y: 0.1, r: 6, c: 'var(--hf-ok)' },
  { m: 'gpt-4o', x: 0.55, y: 0.65, r: 14, c: 'var(--hf-accent)' },
  { m: 'claude-3.5-sonnet', x: 0.68, y: 0.72, r: 20, c: 'var(--hf-accent)' },
  { m: 'gemini-1.5-pro', x: 0.52, y: 0.58, r: 11, c: 'var(--hf-info)' },
  { m: 'gpt-4o-realtime', x: 0.85, y: 0.92, r: 6, c: 'var(--hf-err)' },
  { m: 'claude-3-opus', x: 0.88, y: 0.95, r: 5, c: 'var(--hf-err)' },
];

const KPIS = [
  {
    l: 'qps',
    v: '398',
    d: '+12.4%',
    t: [4, 5, 7, 6, 8, 9, 11, 10, 12, 11, 13, 12, 14],
    c: 'var(--hf-accent)',
  },
  {
    l: 'ttft p50',
    v: '412ms',
    d: '−8ms',
    t: [9, 10, 9, 8, 9, 8, 8, 7, 8, 7, 8, 8, 7],
    c: 'var(--hf-ok)',
  },
  {
    l: 'error rate',
    v: '0.81%',
    d: '+0.22pp',
    t: [2, 3, 2, 3, 4, 5, 6, 8, 9, 11, 12, 10, 8],
    c: 'var(--hf-err)',
  },
  {
    l: 'spend / hr',
    v: '$1,051',
    d: '+$48',
    t: [6, 6, 7, 8, 8, 9, 10, 10, 11, 10, 11, 12, 11],
    c: 'var(--hf-ink)',
  },
];

const SIGNALS = [
  {
    sev: 'err',
    t: 'openai/backup down · 12m',
    s: '0% success · failover engaged',
  },
  {
    sev: 'warn',
    t: 'vertex/asia error rate ↑12%',
    s: 'sustained 8m · upstream_timeout',
  },
  {
    sev: 'warn',
    t: 'tenant `acme` near monthly cap',
    s: '$8,420 / $10,000 · projected exceed in 3.2d',
  },
  {
    sev: 'info',
    t: 'gpt-4o-2024-11-20 newly available',
    s: 'auto-add to openai/main · review',
  },
];

const DRILL_COLS = [
  {
    lvl: 'tenants',
    items: [
      ['acme', 8420, 'var(--hf-accent)'],
      ['contoso', 6210, 'var(--hf-info)'],
      ['globex', 4180, 'var(--hf-ok)'],
      ['initech', 3122, 'var(--hf-ink-2)'],
      ['+9 more', 2250, 'var(--hf-ink-3)'],
    ],
  },
  {
    lvl: 'models',
    items: [
      ['gpt-4o', 9120, 'var(--hf-accent)'],
      ['claude-3.5-sonnet', 6840, 'var(--hf-info)'],
      ['gemini-1.5-pro', 3120, 'var(--hf-ok)'],
      ['gpt-4o-mini', 2480, 'var(--hf-ink-2)'],
      ['+12 more', 2622, 'var(--hf-ink-3)'],
    ],
  },
  {
    lvl: 'products',
    items: [
      ['lurus-chat', 11200, 'var(--hf-accent)'],
      ['lurus-edit', 5210, 'var(--hf-info)'],
      ['public-api', 4120, 'var(--hf-ok)'],
      ['internal', 2480, 'var(--hf-ink-2)'],
      ['unset', 1172, 'var(--hf-ink-3)'],
    ],
  },
];

const HFDashboard = () => {
  const [drill, setDrill] = useState([]);

  return (
    <HFShell
      active='dashboard'
      crumbs={['workspace', 'dashboard']}
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
            live · 2s tick
          </span>
          <button type='button' className='btn'>
            tenant: all ▾
          </button>
          <button type='button' className='btn'>
            last 24h ▾
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
            Today, so far{' '}
            <span className='muted' style={{ fontWeight: 400 }}>
              · $1,051 / hour
            </span>
          </h1>
          <div className='sub'>
            aggregating 8 channels · 54 models · 23 tenants
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
        {KPIS.map((k, i) => (
          <div
            key={i}
            className='panel'
            style={{ gridColumn: 'span 3', padding: 18 }}
          >
            <div className='lbl'>{k.l}</div>
            <div className='display' style={{ fontSize: 32, marginTop: 4 }}>
              {k.v}
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
                {k.d} · 1h
              </span>
              <span className='spark'>
                {k.t.map((v, j) => (
                  <i
                    key={j}
                    style={{ height: v * 1.4 + 'px', background: k.c }}
                  />
                ))}
              </span>
            </div>
          </div>
        ))}

        <div className='panel' style={{ gridColumn: 'span 7', padding: 18 }}>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'baseline',
            }}
          >
            <div>
              <div className='lbl'>model performance · cost vs latency</div>
              <div className='display' style={{ fontSize: 18, marginTop: 2 }}>
                Where to spend, where to swap
              </div>
            </div>
            <span className='faint mono' style={{ fontSize: 10 }}>
              circle area = qps · last 24h
            </span>
          </div>
          <div
            style={{
              position: 'relative',
              height: 320,
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
            {BUBBLES.map((b, i) => (
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
            {BUBBLES.filter((b) => b.r >= 10).map((b, i) => (
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
            <div
              className='lbl'
              style={{ position: 'absolute', left: 0, bottom: -22 }}
            >
              p95 latency →
            </div>
            <div
              className='lbl'
              style={{
                position: 'absolute',
                left: -24,
                top: 0,
                transform: 'rotate(-90deg)',
                transformOrigin: 'left top',
              }}
            >
              $ / 1k tok →
            </div>
          </div>
        </div>

        <div className='panel' style={{ gridColumn: 'span 5', padding: 18 }}>
          <div className='lbl' style={{ marginBottom: 10 }}>
            active signals
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {SIGNALS.map((a, i) => (
              <div
                key={i}
                style={{
                  display: 'flex',
                  gap: 10,
                  padding: '8px 10px',
                  borderLeft: '2px solid var(--hf-' + a.sev + ')',
                  background: 'var(--hf-paper)',
                }}
              >
                <div style={{ flex: 1 }}>
                  <div className='strong' style={{ fontSize: 12 }}>
                    {a.t}
                  </div>
                  <div
                    className='muted mono'
                    style={{ fontSize: 10, marginTop: 2 }}
                  >
                    {a.s}
                  </div>
                </div>
                <button type='button' className='btn ghost sm'>
                  →
                </button>
              </div>
            ))}
          </div>
        </div>

        <div className='panel' style={{ gridColumn: 'span 12', padding: 18 }}>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 14 }}>
            <div className='lbl'>cost drilldown</div>
            <div className='mono muted' style={{ fontSize: 11 }}>
              all
              {drill.map((d, i) => (
                <span key={i}>
                  {' '}
                  › <b style={{ color: 'var(--hf-ink)' }}>{d}</b>
                </span>
              ))}
            </div>
            {drill.length > 0 && (
              <button
                type='button'
                className='btn ghost sm'
                onClick={() => setDrill([])}
              >
                reset
              </button>
            )}
            <span style={{ flex: 1 }} />
            <span className='display' style={{ fontSize: 24 }}>
              $24,182.40
            </span>
            <span className='muted' style={{ fontSize: 11 }}>
              this month
            </span>
          </div>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '1fr 1fr 1fr',
              gap: 14,
              marginTop: 14,
            }}
          >
            {DRILL_COLS.map((col, ci) => (
              <div key={ci}>
                <div className='lbl' style={{ marginBottom: 8 }}>
                  {col.lvl}
                </div>
                {col.items.map((it, i) => {
                  const total = col.items.reduce((s, x) => s + x[1], 0);
                  const pct = ((it[1] / total) * 100).toFixed(1);
                  return (
                    <div
                      key={i}
                      onClick={() => setDrill([...drill, it[0]].slice(0, 3))}
                      style={{
                        padding: '8px 0',
                        borderBottom: '1px dashed var(--hf-rule)',
                        cursor: 'pointer',
                      }}
                    >
                      <div
                        style={{
                          display: 'flex',
                          justifyContent: 'space-between',
                        }}
                      >
                        <span>{it[0]}</span>
                        <span className='mono strong'>
                          ${it[1].toLocaleString()}
                        </span>
                      </div>
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 6,
                          marginTop: 4,
                        }}
                      >
                        <div
                          style={{
                            height: 4,
                            flex: 1,
                            background: 'var(--hf-sunken)',
                          }}
                        >
                          <div
                            style={{
                              height: '100%',
                              width: pct + '%',
                              background: it[2],
                            }}
                          />
                        </div>
                        <span
                          className='mono faint'
                          style={{
                            fontSize: 10,
                            width: 36,
                            textAlign: 'right',
                          }}
                        >
                          {pct}%
                        </span>
                      </div>
                    </div>
                  );
                })}
              </div>
            ))}
          </div>
        </div>
      </div>
    </HFShell>
  );
};

export default HFDashboard;
