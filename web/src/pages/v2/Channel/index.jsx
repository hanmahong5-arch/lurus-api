import React, { Fragment, useCallback, useEffect, useRef, useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';
import { API, showError, showSuccess } from '../../../helpers';

/*
 * HiFi 2 — Channel management wired to live backend.
 * Pattern follows Token page (web/src/pages/v2/Token/index.jsx).
 */

// ─── Tenant slug hook ─────────────────────────────────────────────────────────

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

// ─── Status helpers ───────────────────────────────────────────────────────────

const channelStatus = (ch) => {
  if (ch.Status === 1) return 'ok';
  if (ch.Status === 2) return 'disabled';
  return 'error';
};

const statusDot = (st) =>
  st === 'ok' ? 'dot ok' : st === 'disabled' ? 'dot warn' : 'dot err';

const statusTag = (st) =>
  st === 'ok' ? 'tag ok' : st === 'disabled' ? 'tag warn' : 'tag';

// ─── Model mapping parser ─────────────────────────────────────────────────────

// ModelMapping is stored as "from1=to1,from2=to2" or JSON string.
const parseMapping = (raw) => {
  if (!raw) return [];
  try {
    const obj = JSON.parse(raw);
    return Object.entries(obj).map(([from, to]) => [from, to]);
  } catch (_) {}
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
    .map((s) => {
      const [from, to] = s.split('=').map((x) => x.trim());
      return [from || s, to || s];
    });
};

// ─── Inline field editor ──────────────────────────────────────────────────────

const InlineEdit = ({ value, multiline, onSave, onCancel }) => {
  const [v, setV] = useState(value);
  const ref = useRef(null);

  useEffect(() => {
    ref.current?.focus();
    if (!multiline) ref.current?.select();
  }, [multiline]);

  const commit = () => {
    const trimmed = v.trim();
    if (trimmed !== value.trim()) onSave(trimmed);
    else onCancel();
  };

  const inputStyle = {
    fontFamily: 'var(--hf-mono)',
    fontSize: 11,
    padding: '3px 6px',
    width: '100%',
    border: '1px solid var(--hf-rule)',
    background: 'var(--hf-sunken)',
    color: 'var(--hf-ink)',
    borderRadius: 2,
    outline: 'none',
    resize: 'vertical',
  };

  if (multiline) {
    return (
      <textarea
        ref={ref}
        rows={3}
        style={inputStyle}
        value={v}
        onChange={(e) => setV(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Escape') onCancel();
        }}
        onBlur={commit}
      />
    );
  }
  return (
    <input
      ref={ref}
      style={inputStyle}
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

// ─── Create / Edit modal ──────────────────────────────────────────────────────

const EMPTY_FORM = {
  name: '',
  key: '',
  type: 1,
  baseURL: '',
  models: '',
  group: 'default',
  weight: 1,
  priority: 0,
  modelMapping: '',
  tag: '',
  remark: '',
};

const ChannelModal = ({ tenantSlug, existing, onDone, onClose }) => {
  const [form, setForm] = useState(
    existing
      ? {
          name: existing.Name ?? '',
          key: '',
          type: existing.Type ?? 1,
          baseURL: existing.BaseURL ?? '',
          models: Array.isArray(existing.Models)
            ? existing.Models.join(',')
            : existing.Models ?? '',
          group: existing.Group ?? 'default',
          weight: existing.Weight ?? 1,
          priority: existing.Priority ?? 0,
          modelMapping: existing.ModelMapping ?? '',
          tag: existing.Tag ?? '',
          remark: existing.Remark ?? '',
        }
      : { ...EMPTY_FORM }
  );
  const [saving, setSaving] = useState(false);
  const nameRef = useRef(null);

  useEffect(() => {
    nameRef.current?.focus();
  }, []);

  const set = (field) => (e) =>
    setForm((f) => ({ ...f, [field]: e.target.value }));

  const submit = async (e) => {
    e.preventDefault();
    if (!form.name.trim()) return;
    setSaving(true);
    try {
      const body = {
        name: form.name.trim(),
        type: Number(form.type),
        baseURL: form.baseURL.trim(),
        models: form.models.trim(),
        group: form.group.trim() || 'default',
        weight: Number(form.weight) || 1,
        priority: Number(form.priority) || 0,
        modelMapping: form.modelMapping.trim(),
        tag: form.tag.trim(),
        remark: form.remark.trim(),
      };
      if (!existing && form.key.trim()) body.key = form.key.trim();

      let res;
      if (existing) {
        res = await API.put(`/api/v2/${tenantSlug}/channels/${existing.Id}`, body);
      } else {
        res = await API.post(`/api/v2/${tenantSlug}/channels`, body);
      }

      if (res?.data?.success) {
        showSuccess(existing ? 'Channel updated' : 'Channel created');
        onDone();
      }
    } catch (_) {
    } finally {
      setSaving(false);
    }
  };

  const inputStyle = {
    fontFamily: 'var(--hf-mono)',
    fontSize: 11,
    padding: '5px 8px',
    border: '1px solid var(--hf-rule)',
    background: 'var(--hf-sunken)',
    color: 'var(--hf-ink)',
    borderRadius: 2,
    outline: 'none',
    width: '100%',
  };

  const field = (label, key, extra = {}) => (
    <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <span className='lbl'>{label}</span>
      <input
        style={inputStyle}
        value={form[key]}
        onChange={set(key)}
        {...extra}
      />
    </label>
  );

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
          width: 480,
          display: 'flex',
          flexDirection: 'column',
          gap: 12,
          maxHeight: '90vh',
          overflowY: 'auto',
        }}
      >
        <div className='strong' style={{ fontSize: 15 }}>
          {existing ? `Edit · ${existing.Name}` : 'New channel'}
        </div>

        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span className='lbl'>name *</span>
          <input
            ref={nameRef}
            style={inputStyle}
            value={form.name}
            onChange={set('name')}
            placeholder='e.g. openai/main'
            required
          />
        </label>

        {!existing && (
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            <span className='lbl'>api key</span>
            <input
              style={inputStyle}
              value={form.key}
              onChange={set('key')}
              placeholder='sk-...'
            />
          </label>
        )}

        {field('base url', 'baseURL', { placeholder: 'https://api.openai.com/v1' })}

        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span className='lbl'>type (int)</span>
          <input
            style={inputStyle}
            type='number'
            min='1'
            value={form.type}
            onChange={set('type')}
          />
        </label>

        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span className='lbl'>models (comma-separated)</span>
          <input
            style={inputStyle}
            value={form.models}
            onChange={set('models')}
            placeholder='gpt-4o,gpt-4o-mini'
          />
        </label>

        {field('group', 'group', { placeholder: 'default' })}

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            <span className='lbl'>weight</span>
            <input
              style={inputStyle}
              type='number'
              min='0'
              step='0.1'
              value={form.weight}
              onChange={set('weight')}
            />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            <span className='lbl'>priority</span>
            <input
              style={inputStyle}
              type='number'
              value={form.priority}
              onChange={set('priority')}
            />
          </label>
        </div>

        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span className='lbl'>model mapping (from=to, comma-separated or JSON)</span>
          <input
            style={inputStyle}
            value={form.modelMapping}
            onChange={set('modelMapping')}
            placeholder='gpt-4=gpt-4o,gpt-3.5-turbo=gpt-4o-mini'
          />
        </label>

        {field('tag', 'tag', { placeholder: 'optional tag' })}

        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span className='lbl'>remark</span>
          <input
            style={inputStyle}
            value={form.remark}
            onChange={set('remark')}
            placeholder='optional note'
          />
        </label>

        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 4 }}>
          <button type='button' className='btn ghost' onClick={onClose}>
            cancel
          </button>
          <button type='submit' className='btn primary' disabled={saving}>
            {saving ? 'saving…' : existing ? 'save changes' : 'create channel'}
          </button>
        </div>
      </form>
    </div>
  );
};

