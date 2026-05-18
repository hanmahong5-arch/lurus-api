import React, { useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';
import WIPBanner from '../../../components/hifi/WIPBanner';

/* HiFi 8 — Chat. Ported from hifi/hf8-chat.jsx (2026-05-07 variants pack). */

const CONV = [
  {
    who: 'user',
    t: '帮我把这段 SQL 改成 PostgreSQL 兼容的写法，并解释关键区别。',
  },
  {
    who: 'asst',
    t: "Sure — paste the SQL and I'll rewrite it. The most common issues moving to Postgres are:\n\n1. **Identifier quoting**: backticks → double quotes\n2. **AUTO_INCREMENT** → `GENERATED ALWAYS AS IDENTITY` or `serial`\n3. **DATETIME** → `timestamp` / `timestamptz`\n4. **GROUP_CONCAT** → `string_agg`",
    meta: { ttft: 295, total: 1102, tok: 198, cost: 0.0028 },
  },
  {
    who: 'user',
    t: 'SQL 是这个：\n```sql\nSELECT GROUP_CONCAT(name SEPARATOR ", ") FROM users WHERE created_at > "2024-01-01"\n```',
  },
  {
    who: 'asst',
    t: "Direct port:\n\n```sql\nSELECT string_agg(name, ', ')\nFROM users\nWHERE created_at > '2024-01-01'::timestamptz;\n```\n\n**Key changes**:\n- `GROUP_CONCAT(... SEPARATOR x)` → `string_agg(col, x)` — argument order is the same.\n- Double-quoted strings `\"...\"` are identifiers in Postgres; use single quotes for string literals.\n- Cast date strings explicitly with `::timestamptz` so the planner can pick a btree index.",
    meta: { ttft: 312, total: 1480, tok: 287, cost: 0.0042 },
  },
];

const TODAY = [
  ['SQL ← PostgreSQL 改写', '2m', true],
  ['解释 OAuth 2 PKCE 流程', '34m', false],
  ['Tailwind v4 migration plan', '1h', false],
];

const YESTERDAY = ['nginx → caddy 配置迁移', 'review prd · onboarding'];

const HFChat = () => {
  const [model] = useState('claude-3.5-sonnet');

  return (
    <HFShell
      active='chat'
      crumbs={['workspace', 'chat']}
      actions={
        <>
          <button type='button' className='btn'>
            share ↗
          </button>
          <button type='button' className='btn'>
            ⋯
          </button>
        </>
      }
    >
      <WIPBanner
        reason='Conversations are hardcoded mock content (CONV const). Real chat needs a tenant-scoped conversation store and streaming SSE relay — both pending Wave 2 consumer-feature decision.'
        todo='Pending consumer-feature deletion call (see hardening-swarm-2026-05-18-acceptance.md §Out of Scope).'
      />
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '260px 1fr',
          height: '100%',
          minHeight: 0,
        }}
      >
        <div
          style={{
            borderRight: '1px solid var(--hf-rule)',
            background: 'var(--hf-paper)',
            overflow: 'auto',
          }}
        >
          <div style={{ padding: '14px 16px' }}>
            <button
              type='button'
              className='btn primary'
              style={{ width: '100%', justifyContent: 'center' }}
            >
              + new chat
            </button>
          </div>
          <div className='lbl' style={{ padding: '8px 16px' }}>
            today
          </div>
          {TODAY.map((c, i) => (
            <div
              key={i}
              style={{
                padding: '10px 16px',
                cursor: 'pointer',
                background: c[2] ? 'var(--hf-elev)' : 'transparent',
                borderLeft: c[2]
                  ? '2px solid var(--hf-accent)'
                  : '2px solid transparent',
                borderBottom: '1px solid var(--hf-rule)',
              }}
            >
              <div
                className='strong'
                style={{
                  fontSize: 12,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
              >
                {c[0]}
              </div>
              <div
                className='faint mono'
                style={{ fontSize: 10, marginTop: 2 }}
              >
                {c[1]} ago
              </div>
            </div>
          ))}
          <div className='lbl' style={{ padding: '8px 16px' }}>
            yesterday
          </div>
          {YESTERDAY.map((t, i) => (
            <div
              key={i}
              style={{
                padding: '10px 16px',
                borderBottom: '1px solid var(--hf-rule)',
                cursor: 'pointer',
              }}
            >
              <div
                className='strong'
                style={{
                  fontSize: 12,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
              >
                {t}
              </div>
              <div
                className='faint mono'
                style={{ fontSize: 10, marginTop: 2 }}
              >
                1d ago
              </div>
            </div>
          ))}
        </div>

        <div
          style={{
            display: 'grid',
            gridTemplateRows: 'auto 1fr auto',
            minHeight: 0,
          }}
        >
          <div
            style={{
              padding: '12px 22px',
              borderBottom: '1px solid var(--hf-rule)',
              display: 'flex',
              alignItems: 'center',
              gap: 10,
            }}
          >
            <div>
              <div className='display' style={{ fontSize: 17 }}>
                SQL ← PostgreSQL 改写
              </div>
              <div className='muted mono' style={{ fontSize: 10 }}>
                4 turns · 0.0089 USD · started 2m ago
              </div>
            </div>
            <span style={{ flex: 1 }} />
            <span className='pill'>
              <span className='dot ok' /> {model}
            </span>
            <button type='button' className='btn sm'>
              swap model ▾
            </button>
          </div>

          <div
            style={{
              overflow: 'auto',
              padding: '24px 22px',
              maxWidth: 800,
              margin: '0 auto',
              width: '100%',
            }}
          >
            {CONV.map((m, i) => (
              <div key={i} style={{ marginBottom: 22 }}>
                <div
                  className='lbl'
                  style={{
                    marginBottom: 6,
                    color:
                      m.who === 'user' ? 'var(--hf-ink-3)' : 'var(--hf-accent)',
                  }}
                >
                  {m.who === 'user' ? 'you' : 'claude-3.5-sonnet'}
                </div>
                <div
                  style={{
                    fontSize: 14,
                    lineHeight: 1.65,
                    color: 'var(--hf-ink)',
                    whiteSpace: 'pre-wrap',
                  }}
                >
                  {m.t.split(/(```[\s\S]+?```)/g).map((seg, j) =>
                    seg.startsWith('```') ? (
                      <pre
                        key={j}
                        className='mono'
                        style={{
                          background: 'var(--hf-paper)',
                          border: '1px solid var(--hf-rule)',
                          padding: 14,
                          fontSize: 11,
                          margin: '10px 0',
                          overflow: 'auto',
                        }}
                      >
                        {seg.replace(/```\w*\n?/g, '').replace(/```$/, '')}
                      </pre>
                    ) : (
                      <span key={j}>{seg}</span>
                    ),
                  )}
                </div>
                {m.meta && (
                  <div
                    style={{
                      display: 'flex',
                      gap: 12,
                      marginTop: 8,
                      fontSize: 10,
                    }}
                  >
                    <span className='faint mono'>ttft {m.meta.ttft}ms</span>
                    <span className='faint mono'>total {m.meta.total}ms</span>
                    <span className='faint mono'>{m.meta.tok}t</span>
                    <span className='faint mono'>
                      ${m.meta.cost.toFixed(4)}
                    </span>
                    <span style={{ flex: 1 }} />
                    <button type='button' className='btn ghost sm'>
                      copy
                    </button>
                    <button type='button' className='btn ghost sm'>
                      retry
                    </button>
                    <button type='button' className='btn ghost sm'>
                      branch
                    </button>
                  </div>
                )}
              </div>
            ))}
          </div>

          <div
            style={{
              padding: 22,
              borderTop: '1px solid var(--hf-rule)',
              background: 'var(--hf-paper)',
            }}
          >
            <div
              className='panel'
              style={{ padding: 14, maxWidth: 800, margin: '0 auto' }}
            >
              <div
                style={{
                  minHeight: 48,
                  fontSize: 13,
                  color: 'var(--hf-ink-3)',
                }}
              >
                ask anything…
              </div>
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  marginTop: 8,
                }}
              >
                <button type='button' className='btn ghost sm'>
                  ＋ attach
                </button>
                <button type='button' className='btn ghost sm'>
                  ⊞ tools
                </button>
                <span style={{ flex: 1 }} />
                <span className='muted mono' style={{ fontSize: 10 }}>
                  0 / 200,000
                </span>
                <button type='button' className='btn primary'>
                  ▶ send{' '}
                  <span className='kbd' style={{ marginLeft: 4 }}>
                    ↵
                  </span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </HFShell>
  );
};

export default HFChat;
