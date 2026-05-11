import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import HFShell from '../../../components/hifi/HFShell';
import { API, showError, showSuccess } from '../../../helpers';

const BASE_URL = 'https://api.lurus.cn/v1';
const QUOTA_PER_USD = 500_000;

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

const quotaToUSD = (q) => (q / QUOTA_PER_USD).toFixed(2);

const tokenStatus = (t) => {
  if (!t || t.Status !== 1) return 'disabled';
  if (t.ExpiredTime > 0 && t.ExpiredTime < Math.floor(Date.now() / 1000)) return 'expired';
  if (!t.UnlimitedQuota) {
    const total = t.UsedQuota + t.RemainQuota;
    if (t.RemainQuota <= 0 || (total > 0 && t.UsedQuota / total >= 0.9)) return 'near-cap';
  }
  return 'live';
};

const maskKey = (key) => (key ? `sk-...${key.slice(-4)}` : 'sk-...????');

const relTime = (ts) => {
  if (!ts) return '—';
  const s = Math.floor(Date.now() / 1000) - ts;
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
};

const fmtExpiry = (ts) => {
  if (!ts || ts === -1) return 'never';
  return new Date(ts * 1000).toLocaleDateString();
};

const fmtModels = (t) =>
  t.ModelLimitsEnabled && t.ModelLimits ? t.ModelLimits : 'all models';

const fmtIPs = (ip) => (!ip ? 'any' : ip);

const copy = async (text) => {
  try {
    await navigator.clipboard.writeText(text);
    showSuccess('Copied');
  } catch (_) {
    showError('Copy failed');
  }
};

const buildSnippets = (key) => ({
  curl: `curl ${BASE_URL}/chat/completions \\
  -H "Authorization: Bearer ${key}" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}'`,
  python: `from openai import OpenAI

client = OpenAI(api_key="${key}", base_url="${BASE_URL}")
resp = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "hi"}],
)
print(resp.choices[0].message.content)`,
  node: `import OpenAI from "openai";

const client = new OpenAI({ apiKey: "${key}", baseURL: "${BASE_URL}" });
const r = await client.chat.completions.create({
  model: "gpt-4o",
  messages: [{ role: "user", content: "hi" }],
});`,
  anthropic: `import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({
  apiKey: "${key}",
  baseURL: "https://api.lurus.cn",
});
await client.messages.create({
  model: "claude-3.5-sonnet",
  max_tokens: 1024,
  messages: [{ role: "user", content: "hi" }],
});`,
});

// ─── Create token modal ───────────────────────────────────────────────────────

