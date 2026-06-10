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
import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import HFShell from '../../../components/hifi/HFShell';

/*
 * HiFi 6 — Cmd-K command palette + IA reference.
 * Ported from design canvas hifi/hf6-cmdk.jsx (2026-05-07).
 */

// Translatable strings are [key, fallback] pairs resolved at render via tr()
// (module scope has no i18n context). Raw strings (channel/model/request ids,
// prices, paths) stay untranslated by design.
const GROUPS = [
  {
    g: ['group_navigate', 'navigate'],
    items: [
      [['nav_dashboard', 'Dashboard'], '/console/v2/dashboard', 'g d'],
      [['nav_logs', 'Logs · last 1h'], '/console/v2/log', 'g l'],
      [['nav_channels', 'Channels'], '/console/v2/channel', 'g c'],
      [['nav_tokens', 'Tokens'], '/console/v2/token', 'g t'],
      [['nav_playground', 'Playground'], '/console/v2/playground', 'g p'],
    ],
  },
  {
    g: ['group_channels', 'channels'],
    items: [
      ['openai/main', ['demo_ch_openai', '$412.80 / 1h · 99.4% healthy'], null],
      [
        'anthropic/eu',
        ['demo_ch_anthropic', '$281.10 / 1h · 98.1% healthy'],
        null,
      ],
      ['vertex/asia', ['demo_ch_vertex', '$88.60 / 1h · 87.2% degraded'], null],
    ],
  },
  {
    g: ['group_models', 'models'],
    items: [
      ['gpt-4o', ['demo_model_gpt4o', '$2.50 / $10 · 128k ctx'], null],
      [
        'claude-3.5-sonnet',
        ['demo_model_claude', '$3.00 / $15 · 200k ctx'],
        null,
      ],
      ['gemini-1.5-pro', ['demo_model_gemini', '$1.25 / $5 · 2M ctx'], null],
    ],
  },
  {
    g: ['group_recent', 'recent'],
    items: [
      ['req_1f4a...e90c · 504', 'gpt-4o · 4.8s · acme', null],
      ['req_8b03...77ef · 200', 'claude-3.5 · 1.1s · contoso', null],
    ],
  },
  {
    g: ['group_actions', 'actions'],
    items: [
      [
        ['action_create_token', 'Create token…'],
        ['action_create_token_hint', 'opens wizard'],
        '⌘N',
      ],
      [['action_rotate_key', 'Rotate api key…'], '', null],
      [['action_set_budget', 'Set monthly budget…'], '', null],
      [
        ['action_toggle_theme', 'Toggle theme'],
        ['action_toggle_theme_hint', 'light · dark'],
        '⌘.',
      ],
    ],
  },
];

const KBD_REF = [
  [
    ['⌘', 'K'],
    ['kbd_open_palette', 'open palette · search anything'],
  ],
  [
    ['g', 'd'],
    ['kbd_go_dashboard', 'go to dashboard'],
  ],
  [
    ['g', 'l'],
    ['kbd_go_logs', 'go to logs'],
  ],
  [
    ['g', 'c'],
    ['kbd_go_channels', 'go to channels'],
  ],
  [
    ['g', 't'],
    ['kbd_go_tokens', 'go to tokens'],
  ],
  [
    ['g', 'p'],
    ['kbd_go_playground', 'go to playground'],
  ],
  [
    ['⌘', 'N'],
    ['kbd_new_token', 'new token'],
  ],
  [
    ['⌘', '.'],
    ['kbd_toggle_theme', 'toggle theme'],
  ],
];

const HFCmdK = () => {
  const { t: tr } = useTranslation();
  const [open, setOpen] = useState(true);
  const [q, setQ] = useState('');
  const [hover, setHover] = useState(0);

  // Resolve a [key, fallback] pair via i18n; pass raw strings through.
  const tx = (v) =>
    Array.isArray(v) ? tr(`console.palette.${v[0]}`, v[1]) : v;

  return (
    <HFShell
      active='tokens'
      crumbs={[
        tr('console.palette.crumb_account', 'my account'),
        tr('console.palette.crumb_tokens', 'tokens'),
      ]}
      actions={
        <>
          <button type='button' className='btn'>
            {tr('console.palette.btn_last_30d', 'last 30d ▾')}
          </button>
          <button type='button' className='btn primary'>
            {tr('console.palette.btn_new_token', '+ new token')}
          </button>
        </>
      }
    >
      <div style={{ position: 'relative', height: '100%' }}>
        <div className='hf-page-head'>
          <div>
            <div className='lbl' style={{ marginBottom: 6 }}>
              {tr('console.palette.heading_lbl', 'tokens')}
            </div>
            <h1>{tr('console.palette.heading', { count: 5 })}</h1>
            <div className='sub'>
              {tr(
                'console.palette.sub',
                'try the global palette · ⌘K is the fastest path through Lurus Hub',
              )}
            </div>
          </div>
        </div>
        <div style={{ padding: 28, color: 'var(--hf-ink-3)', fontSize: 13 }}>
          <div className='panel-paper' style={{ padding: 22 }}>
            <div className='lbl'>
              {tr('console.palette.kbd_reference', 'keyboard reference')}
            </div>
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
                  <span className='muted'>{tx(d)}</span>
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
                  placeholder={tr(
                    'console.palette.ph_search',
                    'go to · run · search logs models tokens channels…',
                  )}
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
                      {tx(gr.g)}
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
                            {tx(it[0])}
                          </span>
                          <span
                            className='muted mono'
                            style={{ flex: 1, fontSize: 11 }}
                          >
                            {tx(it[1])}
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
                  <span className='kbd'>↑↓</span>{' '}
                  {tr('console.palette.footer_navigate', 'navigate')}
                </span>
                <span>
                  <span className='kbd'>↵</span>{' '}
                  {tr('console.palette.footer_open', 'open')}
                </span>
                <span>
                  <span className='kbd'>⌘↵</span>{' '}
                  {tr('console.palette.footer_new_tab', 'new tab')}
                </span>
                <span style={{ flex: 1 }} />
                <span>
                  {tr('console.palette.footer_results', { count: 72 })}
                </span>
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
            {tr('console.palette.open_palette_btn', '⌘K open palette')}
          </button>
        )}
      </div>
    </HFShell>
  );
};

export default HFCmdK;
