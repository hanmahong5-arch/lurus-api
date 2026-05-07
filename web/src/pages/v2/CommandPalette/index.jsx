import React, { useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';

/*
 * HiFi 6 — Cmd-K command palette + IA reference.
 * Ported from design canvas hifi/hf6-cmdk.jsx (2026-05-07).
 */

const GROUPS = [
  {
    g: 'navigate',
    items: [
      ['Dashboard', '/console/v2/dashboard', 'g d'],
      ['Logs · last 1h', '/console/v2/log', 'g l'],
      ['Channels', '/console/v2/channel', 'g c'],
      ['Tokens', '/console/v2/token', 'g t'],
      ['Playground', '/console/v2/playground', 'g p'],
    ],
  },
  {
    g: 'channels',
    items: [
      ['openai/main', '$412.80 / 1h · 99.4% healthy', null],
      ['anthropic/eu', '$281.10 / 1h · 98.1% healthy', null],
      ['vertex/asia', '$88.60 / 1h · 87.2% degraded', null],
    ],
  },
  {
    g: 'models',
    items: [
      ['gpt-4o', '$2.50 / $10 · 128k ctx', null],
      ['claude-3.5-sonnet', '$3.00 / $15 · 200k ctx', null],
      ['gemini-1.5-pro', '$1.25 / $5 · 2M ctx', null],
    ],
  },
  {
    g: 'recent',
    items: [
      ['req_1f4a...e90c · 504', 'gpt-4o · 4.8s · acme', null],
      ['req_8b03...77ef · 200', 'claude-3.5 · 1.1s · contoso', null],
    ],
  },
  {
    g: 'actions',
    items: [
      ['Create token…', 'opens wizard', '⌘N'],
      ['Rotate api key…', '', null],
      ['Set monthly budget…', '', null],
      ['Toggle theme', 'light · dark', '⌘.'],
    ],
  },
];

const KBD_REF = [
  [['⌘', 'K'], 'open palette · search anything'],
  [['g', 'd'], 'go to dashboard'],
  [['g', 'l'], 'go to logs'],
  [['g', 'c'], 'go to channels'],
  [['g', 't'], 'go to tokens'],
  [['g', 'p'], 'go to playground'],
  [['⌘', 'N'], 'new token'],
  [['⌘', '.'], 'toggle theme'],
];

const HFCmdK = () => {
  const [open, setOpen] = useState(true);
  const [q, setQ] = useState('');
  const [hover, setHover] = useState(0);

  return (
    <HFShell
      active='tokens'
      crumbs={['my account', 'tokens']}
      actions={
        <>
          <button type='button' className='btn'>
            last 30d ▾
          </button>
          <button type='button' className='btn primary'>
            + new token
          </button>
        </>
      }
    >
      <div style={{ position: 'relative', height: '100%' }}>
        <div className='hf-page-head'>
          <div>
            <div className='lbl' style={{ marginBottom: 6 }}>
              tokens
            </div>
            <h1>5 active tokens</h1>
            <div className='sub'>
              try the global palette · ⌘K is the fastest path through Lurus Hub
            </div>
          </div>
        </div>
        <div style={{ padding: 28, color: 'var(--hf-ink-3)', fontSize: 13 }}>
          <div className='panel-paper' style={{ padding: 22 }}>
            <div className='lbl'>keyboard reference</div>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(2, 1fr)',
                gap: '10px 24px',
                marginTop: 10,
                fontSize: 12,
              }}
            >
              {KBD_REF.map(([k, d], i) => (
                <div
                  key={i}
                  style={{ display: 'flex', alignItems: 'center', gap: 8 }}
                >
                  {k.map((x, j) => (
                    <span key={j} className='kbd'>
                      {x}
                    </span>
                  ))}
                  <span className='muted'>{d}</span>
                </div>
              ))}
            </div>
          </div>
        </div>

        {open && (
          <div
            onClick={() => setOpen(false)}
            style={{
              position: 'absolute',
              inset: 0,
              background: 'rgba(10,9,8,0.4)',
              backdropFilter: 'blur(3px)',
              display: 'flex',
              justifyContent: 'center',
              paddingTop: 96,
              zIndex: 50,
            }}
          >
            <div
              onClick={(e) => e.stopPropagation()}
              style={{
                width: 600,
                maxHeight: 480,
                background: 'var(--hf-elev)',
                border: '1px solid var(--hf-rule-strong)',
                boxShadow: '0 32px 72px rgba(0,0,0,0.28)',
                display: 'flex',
                flexDirection: 'column',
                borderRadius: 4,
              }}
            >
              <div
                style={{
                  padding: 14,
                  borderBottom: '1px solid var(--hf-rule)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 12,
                }}
              >
                <span className='muted' style={{ fontSize: 16 }}>
                  ⌕
                </span>
                <input
                  autoFocus
                  value={q}
                  onChange={(e) => setQ(e.target.value)}
                  placeholder='go to · run · search logs models tokens channels…'
                  style={{
                    flex: 1,
                    border: 0,
                    outline: 0,
                    background: 'transparent',
                    fontFamily: 'var(--hf-mono)',
                    fontSize: 13,
                    color: 'var(--hf-ink)',
                  }}
                />
                <span className='kbd'>esc</span>
              </div>
              <div style={{ overflow: 'auto', flex: 1 }}>
                {GROUPS.map((gr, gi) => (
                  <div key={gi}>
                    <div className='lbl' style={{ padding: '10px 16px 4px' }}>
                      {gr.g}
                    </div>
                    {gr.items.map((it, i) => {
                      const idx = gi * 100 + i;
                      const active = hover === idx;
                      return (
                        <div
                          key={i}
                          onMouseEnter={() => setHover(idx)}
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 12,
                            padding: '8px 16px',
                            cursor: 'pointer',
                            background: active
                              ? 'var(--hf-sunken)'
                              : 'transparent',
                            borderLeft: active
                              ? '2px solid var(--hf-accent)'
                              : '2px solid transparent',
                          }}
                        >
                          <span
                            className='strong'
                            style={{ minWidth: 230, fontSize: 13 }}
                          >
                            {it[0]}
                          </span>
                          <span
                            className='muted mono'
                            style={{ flex: 1, fontSize: 11 }}
                          >
                            {it[1]}
                          </span>
                          {it[2] && <span className='kbd'>{it[2]}</span>}
                        </div>
                      );
                    })}
                  </div>
                ))}
              </div>
              <div
                style={{
                  padding: '8px 14px',
                  borderTop: '1px solid var(--hf-rule)',
                  display: 'flex',
                  gap: 14,
                  fontFamily: 'var(--hf-mono)',
                  fontSize: 10,
                  color: 'var(--hf-ink-3)',
                }}
              >
                <span>
                  <span className='kbd'>↑↓</span> navigate
                </span>
                <span>
                  <span className='kbd'>↵</span> open
                </span>
                <span>
                  <span className='kbd'>⌘↵</span> new tab
                </span>
                <span style={{ flex: 1 }} />
                <span>72 results</span>
              </div>
            </div>
          </div>
        )}

        {!open && (
          <button
            type='button'
            className='btn primary'
            onClick={() => setOpen(true)}
            style={{
              position: 'absolute',
              bottom: 24,
              right: 24,
              height: 36,
              padding: '0 16px',
            }}
          >
            ⌘K open palette
          </button>
        )}
      </div>
    </HFShell>
  );
};

export default HFCmdK;