const CreateModal = ({ tenantSlug, onCreated, onClose }) => {
  const [form, setForm] = useState({
    name: '',
    cap: '',
    unlimited: true,
    models: '',
    limitModels: false,
  });
  const [saving, setSaving] = useState(false);
  const nameRef = useRef(null);

  useEffect(() => {
    nameRef.current?.focus();
  }, []);

  const submit = async (e) => {
    e.preventDefault();
    if (!form.name.trim()) return;
    setSaving(true);
    try {
      const capUSD = parseFloat(form.cap) || 0;
      const res = await API.post(`/api/v2/${tenantSlug}/tokens`, {
        name: form.name.trim(),
        unlimited_quota: form.unlimited || capUSD <= 0,
        remain_quota: form.unlimited || capUSD <= 0 ? 0 : Math.round(capUSD * QUOTA_PER_USD),
        model_limits_enabled: form.limitModels && !!form.models.trim(),
        model_limits: form.models.trim(),
        expired_time: -1,
      });
      if (res?.data?.success) {
        const { key } = res.data.data;
        showSuccess('Token created — copy your key now, it won\'t be shown again.');
        onCreated(key);
      }
    } catch (_) {
      // error toast shown by API interceptor
    } finally {
      setSaving(false);
    }
  };

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.45)',
        zIndex: 500,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
      onClick={(e) => e.target === e.currentTarget && onClose()}
    >
      <form
        onSubmit={submit}
        style={{
          background: 'var(--hf-paper)',
          border: '1px solid var(--hf-rule)',
          borderRadius: 4,
          padding: 28,
          width: 400,
          display: 'flex',
          flexDirection: 'column',
          gap: 14,
        }}
      >
        <div className='strong' style={{ fontSize: 15 }}>New token</div>

        <label style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
          <span className='lbl'>name *</span>
          <input
            ref={nameRef}
            style={{ fontFamily: 'var(--hf-mono)', fontSize: 12, padding: '5px 8px', border: '1px solid var(--hf-rule)', background: 'var(--hf-sunken)', color: 'var(--hf-ink)', borderRadius: 2, outline: 'none', width: '100%' }}
            placeholder='e.g. prod-backend'
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            required
          />
        </label>

        <label style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <input
            type='checkbox'
            checked={form.unlimited}
            onChange={(e) => setForm((f) => ({ ...f, unlimited: e.target.checked }))}
          />
          <span className='lbl'>unlimited quota</span>
        </label>

        {!form.unlimited && (
          <label style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
            <span className='lbl'>monthly cap ($)</span>
            <input
              style={{ fontFamily: 'var(--hf-mono)', fontSize: 12, padding: '5px 8px', border: '1px solid var(--hf-rule)', background: 'var(--hf-sunken)', color: 'var(--hf-ink)', borderRadius: 2, outline: 'none', width: '100%' }}
              type='number'
              min='0'
              step='0.01'
              placeholder='200'
              value={form.cap}
              onChange={(e) => setForm((f) => ({ ...f, cap: e.target.value }))}
            />
          </label>
        )}

        <label style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <input
            type='checkbox'
            checked={form.limitModels}
            onChange={(e) => setForm((f) => ({ ...f, limitModels: e.target.checked }))}
          />
          <span className='lbl'>restrict models</span>
        </label>

        {form.limitModels && (
          <label style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
            <span className='lbl'>allowed models (comma-separated)</span>
            <input
              style={{ fontFamily: 'var(--hf-mono)', fontSize: 12, padding: '5px 8px', border: '1px solid var(--hf-rule)', background: 'var(--hf-sunken)', color: 'var(--hf-ink)', borderRadius: 2, outline: 'none', width: '100%' }}
              placeholder='gpt-4o, claude-3.5-sonnet'
              value={form.models}
              onChange={(e) => setForm((f) => ({ ...f, models: e.target.value }))}
            />
          </label>
        )}

        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 4 }}>
          <button type='button' className='btn ghost' onClick={onClose}>
            cancel
          </button>
          <button type='submit' className='btn primary' disabled={saving}>
            {saving ? 'creating…' : 'create token'}
          </button>
        </div>
      </form>
    </div>
  );
};

// ─── Inline setting editor ────────────────────────────────────────────────────

const InlineEdit = ({ value, onSave, onCancel }) => {
  const [v, setV] = useState(value);
  const ref = useRef(null);

  useEffect(() => {
    ref.current?.select();
  }, []);

  const commit = () => {
    if (v.trim() !== value) onSave(v.trim());
    else onCancel();
  };

  return (
    <input
      ref={ref}
      style={{ fontFamily: 'var(--hf-mono)', fontSize: 12, padding: '3px 6px', width: '100%', border: '1px solid var(--hf-rule)', background: 'var(--hf-sunken)', color: 'var(--hf-ink)', borderRadius: 2, outline: 'none' }}
      value={v}
      onChange={(e) => setV(e.target.value)}
      onKeyDown={(e) => {
        if (e.key === 'Enter') commit();
        if (e.key === 'Escape') onCancel();
      }}
      onBlur={commit}
    />
  );
};

// ─── Main page ────────────────────────────────────────────────────────────────

const LANG_TABS = [
  ['curl', 'cURL'],
  ['python', 'Python'],
  ['node', 'Node.js'],
  ['anthropic', 'Anthropic SDK'],
];

