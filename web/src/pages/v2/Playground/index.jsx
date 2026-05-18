import React from 'react';
import HFShell from '../../../components/hifi/HFShell';
import WIPBanner from '../../../components/hifi/WIPBanner';

/*
 * HiFi 5 — Playground multi-model compare.
 * Ported from design canvas hifi/hf5-playground.jsx (2026-05-07).
 */

const COLS = [
  {
    m: 'gpt-4o',
    vendor: 'OpenAI',
    ttft: 412,
    total: 1843,
    tok: { in: 124, out: 287 },
    cost: 0.0042,
    verdict: 'fast · accurate',
    vc: 'var(--hf-ok)',
    out: `The capital of Australia is **Canberra**, located in the Australian Capital Territory (ACT). It was selected as a compromise between Sydney and Melbourne when the country was federated in 1901.

Canberra is a planned city, designed by American architect Walter Burley Griffin after winning an international competition in 1912.`,
  },
  {
    m: 'claude-3.5-sonnet',
    vendor: 'Anthropic',
    ttft: 295,
    total: 1102,
    tok: { in: 124, out: 198 },
    cost: 0.009,
    verdict: 'fastest · concise',
    vc: 'var(--hf-accent)',
    out: `**Canberra** is the capital of Australia. It was purpose-built as the capital after a long debate between Sydney and Melbourne — neither wanted the other to have the honor, so a new site between them was chosen.

The name comes from a local Ngunnawal word, often translated as "meeting place."`,
  },
  {
    m: 'gemini-1.5-pro',
    vendor: 'Google',
    ttft: 580,
    total: 2210,
    tok: { in: 124, out: 312 },
    cost: 0.0031,
    verdict: 'cheapest · verbose',
    vc: 'var(--hf-info)',
    out: `Australia's capital city is Canberra. Here are some key facts:

1. **Location**: Australian Capital Territory (ACT), inland, ~280 km southwest of Sydney.
2. **Founded**: 1913, although planning began earlier.
3. **Designer**: Walter Burley Griffin and his wife Marion Mahony Griffin.
4. **Population**: ~460,000 (2023 est.)
5. **Why not Sydney or Melbourne**: Federation compromise of 1901.`,
  },
];

const HFPlayground = () => {
  return (
    <HFShell
      active='playground'
      crumbs={['workspace', 'playground']}
      actions={
        <>
          <span className='lbl'>preset:</span>
          <button type='button' className='btn'>
            blank ▾
          </button>
          <button type='button' className='btn'>
            save
          </button>
          <button type='button' className='btn'>
            share ↗
          </button>
        </>
      }
    >
      <WIPBanner
        reason='Multi-model playground requires a fan-out relay endpoint that does not exist yet.'
        todo='Backend: /api/v2/{slug}/playground/run; UI wires after.'
      />
      <div
        style={{
          display: 'grid',
          gridTemplateRows: 'auto auto 1fr auto',
          height: '100%',
        }}
      >
        <div
          style={{
            padding: '20px 28px',
            background: 'var(--hf-paper)',
            borderBottom: '1px solid var(--hf-rule)',
          }}
        >
          <div className='lbl' style={{ marginBottom: 4 }}>
            compare
          </div>
          <h1 className='display' style={{ fontSize: 28, margin: 0 }}>
            3 models, one prompt, side by side
          </h1>

          <div
            style={{
              marginTop: 16,
              display: 'grid',
              gridTemplateColumns: '70px 1fr',
              gap: 12,
            }}
          >
            <div
              className='lbl'
              style={{ alignSelf: 'flex-start', paddingTop: 6 }}
            >
              system
            </div>
            <div
              className='field'
              style={{
                height: 'auto',
                padding: '8px 12px',
                alignItems: 'flex-start',
              }}
            >
              <span className='muted'>
                You are a helpful, concise assistant.
              </span>
            </div>
            <div
              className='lbl'
              style={{ alignSelf: 'flex-start', paddingTop: 6 }}
            >
              user
            </div>
            <div
              className='field'
              style={{
                height: 'auto',
                padding: '8px 12px',
                alignItems: 'flex-start',
              }}
            >
              <span className='strong'>
                What is the capital of Australia? Briefly.
              </span>
            </div>
          </div>
          <div
            style={{
              display: 'flex',
              gap: 8,
              marginTop: 14,
              alignItems: 'center',
            }}
          >
            <span className='lbl'>params</span>
            <span className='pill'>temp · 0.7</span>
            <span className='pill'>top_p · 1.0</span>
            <span className='pill'>max · 1024</span>
            <span style={{ flex: 1 }} />
            <span className='kbd'>⌘</span>
            <span className='kbd'>↵</span>
            <button type='button' className='btn acc'>
              ▶ run all 3
            </button>
          </div>
        </div>

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(3, 1fr)',
            borderBottom: '1px solid var(--hf-rule)',
          }}
        >
          {COLS.map((c, i) => (
            <div
              key={i}
              style={{
                padding: 16,
                borderRight: i < 2 ? '1px solid var(--hf-rule)' : 0,
                background: 'var(--hf-paper)',
              }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'baseline',
                  justifyContent: 'space-between',
                }}
              >
                <div>
                  <div className='display' style={{ fontSize: 17 }}>
                    {c.m}
                  </div>
                  <div className='faint mono' style={{ fontSize: 10 }}>
                    {c.vendor} · openai/main
                  </div>
                </div>
                <button type='button' className='btn ghost sm'>
                  swap ▾
                </button>
              </div>
              <div style={{ display: 'flex', gap: 8, marginTop: 10 }}>
                <span className='pill'>
                  <span className='dot ok' />
                  {c.ttft}ms
                </span>
                <span className='pill'>{c.total}ms</span>
                <span className='pill'>${c.cost.toFixed(4)}</span>
              </div>
              <div className='lbl' style={{ marginTop: 10, color: c.vc }}>
                {c.verdict}
              </div>
            </div>
          ))}
        </div>

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(3, 1fr)',
            overflow: 'hidden',
          }}
        >
          {COLS.map((c, i) => (
            <div
              key={i}
              style={{
                padding: 18,
                borderRight: i < 2 ? '1px solid var(--hf-rule)' : 0,
                overflow: 'auto',
                fontSize: 13,
                lineHeight: 1.6,
                color: 'var(--hf-ink-2)',
                whiteSpace: 'pre-wrap',
              }}
            >
              {c.out}
            </div>
          ))}
        </div>

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(3, 1fr)',
            borderTop: '1px solid var(--hf-rule)',
            background: 'var(--hf-paper)',
          }}
        >
          {COLS.map((c, i) => (
            <div
              key={i}
              style={{
                padding: '10px 16px',
                borderRight: i < 2 ? '1px solid var(--hf-rule)' : 0,
                display: 'flex',
                alignItems: 'center',
                gap: 10,
                fontSize: 11,
              }}
            >
              <span className='muted mono'>
                tok {c.tok.in}
                <span className='faint'>↗</span>
                {c.tok.out}
              </span>
              <span style={{ flex: 1 }} />
              <button type='button' className='btn ghost sm'>
                copy
              </button>
              <button type='button' className='btn ghost sm'>
                save as test
              </button>
            </div>
          ))}
        </div>
      </div>
    </HFShell>
  );
};

export default HFPlayground;