// ─── Expanded row detail ──────────────────────────────────────────────────────

const ExpandedRow = ({ channel, tenantSlug, onRefresh }) => {
  const [editField, setEditField] = useState(null);
  const [saving, setSaving] = useState(false);

  const mappings = parseMapping(channel.ModelMapping);

  const modelsList = (() => {
    if (!channel.Models) return [];
    if (Array.isArray(channel.Models)) return channel.Models;
    return channel.Models.split(',').map((m) => m.trim()).filter(Boolean);
  })();

  const saveField = async (field, value) => {
    setEditField(null);
    const body = {};
    if (field === 'baseURL') body.baseURL = value;
    else if (field === 'group') body.group = value || 'default';
    else if (field === 'weight') body.weight = Number(value) || 1;
    else if (field === 'priority') body.priority = Number(value) || 0;
    else if (field === 'remark') body.remark = value;
    else if (field === 'models') body.models = value;
    else if (field === 'modelMapping') body.modelMapping = value;
    if (Object.keys(body).length === 0) return;
    setSaving(true);
    try {
      const res = await API.put(`/api/v2/${tenantSlug}/channels/${channel.Id}`, body);
      if (res?.data?.success) {
        showSuccess('Saved');
        onRefresh();
      }
    } catch (_) {
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!window.confirm(`Delete channel "${channel.Name}"? This cannot be undone.`)) return;
    setSaving(true);
    try {
      const res = await API.delete(`/api/v2/${tenantSlug}/channels/${channel.Id}`);
      if (res?.data?.success) {
        showSuccess('Channel deleted');
        onRefresh();
      }
    } catch (_) {
    } finally {
      setSaving(false);
    }
  };

  const toggleStatus = async () => {
    const newStatus = channel.Status === 1 ? 2 : 1;
    setSaving(true);
    try {
      const res = await API.put(`/api/v2/${tenantSlug}/channels/${channel.Id}`, { status: newStatus });
      if (res?.data?.success) {
        showSuccess(newStatus === 1 ? 'Channel enabled' : 'Channel disabled');
        onRefresh();
      }
    } catch (_) {
    } finally {
      setSaving(false);
    }
  };

  const row = (label, value, field, multiline) => (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: '110px 1fr auto',
        padding: '8px 0',
        borderBottom: '1px dashed var(--hf-rule)',
        fontSize: 11,
        alignItems: 'center',
        gap: 8,
      }}
    >
      <span className='lbl'>{label}</span>
      {editField === field ? (
        <InlineEdit
          value={String(value ?? '')}
          multiline={multiline}
          onSave={(v) => saveField(field, v)}
          onCancel={() => setEditField(null)}
        />
      ) : (
        <span className='mono' style={{ fontSize: 11, wordBreak: 'break-all' }}>
          {value !== '' && value != null ? String(value) : '—'}
        </span>
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
  );

  return (
    <div style={{ padding: 20, display: 'grid', gridTemplateColumns: '1.4fr 1fr 1fr', gap: 20 }}>
      {/* Column 1: settings */}
      <div>
        <div className='lbl' style={{ marginBottom: 8 }}>channel settings</div>
        <div>
          {row('base url', channel.BaseURL, 'baseURL')}
          {row('group', channel.Group, 'group')}
          {row('weight', channel.Weight, 'weight')}
          {row('priority', channel.Priority, 'priority')}
          {row('remark', channel.Remark, 'remark')}
        </div>
      </div>

      {/* Column 2: models */}
      <div>
        <div className='lbl' style={{ marginBottom: 8 }}>
          models ({modelsList.length})
        </div>
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            gap: 4,
            maxHeight: 180,
            overflowY: 'auto',
          }}
        >
          {modelsList.length === 0 && (
            <span className='muted' style={{ fontSize: 11 }}>
              no models configured
            </span>
          )}
          {modelsList.map((m, i) => (
            <span key={i} className='pill mono' style={{ fontSize: 10 }}>
              {m}
            </span>
          ))}
        </div>
        <button
          type='button'
          className='btn sm'
          style={{ marginTop: 8 }}
          disabled={saving}
          onClick={() => setEditField('models')}
        >
          edit models
        </button>
        {editField === 'models' && (
          <div style={{ marginTop: 6 }}>
            <InlineEdit
              value={modelsList.join(',')}
              onSave={(v) => saveField('models', v)}
              onCancel={() => setEditField(null)}
            />
          </div>
        )}
      </div>

      {/* Column 3: model mapping + actions */}
      <div>
        <div className='lbl' style={{ marginBottom: 8 }}>model mapping</div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
          {mappings.length === 0 && (
            <span className='muted' style={{ fontSize: 11 }}>none</span>
          )}
          {mappings.map(([from, to], j) => (
            <div
              key={j}
              style={{
                display: 'grid',
                gridTemplateColumns: '1fr 16px 1fr',
                gap: 6,
                alignItems: 'center',
                fontSize: 10,
              }}
            >
              <span className='pill mono'>{from}</span>
              <span className='faint' style={{ textAlign: 'center' }}>→</span>
              <span className='pill mono'>{to}</span>
            </div>
          ))}
          {editField === 'modelMapping' ? (
            <div style={{ marginTop: 4 }}>
              <InlineEdit
                value={channel.ModelMapping ?? ''}
                onSave={(v) => saveField('modelMapping', v)}
                onCancel={() => setEditField(null)}
              />
            </div>
          ) : (
            <button
              type='button'
              className='btn sm'
              style={{ alignSelf: 'flex-start', marginTop: 4 }}
              disabled={saving}
              onClick={() => setEditField('modelMapping')}
            >
              edit mapping
            </button>
          )}
        </div>

        <div style={{ display: 'flex', gap: 6, marginTop: 16, flexWrap: 'wrap' }}>
          <button
            type='button'
            className='btn sm'
            disabled={saving}
            onClick={toggleStatus}
          >
            {channel.Status === 1 ? 'disable' : 'enable'}
          </button>
          <button
            type='button'
            className='btn sm'
            style={{ color: 'var(--hf-err)', borderColor: 'var(--hf-err)' }}
            disabled={saving}
            onClick={handleDelete}
          >
            delete
          </button>
        </div>
      </div>
    </div>
  );
};

