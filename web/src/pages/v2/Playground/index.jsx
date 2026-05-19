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
import { API, showError } from '../../../helpers';
import { useFormDraft } from '../../../hooks/common/useFormDraft';

// HiFi 5 — Playground multi-model compare. Wired to
// POST /api/v2/:tenant_slug/playground/run (2026-05-19).

const DEFAULT_MODELS = ['gpt-4o', 'claude-3.5-sonnet', 'gemini-1.5-pro'];
const VENDOR_FOR = {
  'gpt-4o': 'OpenAI',
  'claude-3.5-sonnet': 'Anthropic',
  'gemini-1.5-pro': 'Google',
};

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

// Sort columns by latency so the fastest is leftmost — gives the user an
// at-a-glance perf comparison without an extra UI control. Stable: keeps
// original column index as tiebreaker so the layout doesn't reshuffle
// between identical runs.
const verdictFor = (item, allItems) => {
  if (item.error_code) return { label: 'error', color: 'var(--hf-err)' };
  const sorted = allItems
    .filter((x) => !x.error_code)
    .slice()
    .sort((a, b) => a.latency_ms - b.latency_ms);
  if (sorted.length && item === sorted[0])
    return { label: 'fastest', color: 'var(--hf-accent)' };
  if (sorted.length && item === sorted[sorted.length - 1])
    return { label: 'slowest', color: 'var(--hf-info)' };
  return { label: '', color: 'var(--hf-ink-2)' };
};

