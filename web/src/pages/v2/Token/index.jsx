import React, { useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';

/*
 * HiFi 4 — Token mgmt + integration snippets.
 * Ported from design canvas hifi/hf4-token.jsx (2026-05-07).
 */

const TOKENS = [
  {
    name: 'lurus-chat · prod',
    id: 'sk-...e23a',
    used: 0.62,
    used$: 124.2,
    cap$: 200,
    models: 'gpt-4o, claude-3.5-sonnet',
    last: '32s ago',
    state: 'live',
  },
  {
    name: 'lurus-edit · staging',
    id: 'sk-...8f10',
    used: 0.18,
    used$: 18.4,
    cap$: 100,
    models: 'gpt-4o-mini',
    last: '4m ago',
    state: 'live',
  },
  {
    name: 'public-demo',
    id: 'sk-...1cc4',
    used: 0.94,
    used$: 94.8,
    cap$: 100,
    models: 'gpt-4o-mini, gemini-flash',
    last: '2m ago',
    state: 'near-cap',
  },
  {
    name: 'andy-debug',
    id: 'sk-...77ab',
    used: 0.04,
    used$: 4.2,
    cap$: 50,
    models: 'all',
    last: '1h ago',
    state: 'live',
  },
  {
    name: 'old-batch-2024',
    id: 'sk-...0001',
    used: 0.0,
    used$: 0.0,
    cap$: 0,
    models: '—',
    last: '14d ago',
    state: 'rotated',
  },
];

const SNIPPETS = {
  curl: `curl https://api.lurus.cn/v1/chat/completions \\
  -H "Authorization: Bearer $LURUS_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-4o",
    "messages": [{"role":"user","content":"hi"}]
  }'`,
  python: `from openai import OpenAI

client = OpenAI(
    api_key="sk-...e23a",
    base_url="https://api.lurus.cn/v1",
)

resp = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role":"user","content":"hi"}],
)
print(resp.choices[0].message.content)`,
  node: `import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "sk-...e23a",
  baseURL: "https://api.lurus.cn/v1",
});

const r = await client.chat.completions.create({
  model: "gpt-4o",
  messages: [{ role: "user", content: "hi" }],
});`,
  anthropic: `import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({
  apiKey: "sk-...e23a",
  baseURL: "https://api.lurus.cn",
});

await client.messages.create({
  model: "claude-3.5-sonnet",
  max_tokens: 1024,
  messages: [{ role: "user", content: "hi" }],
});`,
};

const SETTINGS = [
  ['model scope', 'gpt-4o, claude-3.5-sonnet · 2 selected'],
  ['monthly cap', '$200.00'],
  ['rate limit', '60 rpm · 200k tpm'],
  ['expires', 'never'],
  ['allowed ips', 'any'],
];

const HFToken = () => {
  const [lang, setLang] = useState('curl');
  const [sel, setSel] = useState(0);

  return (
    <HFShell
      active='tokens'
      crumbs={['my account', 'tokens']}
      actions={
        <>
          <span className='muted mono' style={{ fontSize: 11 }}>
            5 active · $241.60 / $450
          </span>
          <button type='button' className='btn primary'>
            + new token
          </button>
        </>
      }
    >
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '380px 1fr',
          minHeight: 0,
          height: '100%',
        }}
      >
        <div
          style={{
            borderRight: '1px solid var(--hf-rule)',
            overflow: 'auto',
            background: 'var(--hf-paper)',
          }}
        >
          <div
            style={{
              padding: '20px 22px',
              borderBottom: '1px solid var(--hf-rule)',
            }}
          >
            <div className='lbl' style={{ marginBottom: 4 }}>
              your tokens
            </div>
            <div className='display' style={{ fontSize: 26 }}>
              5 active
            </div>
          </div>
          {TOKENS.map((t, i) => (
            <div
              key={i}
              onClick={() => setSel(i)}
              style={{
                padding: '14px 22px',
                borderBottom: '1px solid var(--hf-rule)',
                cursor: 'pointer',
                background: sel === i ? 'var(--hf-elev)' : 'transparent',
                borderLeft:
                  sel === i
                    ? '2px solid var(--hf-accent)'
                    : '2px solid transparent',
              }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'baseline',
                  justifyContent: 'space-between',
                }}
              >
                <span className='strong' style={{ fontSize: 13 }}>
                  {t.name}
                </span>
                <span
                  className={
                    'tag ' +
                    (t.state === 'live'
                      ? 'ok'
                      : t.state === 'near-cap'
                        ? 'warn'
                        : '')
                  }
                >
                  {t.state}
                </span>
              </div>
              <div
                className='mono muted'
                style={{ fontSize: 10, marginTop: 4 }}
              >
                {t.id} · {t.last}
              </div>
              <div className='muted' style={{ fontSize: 11, marginTop: 2 }}>
                {t.models}
              </div>
              {t.cap$ > 0 && (
                <div style={{ marginTop: 8 }}>
                  <div
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      fontSize: 10,
                      marginBottom: 3,
                    }}
                  >
                    <span className='mono muted'>
                      ${t.used$.toFixed(2)} / ${t.cap$}
                    </span>
                    <span
                      className={'mono ' + (t.used > 0.9 ? 'acc' : 'muted')}
                    >
                      {(t.used * 100).toFixed(0)}%
                    </span>
                  </div>
                  <div style={{ height: 3, background: 'var(--hf-sunken)' }}>
                    <div
                      style={{
                        height: '100%',
                        width: t.used * 100 + '%',
                        background:
                          t.used > 0.9 ? 'var(--hf-accent)' : 'var(--hf-ink-2)',
                      }}
                    />
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>

        <div style={{ overflow: 'auto', padding: 28 }}>
          <div className='lbl' style={{ marginBottom: 4 }}>
            integration · {TOKENS[sel].name}
          </div>
          <h1
            className='display'
            style={{ fontSize: 36, margin: 0, letterSpacing: '-0.025em' }}
          >
            Drop into your stack
          </h1>
          <div className='muted' style={{ marginTop: 6 }}>
            OpenAI-compatible. Same SDK. Swap the base URL.
          </div>

          <div className='panel' style={{ marginTop: 22, padding: 16 }}>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: '110px 1fr auto',
                gap: 14,
                alignItems: 'center',
              }}
            >
              <span className='lbl'>base url</span>
              <span className='mono strong' style={{ fontSize: 13 }}>
                https://api.lurus.cn/v1
              </span>
              <button type='button' className='btn sm'>
                copy
              </button>
            </div>
            <hr
              style={{
                border: 0,
                borderTop: '1px dashed var(--hf-rule)',
                margin: '12px 0',
              }}
            />
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: '110px 1fr auto',
                gap: 14,
                alignItems: 'center',
              }}
            >
              <span className='lbl'>api key</span>
              <span className='mono strong' style={{ fontSize: 13 }}>
                {TOKENS[sel].id}
              </span>
              <button type='button' className='btn sm'>
                reveal
              </button>
            </div>
          </div>

          <div
            style={{
              display: 'flex',
              gap: 0,
              marginTop: 22,
              borderBottom: '1px solid var(--hf-rule)',
            }}
          >
            {[
              ['curl', 'cURL'],
              ['python', 'Python'],
              ['node', 'Node.js'],
              ['anthropic', 'Anthropic SDK'],
            ].map(([k, l]) => (
              <button
                key={k}
                type='button'
                onClick={() => setLang(k)}
                style={{
                  padding: '10px 16px',
                  border: 0,
                  background: 'transparent',
                  cursor: 'pointer',
                  fontFamily: 'var(--hf-mono)',
                  fontSize: 11,
                  color: lang === k ? 'var(--hf-ink)' : 'var(--hf-ink-3)',
                  borderBottom:
                    lang === k
                      ? '2px solid var(--hf-accent)'
                      : '2px solid transparent',
                  marginBottom: -1,
                }}
              >
                {l}
              </button>
            ))}
            <span style={{ flex: 1 }} />
            <button
              type='button'
              className='btn ghost sm'
              style={{ alignSelf: 'center' }}
            >
              copy ⧉
            </button>
          </div>
          <pre
            className='mono'
            style={{
              margin: 0,
              padding: 18,
              fontSize: 11,
              background: 'var(--hf-paper)',
              border: '1px solid var(--hf-rule)',
              borderTop: 0,
              color: 'var(--hf-ink-2)',
              whiteSpace: 'pre',
              overflow: 'auto',
            }}
          >
            {SNIPPETS[lang]}
          </pre>

          <div className='lbl' style={{ marginTop: 24, marginBottom: 8 }}>
            token settings
          </div>
          <div className='panel'>
            {SETTINGS.map((r, i, arr) => (
              <div
                key={i}
                style={{
                  display: 'grid',
                  gridTemplateColumns: '160px 1fr auto',
                  padding: '12px 16px',
                  borderBottom:
                    i < arr.length - 1 ? '1px dashed var(--hf-rule)' : 0,
                  fontSize: 12,
                }}
              >
                <span className='lbl' style={{ alignSelf: 'center' }}>
                  {r[0]}
                </span>
                <span className='strong'>{r[1]}</span>
                <button type='button' className='btn ghost sm'>
                  edit
                </button>
              </div>
            ))}
          </div>

          <div style={{ display: 'flex', gap: 8, marginTop: 18 }}>
            <button type='button' className='btn'>
              rotate key
            </button>
            <button type='button' className='btn'>
              view logs
            </button>
            <button type='button' className='btn'>
              test in playground
            </button>
            <span style={{ flex: 1 }} />
            <button
              type='button'
              className='btn'
              style={{ color: 'var(--hf-err)', borderColor: 'var(--hf-err)' }}
            >
              revoke
            </button>
          </div>
        </div>
      </div>
    </HFShell>
  );
};

export default HFToken;