// ─── Main page ────────────────────────────────────────────────────────────────

const HFChannel = () => {
  const tenantSlug = useTenantSlug();

  const [channels, setChannels] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [sel, setSel] = useState(new Set());
  const [open, setOpen] = useState(-1);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState(null); // channel object being edited

  const fetchChannels = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get(`/api/v2/${tenantSlug}/channels?page=1&page_size=100`);
      if (res?.data?.success) {
        const d = res.data.data;
        setChannels(d.channels ?? []);
        setTotal(d.total ?? (d.channels?.length ?? 0));
        setSel(new Set());
        setOpen(-1);
      }
    } catch (_) {
    } finally {
      setLoading(false);
    }
  }, [tenantSlug]);

  useEffect(() => {
    if (tenantSlug) fetchChannels();
  }, [fetchChannels, tenantSlug]);

  const toggle = (i) => {
    setSel((prev) => {
      const n = new Set(prev);
      if (n.has(i)) n.delete(i);
      else n.add(i);
      return n;
    });
  };

  const handleModalDone = async () => {
    setCreating(false);
    setEditing(null);
    await fetchChannels();
  };

  // Derived summary counts
  const okCount = channels.filter((c) => channelStatus(c) === 'ok').length;
  const disabledCount = channels.filter((c) => channelStatus(c) === 'disabled').length;
  const errorCount = channels.filter((c) => channelStatus(c) === 'error').length;

  // Batch enable/disable
  const batchSetStatus = async (status) => {
    const targets = [...sel].map((i) => channels[i]).filter(Boolean);
    if (targets.length === 0) return;
    try {
      await Promise.all(
        targets.map((ch) =>
          API.put(`/api/v2/${tenantSlug}/channels/${ch.Id}`, { status })
        )
      );
      showSuccess(`${targets.length} channel(s) updated`);
      await fetchChannels();
    } catch (_) {}
  };

  return (
    <HFShell
      active='channels'
      crumbs={['platform · admin', 'channels']}
      actions={
        <>
          <button
            type='button'
            className='btn primary'
            onClick={() => setCreating(true)}
          >
            + new channel
          </button>
        </>
      }
    >
      {/* Page header */}
      <div className='hf-page-head'>
        <div>
          <div className='lbl' style={{ marginBottom: 6 }}>channels</div>
          <h1>
            {loading ? '…' : `${total} upstream channel${total !== 1 ? 's' : ''}`}
          </h1>
          <div className='sub'>channel management · live data</div>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 28 }}>
          {[
            ['healthy', loading ? '…' : String(okCount), 'var(--hf-ok)'],
            ['disabled', loading ? '…' : String(disabledCount), 'var(--hf-warn)'],
            ['error', loading ? '…' : String(errorCount), 'var(--hf-err)'],
          ].map(([l, v, c], i) => (
            <div key={i}>
              <div className='lbl'>{l}</div>
              <div className='display' style={{ fontSize: 26, color: c, marginTop: 2 }}>
                {v}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Batch action bar */}
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
            <button
              type='button'
              className='btn'
              style={{ background: 'rgba(255,255,255,0.15)', borderColor: 'rgba(255,255,255,0.3)', color: '#fff' }}
              onClick={() => batchSetStatus(1)}
            >
              enable
            </button>
            <button
              type='button'
              className='btn'
              style={{ background: 'rgba(255,255,255,0.15)', borderColor: 'rgba(255,255,255,0.3)', color: '#fff' }}
              onClick={() => batchSetStatus(2)}
            >
              disable
            </button>
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
          <span>tip · select rows for batch operations · or click any row to expand</span>
        )}
      </div>

      {/* Table */}
      {loading ? (
        <div className='muted' style={{ padding: '24px 28px', fontSize: 12 }}>
          Loading…
        </div>
      ) : channels.length === 0 ? (
        <div className='muted' style={{ padding: '24px 28px', fontSize: 12 }}>
          No channels yet. Add one to get started.
        </div>
      ) : (
        <table className='t'>
          <thead>
            <tr>
              <th style={{ width: 32 }}></th>
              <th>channel</th>
              <th>type</th>
              <th>models</th>
              <th>qps · 1h trend</th>
              <th>p50 / p95</th>
              <th>success</th>
              <th>cost · 1h</th>
              <th>status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {channels.map((ch, i) => {
              const st = channelStatus(ch);
              const isOpen = open === i;

              const modelsList = (() => {
                if (!ch.Models) return [];
                if (Array.isArray(ch.Models)) return ch.Models;
                return ch.Models.split(',').map((m) => m.trim()).filter(Boolean);
              })();

              return (
                <Fragment key={ch.Id ?? i}>
                  <tr
                    style={{
                      background: sel.has(i) ? 'rgba(255,93,31,0.08)' : undefined,
                      borderLeft: sel.has(i)
                        ? '2px solid var(--hf-accent)'
                        : '2px solid transparent',
                      cursor: 'pointer',
                    }}
                  >
                    {/* Checkbox */}
                    <td onClick={() => toggle(i)}>
                      <input type='checkbox' checked={sel.has(i)} readOnly />
                    </td>

                    {/* Name + group */}
                    <td onClick={() => setOpen(isOpen ? -1 : i)}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                        <span className={statusDot(st)} />
                        <div>
                          <div className='strong'>{ch.Name}</div>
                          <div className='faint mono' style={{ fontSize: 9 }}>
                            {ch.Group || 'default'}
                            {ch.Tag ? ` · ${ch.Tag}` : ''}
                          </div>
                        </div>
                      </div>
                    </td>

                    {/* Type */}
                    <td className='mono muted' style={{ fontSize: 11 }} onClick={() => setOpen(isOpen ? -1 : i)}>
                      {ch.Type ?? '—'}
                    </td>

                    {/* Model count */}
                    <td className='mono' onClick={() => setOpen(isOpen ? -1 : i)}>
                      {modelsList.length > 0 ? modelsList.length : '—'}
                    </td>

                    {/* QPS — no backend aggregation yet */}
                    <td className='muted' style={{ fontSize: 11 }} onClick={() => setOpen(isOpen ? -1 : i)}>
                      —
                    </td>

                    {/* Latency — no backend aggregation yet */}
                    <td className='muted' style={{ fontSize: 11 }} onClick={() => setOpen(isOpen ? -1 : i)}>
                      —
                    </td>

                    {/* Success rate */}
                    <td onClick={() => setOpen(isOpen ? -1 : i)}>
                      {ch.SuccessRate != null && ch.SuccessRate > 0 ? (
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <div style={{ width: 60, height: 4, background: 'var(--hf-sunken)' }}>
                            <div
                              style={{
                                width: `${Math.min(ch.SuccessRate, 100)}%`,
                                height: '100%',
                                background:
                                  ch.SuccessRate >= 99
                                    ? 'var(--hf-ok)'
                                    : ch.SuccessRate >= 95
                                      ? 'var(--hf-warn)'
                                      : 'var(--hf-err)',
                              }}
                            />
                          </div>
                          <span
                            className='mono'
                            style={{
                              fontSize: 11,
                              color:
                                ch.SuccessRate >= 99
                                  ? 'var(--hf-ok)'
                                  : ch.SuccessRate >= 95
                                    ? 'var(--hf-warn)'
                                    : 'var(--hf-err)',
                            }}
                          >
                            {Number(ch.SuccessRate).toFixed(1)}%
                          </span>
                        </div>
                      ) : (
                        <span className='muted' style={{ fontSize: 11 }}>—</span>
                      )}
                    </td>

                    {/* Cost — no backend aggregation yet */}
                    <td className='muted' style={{ fontSize: 11 }} onClick={() => setOpen(isOpen ? -1 : i)}>
                      —
                    </td>

                    {/* Status badge */}
                    <td onClick={() => setOpen(isOpen ? -1 : i)}>
                      <span className={statusTag(st)}>{st}</span>
                    </td>

                    {/* Expand toggle */}
                    <td>
                      <button
                        type='button'
                        className='btn ghost'
                        onClick={() => setOpen(isOpen ? -1 : i)}
                      >
                        {isOpen ? '▾' : '▸'}
                      </button>
                    </td>
                  </tr>

                  {/* Expanded detail row */}
                  {isOpen && (
                    <tr>
                      <td
                        colSpan={10}
                        style={{
                          padding: 0,
                          background: 'var(--hf-paper)',
                          borderLeft: '2px solid var(--hf-accent)',
                        }}
                      >
                        <ExpandedRow
                          channel={ch}
                          tenantSlug={tenantSlug}
                          onRefresh={fetchChannels}
                        />
                        <div
                          style={{
                            padding: '0 20px 14px',
                            display: 'flex',
                            gap: 8,
                          }}
                        >
                          <button
                            type='button'
                            className='btn sm'
                            onClick={() => setEditing(ch)}
                          >
                            edit all fields
                          </button>
                        </div>
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      )}

      {/* Create modal */}
      {creating && (
        <ChannelModal
          tenantSlug={tenantSlug}
          existing={null}
          onDone={handleModalDone}
          onClose={() => setCreating(false)}
        />
      )}

      {/* Edit modal */}
      {editing && (
        <ChannelModal
          tenantSlug={tenantSlug}
          existing={editing}
          onDone={handleModalDone}
          onClose={() => setEditing(null)}
        />
      )}
    </HFShell>
  );
};

export default HFChannel;