const HFPlayground = () => {
  const tenantSlug = useTenantSlug();

  // Form state — persisted via useFormDraft so a tab close mid-edit doesn't
  // lose the user's prompt. Key NOT scoped per-tenant (playground is a
  // user-level tool; same prompt likely reused across tenants).
  const [form, setForm, clearDraft, , /* isDirty */ restoredFromDraft] =
    useFormDraft('playground-form', {
      system: 'You are a helpful, concise assistant.',
      user: 'What is the capital of Australia? Briefly.',
      temperature: 0.7,
      top_p: 1.0,
      max_tokens: 1024,
      models: DEFAULT_MODELS,
    });
  const [running, setRunning] = useState(false);
  const [items, setItems] = useState(null); // null = never run; [] = ran with errors

  const update = (k) => (v) => setForm({ ...form, [k]: v });

  const runAll = useCallback(async () => {
    if (!form.user.trim()) {
      showError('User prompt cannot be empty');
      return;
    }
    setRunning(true);
    setItems(null);
    try {
      const res = await API.post(`/api/v2/${tenantSlug}/playground/run`, {
        system: form.system,
        user: form.user,
        models: form.models,
        params: {
          temperature: Number(form.temperature),
          top_p: Number(form.top_p),
          max_tokens: Math.max(1, parseInt(form.max_tokens, 10) || 1024),
        },
      });
      if (res?.data?.success) {
        setItems(res.data.data.items);
      } else {
        showError(res?.data?.message || 'Run failed');
      }
    } catch (err) {
      const msg = err?.response?.data?.message || err?.message || 'Run failed';
      showError(msg);
    } finally {
      setRunning(false);
    }
  }, [form, tenantSlug]);

  // ⌘/Ctrl + Enter triggers run from any focus position inside the page.
  useEffect(() => {
    const onKey = (e) => {
      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && !running) {
        e.preventDefault();
        runAll();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [runAll, running]);

  const displayCols = form.models.map((m, i) => {
    const found = items?.[i];
    return {
      idx: i,
      model: m,
      vendor: VENDOR_FOR[m] || '—',
      result: found || null,
    };
  });

  return (
    <HFShell
      active='playground'
      crumbs={['workspace', 'playground']}
      actions={
        <>
          <span className='lbl'>preset:</span>
          <button type='button' className='btn' disabled>
            blank ▾
          </button>
          <button type='button' className='btn' disabled>
            save
          </button>
          <button type='button' className='btn' disabled>
            share ↗
          </button>
        </>
      }
    >
      <div
        style={{
          display: 'grid',
          gridTemplateRows: 'auto auto 1fr auto',
          height: '100%',
        }}
      >
        {/* ── Header + prompt editor ── */}
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
            {form.models.length} models, one prompt, side by side
          </h1>

          {restoredFromDraft && (
            <div
              style={{
                marginTop: 10,
                fontSize: 11,
                padding: '4px 10px',
                borderLeft: '2px solid var(--hf-amber, #d97706)',
                background: 'var(--hf-amber-bg, rgba(217, 119, 6, 0.08))',
                color: 'var(--hf-ink-2)',
              }}
              data-testid='playground-restored-banner'
            >
              Restored from saved draft.{' '}
              <button
                type='button'
                className='btn ghost xs'
                onClick={clearDraft}
              >
                Discard
              </button>
            </div>
          )}

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
            <textarea
              data-testid='playground-system'
              value={form.system}
              onChange={(e) => update('system')(e.target.value)}
              rows={2}
              style={{
                width: '100%',
                fontFamily: 'var(--hf-mono)',
                fontSize: 12,
                padding: '6px 10px',
                border: '1px solid var(--hf-rule)',
                background: 'var(--hf-sunken)',
                color: 'var(--hf-ink)',
                borderRadius: 2,
                resize: 'vertical',
                outline: 'none',
              }}
            />
            <div
              className='lbl'
              style={{ alignSelf: 'flex-start', paddingTop: 6 }}
            >
              user
            </div>
            <textarea
              data-testid='playground-user'
              value={form.user}
              onChange={(e) => update('user')(e.target.value)}
              rows={3}
              style={{
                width: '100%',
                fontFamily: 'var(--hf-mono)',
                fontSize: 13,
                padding: '6px 10px',
                border: '1px solid var(--hf-rule)',
                background: 'var(--hf-sunken)',
                color: 'var(--hf-ink)',
                borderRadius: 2,
                resize: 'vertical',
                outline: 'none',
              }}
            />
          </div>

          <div
            style={{
              display: 'flex',
              gap: 8,
              marginTop: 14,
              alignItems: 'center',
              flexWrap: 'wrap',
            }}
          >
            <span className='lbl'>params</span>
            <label className='pill' style={{ display: 'inline-flex', gap: 6 }}>
              temp ·
              <input
                data-testid='playground-temp'
                type='number'
                min='0'
                max='2'
                step='0.1'
                value={form.temperature}
                onChange={(e) => update('temperature')(e.target.value)}
                style={{
                  width: 48,
                  border: 'none',
                  background: 'transparent',
                  color: 'inherit',
                  outline: 'none',
                  fontFamily: 'var(--hf-mono)',
                }}
              />
            </label>
            <label className='pill' style={{ display: 'inline-flex', gap: 6 }}>
              top_p ·
              <input
                data-testid='playground-topp'
                type='number'
                min='0'
                max='1'
                step='0.05'
                value={form.top_p}
                onChange={(e) => update('top_p')(e.target.value)}
                style={{
                  width: 48,
                  border: 'none',
                  background: 'transparent',
                  color: 'inherit',
                  outline: 'none',
                  fontFamily: 'var(--hf-mono)',
                }}
              />
            </label>
            <label className='pill' style={{ display: 'inline-flex', gap: 6 }}>
              max ·
              <input
                data-testid='playground-max'
                type='number'
                min='1'
                max='8192'
                step='128'
                value={form.max_tokens}
                onChange={(e) => update('max_tokens')(e.target.value)}
                style={{
                  width: 64,
                  border: 'none',
                  background: 'transparent',
                  color: 'inherit',
                  outline: 'none',
                  fontFamily: 'var(--hf-mono)',
                }}
              />
            </label>
            <span style={{ flex: 1 }} />
            <span className='kbd'>⌘</span>
            <span className='kbd'>↵</span>
            <button
              type='button'
              className='btn acc'
              disabled={running}
              onClick={runAll}
              data-testid='playground-run'
            >
              {running ? '▶ running…' : `▶ run all ${form.models.length}`}
            </button>
          </div>
        </div>

        {/* ── Model header strip ── */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: `repeat(${form.models.length}, 1fr)`,
            borderBottom: '1px solid var(--hf-rule)',
          }}
        >
          {displayCols.map((c) => {
            const v = c.result
              ? verdictFor(c.result, items || [])
              : { label: '', color: 'var(--hf-ink-2)' };
            return (
              <div
                key={c.idx}
                style={{
                  padding: 16,
                  borderRight:
                    c.idx < form.models.length - 1
                      ? '1px solid var(--hf-rule)'
                      : 0,
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
                      {c.model}
                    </div>
                    <div className='faint mono' style={{ fontSize: 10 }}>
                      {c.vendor}
                    </div>
                  </div>
                  <button type='button' className='btn ghost sm' disabled>
                    swap ▾
                  </button>
                </div>
                <div style={{ display: 'flex', gap: 8, marginTop: 10 }}>
                  {c.result ? (
                    <>
                      <span className='pill'>
                        <span
                          className={`dot ${
                            c.result.error_code ? 'err' : 'ok'
                          }`}
                        />
                        {c.result.latency_ms}ms
                      </span>
                      <span className='pill'>
                        tok {c.result.prompt_tokens}↗
                        {c.result.completion_tokens}
                      </span>
                    </>
                  ) : (
                    <span className='pill faint'>
                      {running ? 'running…' : 'idle'}
                    </span>
                  )}
                </div>
                {v.label && (
                  <div
                    className='lbl'
                    style={{ marginTop: 10, color: v.color }}
                  >
                    {v.label}
                  </div>
                )}
              </div>
            );
          })}
        </div>

        {/* ── Output panels ── */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: `repeat(${form.models.length}, 1fr)`,
            overflow: 'hidden',
          }}
        >
          {displayCols.map((c) => (
            <div
              key={c.idx}
              data-testid={`playground-col-${c.idx}`}
              style={{
                padding: 18,
                borderRight:
                  c.idx < form.models.length - 1
                    ? '1px solid var(--hf-rule)'
                    : 0,
                overflow: 'auto',
                fontSize: 13,
                lineHeight: 1.6,
                color: c.result?.error_code
                  ? 'var(--hf-err)'
                  : 'var(--hf-ink-2)',
                whiteSpace: 'pre-wrap',
              }}
            >
              {!c.result && running && (
                <span className='muted'>Awaiting response…</span>
              )}
              {!c.result && !running && (
                <span className='muted'>
                  Press ⌘/Ctrl + Enter or click "run all" to compare.
                </span>
              )}
              {c.result?.error_code && (
                <>
                  <div className='strong'>Error · {c.result.error_code}</div>
                  <div style={{ marginTop: 6 }}>
                    {c.result.error_message || '(no message)'}
                  </div>
                </>
              )}
              {c.result && !c.result.error_code && c.result.content}
            </div>
          ))}
        </div>

        {/* ── Footer (token usage + copy) ── */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: `repeat(${form.models.length}, 1fr)`,
            borderTop: '1px solid var(--hf-rule)',
            background: 'var(--hf-paper)',
          }}
        >
          {displayCols.map((c) => (
            <div
              key={c.idx}
              style={{
                padding: '10px 16px',
                borderRight:
                  c.idx < form.models.length - 1
                    ? '1px solid var(--hf-rule)'
                    : 0,
                display: 'flex',
                alignItems: 'center',
                gap: 10,
                fontSize: 11,
              }}
            >
              <span className='muted mono'>
                {c.result
                  ? `${c.result.prompt_tokens + c.result.completion_tokens} tok`
                  : '—'}
              </span>
              <span style={{ flex: 1 }} />
              <button
                type='button'
                className='btn ghost sm'
                disabled={!c.result?.content}
                onClick={() => {
                  if (!c.result?.content) return;
                  navigator.clipboard?.writeText(c.result.content);
                }}
              >
                copy
              </button>
            </div>
          ))}
        </div>
      </div>
    </HFShell>
  );
};

export default HFPlayground;
