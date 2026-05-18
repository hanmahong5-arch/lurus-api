import React, { useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';
import WIPBanner from '../../../components/hifi/WIPBanner';

/* HiFi 7 — Models catalog. Ported from hifi/hf7-models.jsx (2026-05-07 variants pack). */

const MODELS = [
  {
    name: 'gpt-4o',
    v: 'OpenAI',
    ctx: '128k',
    tier: 'flagship',
    in: 2.5,
    out: 10.0,
    latency: 412,
    succ: 99.4,
    modal: ['text', 'vision', 'audio'],
    badge: 'recommended',
  },
  {
    name: 'gpt-4o-mini',
    v: 'OpenAI',
    ctx: '128k',
    tier: 'fast',
    in: 0.15,
    out: 0.6,
    latency: 188,
    succ: 99.7,
    modal: ['text', 'vision'],
    badge: 'cheap',
  },
  {
    name: 'claude-3.5-sonnet',
    v: 'Anthropic',
    ctx: '200k',
    tier: 'flagship',
    in: 3.0,
    out: 15.0,
    latency: 295,
    succ: 98.9,
    modal: ['text', 'vision'],
    badge: 'best-quality',
  },
  {
    name: 'claude-3.5-haiku',
    v: 'Anthropic',
    ctx: '200k',
    tier: 'fast',
    in: 0.8,
    out: 4.0,
    latency: 210,
    succ: 99.5,
    modal: ['text'],
  },
  {
    name: 'claude-3-opus',
    v: 'Anthropic',
    ctx: '200k',
    tier: 'flagship',
    in: 15.0,
    out: 75.0,
    latency: 920,
    succ: 97.2,
    modal: ['text', 'vision'],
    badge: 'premium',
  },
  {
    name: 'gemini-1.5-pro',
    v: 'Google',
    ctx: '2M',
    tier: 'flagship',
    in: 1.25,
    out: 5.0,
    latency: 580,
    succ: 98.4,
    modal: ['text', 'vision', 'audio'],
    badge: 'long-ctx',
  },
  {
    name: 'gemini-1.5-flash',
    v: 'Google',
    ctx: '1M',
    tier: 'fast',
    in: 0.075,
    out: 0.3,
    latency: 140,
    succ: 99.6,
    modal: ['text', 'vision', 'audio'],
    badge: 'cheap',
  },
  {
    name: 'gemini-2.0-flash',
    v: 'Google',
    ctx: '1M',
    tier: 'fast',
    in: 0.1,
    out: 0.4,
    latency: 132,
    succ: 98.1,
    modal: ['text', 'vision'],
    badge: 'beta',
  },
  {
    name: 'glm-4-plus',
    v: 'Zhipu',
    ctx: '128k',
    tier: 'flagship',
    in: 0.7,
    out: 0.7,
    latency: 580,
    succ: 96.4,
    modal: ['text'],
  },
  {
    name: 'qwen-2.5-72b',
    v: 'Alibaba',
    ctx: '128k',
    tier: 'open',
    in: 0.4,
    out: 1.2,
    latency: 380,
    succ: 98.0,
    modal: ['text'],
  },
  {
    name: 'deepseek-v3',
    v: 'DeepSeek',
    ctx: '64k',
    tier: 'open',
    in: 0.27,
    out: 1.1,
    latency: 410,
    succ: 98.7,
    modal: ['text'],
    badge: 'trending',
  },
  {
    name: 'llama-3.3-70b',
    v: 'Meta',
    ctx: '128k',
    tier: 'open',
    in: 0.2,
    out: 0.3,
    latency: 320,
    succ: 99.0,
    modal: ['text'],
  },
];

const TIER_COLOR = {
  flagship: 'var(--hf-accent)',
  fast: 'var(--hf-ok)',
  open: 'var(--hf-info)',
};

const HFModels = () => {
  const [view] = useState('grid');
  const [vendor, setVendor] = useState('all');

  const vendors = ['all', ...Array.from(new Set(MODELS.map((m) => m.v)))];
  const filtered =
    vendor === 'all' ? MODELS : MODELS.filter((m) => m.v === vendor);

  return (
    <HFShell
      active='models'
      crumbs={['platform · admin', 'models']}
      actions={
        <>
          <button type='button' className='btn'>
            view: {view} ▾
          </button>
          <button type='button' className='btn primary'>
            + add model
          </button>
        </>
      }
    >
      <WIPBanner
        reason='Models catalog renders a static hi-fi mockup (MODELS const). Live data requires a /api/v2/{slug}/models endpoint that joins channel.models with pricing.'
        todo='Backend: /api/v2/{slug}/models aggregation; UI wires after.'
      />
      <div className='hf-page-head'>
        <div>
          <div className='lbl' style={{ marginBottom: 6 }}>
            catalog
          </div>
          <h1>
            54 models{' '}
            <span className='muted' style={{ fontWeight: 400 }}>
              across 8 vendors
            </span>
          </h1>
          <div className='sub'>
            unified pricing · normalized to USD · per 1M tokens
          </div>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 28 }}>
          {[
            ['flagship', '12', 'var(--hf-accent)'],
            ['fast', '24', 'var(--hf-ok)'],
            ['open', '18', 'var(--hf-info)'],
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
          display: 'flex',
          gap: 10,
          padding: '14px 28px',
          borderBottom: '1px solid var(--hf-rule)',
          background: 'var(--hf-paper)',
          alignItems: 'center',
          flexWrap: 'wrap',
        }}
      >
        <span className='lbl'>vendor</span>
        {vendors.map((v) => (
          <button
            key={v}
            type='button'
            onClick={() => setVendor(v)}
            className={'pill ' + (vendor === v ? 'solid' : '')}
            style={{
              cursor: 'pointer',
              border: '1px solid var(--hf-rule)',
              background: vendor === v ? 'var(--hf-ink)' : 'var(--hf-elev)',
              color: vendor === v ? 'var(--hf-bg)' : 'var(--hf-ink-2)',
            }}
          >
            {v}
          </button>
        ))}
        <span style={{ flex: 1 }} />
        <div className='field' style={{ width: 280 }}>
          <span className='faint'>⌕</span>
          <span className='muted'>filter by name, modality, context…</span>
        </div>
      </div>

      <div
        style={{
          padding: 24,
          display: 'grid',
          gridTemplateColumns: 'repeat(3, 1fr)',
          gap: 14,
        }}
      >
        {filtered.map((m, i) => (
          <div
            key={i}
            className='panel'
            style={{ padding: 18, position: 'relative', overflow: 'hidden' }}
          >
            {m.badge && (
              <span
                className='tag acc'
                style={{
                  position: 'absolute',
                  top: 14,
                  right: 14,
                  fontSize: 9,
                }}
              >
                {m.badge}
              </span>
            )}
            <div className='lbl' style={{ color: TIER_COLOR[m.tier] }}>
              ● {m.tier}
            </div>
            <div
              className='display'
              style={{ fontSize: 22, marginTop: 6, letterSpacing: '-0.025em' }}
            >
              {m.name}
            </div>
            <div className='muted mono' style={{ fontSize: 11, marginTop: 2 }}>
              {m.v} · {m.ctx} ctx
            </div>

            <div
              style={{
                display: 'grid',
                gridTemplateColumns: '1fr 1fr',
                gap: 10,
                marginTop: 16,
              }}
            >
              <div>
                <div className='lbl'>input · 1M</div>
                <div className='display' style={{ fontSize: 18 }}>
                  ${m.in.toFixed(2)}
                </div>
              </div>
              <div>
                <div className='lbl'>output · 1M</div>
                <div className='display' style={{ fontSize: 18 }}>
                  ${m.out.toFixed(2)}
                </div>
              </div>
            </div>

            <hr
              style={{
                border: 0,
                borderTop: '1px dashed var(--hf-rule)',
                margin: '14px 0',
              }}
            />

            <div style={{ display: 'flex', gap: 10, fontSize: 11 }}>
              <span>
                <span className='muted'>ttft</span>{' '}
                <span className='strong'>{m.latency}ms</span>
              </span>
              <span>
                <span className='muted'>succ</span>{' '}
                <span
                  style={{
                    color: m.succ >= 99 ? 'var(--hf-ok)' : 'var(--hf-warn)',
                  }}
                >
                  {m.succ}%
                </span>
              </span>
            </div>

            <div
              style={{
                display: 'flex',
                gap: 4,
                marginTop: 10,
                flexWrap: 'wrap',
              }}
            >
              {m.modal.map((x) => (
                <span key={x} className='pill' style={{ fontSize: 9 }}>
                  {x}
                </span>
              ))}
            </div>

            <div style={{ display: 'flex', gap: 6, marginTop: 14 }}>
              <button type='button' className='btn sm'>
                try ↗
              </button>
              <button type='button' className='btn sm'>
                docs
              </button>
              <span style={{ flex: 1 }} />
              <button type='button' className='btn sm primary'>
                enable
              </button>
            </div>
          </div>
        ))}
      </div>
    </HFShell>
  );
};

export default HFModels;