const HFToken = () => {
  const navigate = useNavigate();
  const tenantSlug = useTenantSlug();

  const [tokens, setTokens] = useState([]);
  const [loading, setLoading] = useState(true);
  const [sel, setSel] = useState(0);
  const [lang, setLang] = useState('curl');
  const [revealed, setRevealed] = useState(new Set());
  const [rotatedKeys, setRotatedKeys] = useState({}); // id → new sk-xxx key
  const [creating, setCreating] = useState(false);
  const [newlyCreatedKey, setNewlyCreatedKey] = useState(null);
  const [editField, setEditField] = useState(null); // 'models'|'cap'|'expires'|'ips'
  const [saving, setSaving] = useState(false);

  const fetchTokens = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get(`/api/v2/${tenantSlug}/tokens?p=1&size=100`);
      if (res?.data?.success) {
        setTokens(res.data.data.items ?? []);
        setSel(0);
      }
    } catch (_) {
    } finally {
      setLoading(false);
    }
  }, [tenantSlug]);

  useEffect(() => {
    if (tenantSlug) fetchTokens();
  }, [fetchTokens, tenantSlug]);

  const token = tokens[sel];

  // ── Computed summary ──────────────────────────────────────────────────────

  const activeCount = tokens.filter((t) => tokenStatus(t) === 'live').length;
  const totalUsedUSD = tokens.reduce((s, t) => s + t.UsedQuota / QUOTA_PER_USD, 0).toFixed(2);
  const totalCapUSD = tokens
    .filter((t) => !t.UnlimitedQuota)
    .reduce((s, t) => s + (t.UsedQuota + t.RemainQuota) / QUOTA_PER_USD, 0)
    .toFixed(2);

  // ── Per-token derived values ──────────────────────────────────────────────

  const getKey = (t) => {
    if (!t) return 'YOUR_KEY';
    if (rotatedKeys[t.Id]) return rotatedKeys[t.Id];
    return `sk-${t.Key}`;
  };

  const displayKey = (t) => {
    const full = getKey(t);
    if (revealed.has(t.Id) || rotatedKeys[t.Id]) return full;
    return maskKey(t.Key);
  };

  // ── Actions ───────────────────────────────────────────────────────────────

  const handleReveal = (t) => {
    setRevealed((prev) => {
      const next = new Set(prev);
      if (next.has(t.Id)) next.delete(t.Id);
      else next.add(t.Id);
      return next;
    });
  };

  const handleRotate = async () => {
    if (!token) return;
    if (!window.confirm(`Rotate key for "${token.Name}"? The old key stops working immediately.`)) return;
    setSaving(true);
    try {
      const res = await API.post(
        `/api/v2/${tenantSlug}/tokens/${token.Id}/rotate`,
        {},
        { skipErrorHandler: false }
      );
      if (res?.data?.success) {
        const newKey = res.data.data.key;
        setRotatedKeys((prev) => ({ ...prev, [token.Id]: newKey }));
        setRevealed((prev) => new Set([...prev, token.Id]));
        showSuccess('Key rotated — copy the new key now.');
      }
    } catch (_) {
    } finally {
      setSaving(false);
    }
  };

  const handleRevoke = async () => {
    if (!token) return;
    if (!window.confirm(`Revoke token "${token.Name}"? This cannot be undone.`)) return;
    setSaving(true);
    try {
      const res = await API.delete(`/api/v2/${tenantSlug}/tokens/${token.Id}`);
      if (res?.data?.success) {
        showSuccess('Token revoked');
        await fetchTokens();
      }
    } catch (_) {
    } finally {
      setSaving(false);
    }
  };

  const handleSaveField = async (field, rawValue) => {
    if (!token) return;
    setEditField(null);
    const body = {};
    if (field === 'models') {
      const v = rawValue.trim();
      body.model_limits_enabled = v !== '' && v !== 'all models';
      body.model_limits = body.model_limits_enabled ? v : '';
    } else if (field === 'cap') {
      const usd = parseFloat(rawValue) || 0;
      if (usd <= 0) {
        body.unlimited_quota = true;
        body.remain_quota = 0;
      } else {
        const newTotalUnits = Math.round(usd * QUOTA_PER_USD);
        body.unlimited_quota = false;
        body.remain_quota = Math.max(0, newTotalUnits - token.UsedQuota);
      }
    } else if (field === 'expires') {
      body.expired_time = rawValue === 'never' || rawValue === '' ? -1 : Math.floor(new Date(rawValue).getTime() / 1000);
    } else if (field === 'ips') {
      body.allow_ips = rawValue === 'any' ? '' : rawValue;
    }
    if (Object.keys(body).length === 0) return;
    setSaving(true);
    try {
      const res = await API.put(`/api/v2/${tenantSlug}/tokens/${token.Id}`, body);
      if (res?.data?.success) {
        showSuccess('Saved');
        await fetchTokens();
      }
    } catch (_) {
    } finally {
      setSaving(false);
    }
  };

  const handleCreated = async (key) => {
    setCreating(false);
    setNewlyCreatedKey(key);
    await fetchTokens();
  };

  // ── Render helpers ────────────────────────────────────────────────────────

  const statusClass = (st) =>
    st === 'live' ? 'tag ok' : st === 'near-cap' ? 'tag warn' : 'tag';

  const capDisplay = (t) => {
    if (t.UnlimitedQuota) return '∞';
    const total = t.UsedQuota + t.RemainQuota;
    return `$${quotaToUSD(t.UsedQuota)} / $${quotaToUSD(total)}`;
  };

  const capRatio = (t) => {
    if (t.UnlimitedQuota) return 0;
    const total = t.UsedQuota + t.RemainQuota;
    return total > 0 ? t.UsedQuota / total : 0;
  };

  const settingsRows = token
    ? [
        ['model scope', fmtModels(token), 'models'],
        [
          'monthly cap',
          token.UnlimitedQuota
            ? '∞'
            : `$${quotaToUSD(token.UsedQuota + token.RemainQuota)}`,
          'cap',
        ],
        ['expires', fmtExpiry(token.ExpiredTime), 'expires'],
        ['allowed ips', fmtIPs(token.AllowIps), 'ips'],
      ]
    : [];

  const snippetMap = buildSnippets(token ? getKey(token) : 'YOUR_KEY');

  // ─────────────────────────────────────────────────────────────────────────

  return (
    <HFShell
      active='tokens'
      crumbs={['my account', 'tokens']}
      actions={
        <>
          {loading ? (
            <span className='muted mono' style={{ fontSize: 11 }}>
              loading…
            </span>
          ) : (
            <span className='muted mono' style={{ fontSize: 11 }}>
              {activeCount} active · ${totalUsedUSD}
              {parseFloat(totalCapUSD) > 0 ? ` / $${totalCapUSD}` : ''}
            </span>
          )}
          <button
            type='button'
            className='btn primary'
            onClick={() => setCreating(true)}
          >
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
        {/* ── Left: token list ── */}
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
              {loading ? '…' : `${tokens.length} token${tokens.length !== 1 ? 's' : ''}`}
            </div>
          </div>

          {loading && (
            <div className='muted' style={{ padding: '20px 22px', fontSize: 12 }}>
              Loading…
            </div>
          )}

          {!loading && tokens.length === 0 && (
            <div className='muted' style={{ padding: '20px 22px', fontSize: 12 }}>
              No tokens yet. Create one to get started.
            </div>
          )}

          {tokens.map((t, i) => {
            const st = tokenStatus(t);
            const ratio = capRatio(t);
            return (
              <div
                key={t.Id}
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
                    {t.Name}
                  </span>
                  <span className={statusClass(st)}>{st}</span>
                </div>
                <div className='mono muted' style={{ fontSize: 10, marginTop: 4 }}>
                  {maskKey(t.Key)} · {relTime(t.AccessedTime)}
                </div>
                <div className='muted' style={{ fontSize: 11, marginTop: 2 }}>
                  {fmtModels(t)}
                </div>
                {!t.UnlimitedQuota && (t.UsedQuota + t.RemainQuota) > 0 && (
                  <div style={{ marginTop: 8 }}>
                    <div
                      style={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        fontSize: 10,
                        marginBottom: 3,
                      }}
                    >
                      <span className='mono muted'>{capDisplay(t)}</span>
                      <span className={'mono ' + (ratio > 0.9 ? 'acc' : 'muted')}>
                        {(ratio * 100).toFixed(0)}%
                      </span>
                    </div>
                    <div style={{ height: 3, background: 'var(--hf-sunken)' }}>
                      <div
                        style={{
                          height: '100%',
                          width: `${Math.min(ratio * 100, 100)}%`,
                          background:
                            ratio > 0.9 ? 'var(--hf-accent)' : 'var(--hf-ink-2)',
                        }}
                      />
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>

        {/* ── Right: token detail ── */}
        <div style={{ overflow: 'auto', padding: 28 }}>
          {!token && !loading && (
            <div className='muted' style={{ fontSize: 13 }}>
              Create a token to get started.
            </div>
          )}

          {token && (
            <>
              {newlyCreatedKey && (
                <div
                  className='panel'
                  style={{
                    marginBottom: 20,
                    padding: '12px 16px',
                    border: '1px solid var(--hf-ok)',
                    background: 'rgba(31,122,79,0.06)',
                  }}
                >
                  <div className='strong' style={{ fontSize: 12, marginBottom: 6, color: 'var(--hf-ok)' }}>
                    Token created — copy your key now
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <code className='mono' style={{ fontSize: 12, flex: 1, wordBreak: 'break-all' }}>
                      {newlyCreatedKey}
                    </code>
                    <button
                      type='button'
                      className='btn sm'
                      onClick={() => copy(newlyCreatedKey)}
                    >
                      copy
                    </button>
                    <button
                      type='button'
                      className='btn ghost sm'
                      onClick={() => setNewlyCreatedKey(null)}
                    >
                      ✕
                    </button>
                  </div>
                </div>
              )}

              <div className='lbl' style={{ marginBottom: 4 }}>
                integration · {token.Name}
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
                    {BASE_URL}
                  </span>
                  <button
                    type='button'
                    className='btn sm'
                    onClick={() => copy(BASE_URL)}
                  >
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
                    gridTemplateColumns: '110px 1fr auto auto',
                    gap: 14,
                    alignItems: 'center',
                  }}
                >
                  <span className='lbl'>api key</span>
                  <span
                    className='mono strong'
                    style={{ fontSize: 13, wordBreak: 'break-all' }}
                  >
                    {displayKey(token)}
                  </span>
                  <button
                    type='button'
                    className='btn sm'
                    onClick={() => handleReveal(token)}
                  >
                    {revealed.has(token.Id) || rotatedKeys[token.Id] ? 'hide' : 'reveal'}
                  </button>
                  <button
                    type='button'
                    className='btn sm'
                    onClick={() => copy(getKey(token))}
                  >
                    copy
                  </button>
                </div>
              </div>

              {/* Code snippets */}
              <div
                style={{
                  display: 'flex',
                  gap: 0,
                  marginTop: 22,
                  borderBottom: '1px solid var(--hf-rule)',
                }}
              >
                {LANG_TABS.map(([k, l]) => (
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
                  onClick={() => copy(snippetMap[lang])}
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
                {snippetMap[lang]}
              </pre>

              {/* Settings */}
              <div className='lbl' style={{ marginTop: 24, marginBottom: 8 }}>
                token settings
              </div>
              <div className='panel'>
                {settingsRows.map(([label, value, field], i, arr) => (
                  <div
                    key={field}
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '160px 1fr auto',
                      padding: '12px 16px',
                      borderBottom:
                        i < arr.length - 1 ? '1px dashed var(--hf-rule)' : 0,
                      fontSize: 12,
                      alignItems: 'center',
                    }}
                  >
                    <span className='lbl' style={{ alignSelf: 'center' }}>
                      {label}
                    </span>
                    {editField === field ? (
                      <InlineEdit
                        value={value}
                        onSave={(v) => handleSaveField(field, v)}
                        onCancel={() => setEditField(null)}
                      />
                    ) : (
                      <span className='strong'>{value}</span>
                    )}
                    {editField !== field && (
                      <button
                        type='button'
                        className='btn ghost sm'
                        disabled={saving}
                        onClick={() => setEditField(field)}
                      >
                        edit
                      </button>
                    )}
                  </div>
                ))}
              </div>

              {/* Actions */}
              <div style={{ display: 'flex', gap: 8, marginTop: 18 }}>
                <button
                  type='button'
                  className='btn'
                  disabled={saving}
                  onClick={handleRotate}
                >
                  rotate key
                </button>
                <button
                  type='button'
                  className='btn'
                  onClick={() => navigate('/console/v2/log')}
                >
                  view logs
                </button>
                <button
                  type='button'
                  className='btn'
                  onClick={() => navigate('/console/v2/playground')}
                >
                  test in playground
                </button>
                <span style={{ flex: 1 }} />
                <button
                  type='button'
                  className='btn'
                  disabled={saving}
                  style={{ color: 'var(--hf-err)', borderColor: 'var(--hf-err)' }}
                  onClick={handleRevoke}
                >
                  revoke
                </button>
              </div>
            </>
          )}
        </div>
      </div>

      {creating && (
        <CreateModal
          tenantSlug={tenantSlug}
          onCreated={handleCreated}
          onClose={() => setCreating(false)}
        />
      )}
    </HFShell>
  );
};

export default HFToken;
