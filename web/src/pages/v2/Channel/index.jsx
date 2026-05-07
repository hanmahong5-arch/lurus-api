import React, { Fragment, useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';

/*
 * HiFi 2 — Channel health + batch ops.
 * Ported from design canvas hifi/hf2-channel.jsx (2026-05-07).
 */

const CHANNELS = [
  {
    name: 'openai/main',
    vendor: 'OpenAI',
    keys: 4,
    models: 18,
    qps: 142,
    p50: 812,
    p95: 2104,
    succ: 99.4,
    cost: 412.8,
    spark: [8, 9, 10, 11, 9, 8, 7, 9, 10, 11, 12, 11, 10, 9, 8, 9],
    status: 'ok',
  },
  {
    name: 'anthropic/eu',
    vendor: 'Anthropic',
    keys: 2,
    models: 6,
    qps: 88,
    p50: 1102,
    p95: 3210,
    succ: 98.1,
    cost: 281.1,
    spark: [4, 5, 5, 6, 6, 5, 5, 6, 7, 7, 6, 5, 4, 5, 5, 6],
    status: 'ok',
  },
  {
    name: 'azure/eu',
    vendor: 'Azure',
    keys: 6,
    models: 12,
    qps: 64,
    p50: 920,
    p95: 2580,
    succ: 99.8,
    cost: 198.4,
    spark: [3, 3, 4, 4, 5, 5, 4, 4, 3, 3, 4, 4, 5, 5, 4, 4],
    status: 'ok',
  },
  {
    name: 'vertex/asia',
    vendor: 'Google',
    keys: 3,
    models: 9,
    qps: 41,
    p50: 680,
    p95: 1980,
    succ: 87.2,
    cost: 88.6,
    spark: [2, 3, 3, 2, 2, 3, 4, 8, 12, 14, 11, 9, 7, 5, 4, 4],
    status: 'warn',
  },
  {
    name: 'bedrock/us-w2',
    vendor: 'AWS',
    keys: 1,
    models: 7,
    qps: 18,
    p50: 720,
    p95: 2240,
    succ: 99.0,
    cost: 44.2,
    spark: [1, 1, 2, 2, 2, 1, 1, 2, 2, 2, 1, 1, 2, 2, 2, 2],
    status: 'ok',
  },
  {
    name: 'openai/backup',
    vendor: 'OpenAI',
    keys: 1,
    models: 18,
    qps: 0,
    p50: 0,
    p95: 0,
    succ: 0.0,
    cost: 0.0,
    spark: [2, 3, 2, 3, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
    status: 'down',
  },
  {
    name: 'siliconflow/cn',
    vendor: 'Silicon',
    keys: 2,
    models: 22,
    qps: 31,
    p50: 410,
    p95: 1102,
    succ: 99.9,
    cost: 18.4,
    spark: [2, 2, 2, 3, 3, 3, 2, 2, 2, 3, 3, 3, 2, 2, 2, 3],
    status: 'ok',
  },
  {
    name: 'zhipu/glm',
    vendor: 'Zhipu',
    keys: 1,
    models: 4,
    qps: 12,
    p50: 580,
    p95: 1480,
    succ: 96.4,
    cost: 8.1,
    spark: [1, 1, 1, 2, 1, 1, 1, 1, 2, 2, 1, 1, 1, 1, 2, 1],
    status: 'ok',
  },
];

const MAPPINGS = [
  ['gemini-1.5-pro-002', 'gemini-1.5-pro', 'ok'],
  ['gemini-1.5-flash-002', 'gemini-1.5-flash', 'ok'],
  ['gemini-2.0-flash-exp', 'gemini-2-flash', 'warn'],
  ['text-embedding-004', 'embedding-default', 'ok'],
];

const KEYS = [
  ['k_a91f...e2', 'ok', '14ms', '$31.20', 32],
  ['k_8b03...77', 'warn', '180ms', '$48.10', 51],
  ['k_de14...0a', 'err', '—', '$9.30', 9],
];

const HFChannel = () => {
  const [sel, setSel] = useState(new Set([3]));
  const [open, setOpen] = useState(3);

  const toggle = (i) => {
    const n = new Set(sel);
    if (n.has(i)) {
      n.delete(i);
    } else {
      n.add(i);
    }
    setSel(n);
  };

  return (
    <HFShell
      active='channels'
      crumbs={['platform · admin', 'channels']}
      actions={
        <>
          <button type='button' className='btn'>
            group: all ▾
          </button>
          <button type='button' className='btn'>
            vendor: all ▾
          </button>
          <button type='button' className='btn primary'>
            + new channel
          </button>
        </>
      }
    >
      <div className='hf-page-head'>
        <div>
          <div className='lbl' style={{ marginBottom: 6 }}>
            channels
          </div>
          <h1>
            8 upstream channels{' '}
            <span className='muted' style={{ fontWeight: 400 }}>
              routing 398 qps
            </span>
          </h1>
          <div className='sub'>monitoring · last 1h · auto-refresh</div>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 28 }}>
          {[
            ['healthy', '6', 'var(--hf-ok)'],
            ['degraded', '1', 'var(--hf-warn)'],
            ['down', '1', 'var(--hf-err)'],
            ['cost / hr', '$1,051', 'var(--hf-ink)'],
          ].map(([l, v, c], i) => (
            <div key={i}>
              <div className='lbl'>{l}</div>
              <div
                className='display'
                style={{ fontSize: 26, color: c, marginTop: 2 }}
              >
                {v}
              </div>
            </div>
          ))}
        </div>
      </div>

      <div
        style={{
          padding: '10px 28px',
          background: sel.size ? 'var(--hf-accent)' : 'var(--hf-paper)',
          color: sel.size ? '#fff' : 'var(--hf-ink-3)',
          borderBottom: '1px solid var(--hf-rule)',
          fontFamily: 'var(--hf-mono)',
          fontSize: 11,
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          transition: 'background 0.15s',
        }}
      >
        {sel.size ? (
          <>
            <span>
              <b>{sel.size}</b> selected
            </span>
            <span style={{ opacity: 0.5 }}>·</span>
            {[
              'enable',
              'disable',
              'set weight…',
              'move group…',
              'test all',
            ].map((label) => (
              <button
                key={label}
                type='button'
                className='btn'
                style={{
                  background: 'rgba(255,255,255,0.15)',
                  borderColor: 'rgba(255,255,255,0.3)',
                  color: '#fff',
                }}
              >
                {label}
              </button>
            ))}
            <span style={{ flex: 1 }} />
            <button
              type='button'
              className='btn ghost'
              style={{ color: '#fff' }}
              onClick={() => setSel(new Set())}
            >
              clear
            </button>
          </>
        ) : (
          <span>
            tip · select rows for batch operations · or click any row to expand
          </span>
        )}
      </div>

      <table className='t'>
        <thead>
          <tr>
            <th style={{ width: 32 }}></th>
            <th>channel</th>
            <th>keys</th>
            <th>models</th>
            <th>qps · 1h trend</th>
            <th>p50 / p95</th>
            <th>success</th>
            <th>cost · 1h</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {CHANNELS.map((c, i) => (
            <Fragment key={i}>
              <tr
                style={{
                  background: sel.has(i) ? 'rgba(255,93,31,0.08)' : undefined,
                  borderLeft: sel.has(i)
                    ? '2px solid var(--hf-accent)'
                    : '2px solid transparent',
                  cursor: 'pointer',
                }}
              >
                <td onClick={() => toggle(i)}>
                  <input type='checkbox' checked={sel.has(i)} readOnly />
                </td>
                <td onClick={() => setOpen(open === i ? -1 : i)}>
                  <div
                    style={{ display: 'flex', alignItems: 'center', gap: 10 }}
                  >
                    <span
                      className={
                        'dot ' +
                        (c.status === 'ok'
                          ? 'ok'
                          : c.status === 'warn'
                            ? 'warn'
                            : 'err')
                      }
                    />
                    <div>
                      <div className='strong'>{c.name}</div>
                      <div className='faint mono' style={{ fontSize: 9 }}>
                        {c.vendor}
                      </div>
                    </div>
                  </div>
                </td>
                <td className='mono'>{c.keys}</td>
                <td className='mono'>{c.models}</td>
                <td>
                  <div
                    style={{ display: 'flex', alignItems: 'center', gap: 10 }}
                  >
                    <span
                      className='display'
                      style={{ fontSize: 16, minWidth: 36 }}
                    >
                      {c.qps}
                    </span>
                    <span
                      className={
                        'spark ' +
                        (c.status === 'down'
                          ? 'err'
                          : c.status === 'warn'
                            ? 'warn'
                            : '')
                      }
                    >
                      {c.spark.map((v, j) => (
                        <i
                          key={j}
                          style={{ height: Math.max(1, v * 1.5) + 'px' }}
                        />
                      ))}
                    </span>
                  </div>
                </td>
                <td className='mono muted'>
                  {c.p50}/{c.p95}
                  <span className='faint'>ms</span>
                </td>
                <td>
                  <div
                    style={{ display: 'flex', alignItems: 'center', gap: 8 }}
                  >
                    <div
                      style={{
                        width: 80,
                        height: 4,
                        background: 'var(--hf-sunken)',
                      }}
                    >
                      <div
                        style={{
                          width: c.succ + '%',
                          height: '100%',
                          background:
                            c.succ >= 99
                              ? 'var(--hf-ok)'
                              : c.succ >= 95
                                ? 'var(--hf-warn)'
                                : 'var(--hf-err)',
                        }}
                      />
                    </div>
                    <span
                      className='mono'
                      style={{
                        color:
                          c.succ >= 99
                            ? 'var(--hf-ok)'
                            : c.succ >= 95
                              ? 'var(--hf-warn)'
                              : 'var(--hf-err)',
                      }}
                    >
                      {c.succ.toFixed(1)}%
                    </span>
                  </div>
                </td>
                <td className='mono'>${c.cost.toFixed(2)}</td>
                <td>
                  <button
                    type='button'
                    className='btn ghost'
                    onClick={() => setOpen(open === i ? -1 : i)}
                  >
                    {open === i ? '▾' : '▸'}
                  </button>
                </td>
              </tr>
              {open === i && (
                <tr>
                  <td
                    colSpan={9}
                    style={{
                      padding: 0,
                      background: 'var(--hf-paper)',
                      borderLeft: '2px solid var(--hf-accent)',
                    }}
                  >
                    <div
                      style={{
                        padding: 20,
                        display: 'grid',
                        gridTemplateColumns: '1.3fr 1fr 1fr',
                        gap: 18,
                      }}
                    >
                      <div>
                        <div className='lbl' style={{ marginBottom: 8 }}>
                          model mapping
                        </div>
                        <div
                          style={{
                            display: 'flex',
                            flexDirection: 'column',
                            gap: 6,
                          }}
                        >
                          {MAPPINGS.map((r, j) => (
                            <div
                              key={j}
                              style={{
                                display: 'grid',
                                gridTemplateColumns: '1fr 16px 1fr',
                                gap: 8,
                                alignItems: 'center',
                                fontSize: 11,
                              }}
                            >
                              <span className='pill mono'>{r[0]}</span>
                              <span
                                className='faint'
                                style={{ textAlign: 'center' }}
                              >
                                →
                              </span>
                              <span className='pill mono'>
                                <span className={'dot ' + r[2]} />
                                {r[1]}
                              </span>
                            </div>
                          ))}
                          <button
                            type='button'
                            className='btn sm'
                            style={{ alignSelf: 'flex-start', marginTop: 4 }}
                          >
                            + add mapping
                          </button>
                        </div>
                      </div>
                      <div>
                        <div className='lbl' style={{ marginBottom: 8 }}>
                          keys (3) · click to inspect
                        </div>
                        {KEYS.map((k, j) => (
                          <div
                            key={j}
                            style={{
                              display: 'grid',
                              gridTemplateColumns: 'auto 1fr auto auto auto',
                              gap: 10,
                              alignItems: 'center',
                              padding: '6px 0',
                              borderBottom: '1px dashed var(--hf-rule)',
                              fontSize: 11,
                            }}
                          >
                            <span className={'dot ' + k[1]} />
                            <span className='mono'>{k[0]}</span>
                            <span className='faint mono'>{k[2]}</span>
                            <span
                              className='mono'
                              style={{ width: 60, textAlign: 'right' }}
                            >
                              {k[3]}
                            </span>
                            <button type='button' className='btn sm'>
                              rotate
                            </button>
                          </div>
                        ))}
                      </div>
                      <div>
                        <div className='lbl' style={{ marginBottom: 8 }}>
                          routing
                        </div>
                        <div
                          className='panel-paper'
                          style={{ padding: 12, fontSize: 11, lineHeight: 1.7 }}
                        >
                          <div>
                            <span className='muted'>weight:</span> <b>1.0</b>
                          </div>
                          <div>
                            <span className='muted'>priority:</span>{' '}
                            <b>primary</b>
                          </div>
                          <div>
                            <span className='muted'>groups:</span>{' '}
                            <b>default, vip</b>
                          </div>
                          <div>
                            <span className='muted'>timeout:</span> 60s
                          </div>
                          <div>
                            <span className='muted'>retry:</span> 2× →
                            openai/backup
                          </div>
                          <div
                            style={{ display: 'flex', gap: 6, marginTop: 10 }}
                          >
                            <button type='button' className='btn sm'>
                              test now
                            </button>
                            <button type='button' className='btn sm'>
                              edit
                            </button>
                            <button
                              type='button'
                              className='btn sm'
                              style={{ color: 'var(--hf-err)' }}
                            >
                              disable
                            </button>
                          </div>
                        </div>
                      </div>
                    </div>
                  </td>
                </tr>
              )}
            </Fragment>
          ))}
        </tbody>
      </table>
    </HFShell>
  );
};

export default HFChannel;
