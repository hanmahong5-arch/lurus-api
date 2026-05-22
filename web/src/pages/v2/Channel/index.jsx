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
import React, {
  Fragment,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';
import { useTranslation } from 'react-i18next';
import HFShell from '../../../components/hifi/HFShell';
import ConfirmDialog from '../../../components/common/ConfirmDialog';
import { API, showError, showSuccess } from '../../../helpers';

// ─── SyncModelsModal ─────────────────────────────────────────────────────────

const SyncModelsModal = ({ tenantSlug, channel, onClose, onApply }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [diff, setDiff] = useState(null);
  const [selected, setSelected] = useState(new Set());
  const [applying, setApplying] = useState(false);

  useEffect(() => {
    setLoading(true);
    API.get(`/api/v2/${tenantSlug}/channels/${channel.id}/upstream-models`)
      .then((res) => {
        if (res?.data?.success) {
          const d = res.data.data;
          setDiff(d);
          // Pre-select new models by default.
          setSelected(new Set(d.new ?? []));
        } else {
          showError(t('获取上游模型失败'));
          onClose();
        }
      })
      .catch(() => {
        showError(t('获取上游模型失败'));
        onClose();
      })
      .finally(() => setLoading(false));
  }, [tenantSlug, channel.id]); // eslint-disable-line react-hooks/exhaustive-deps

  const toggleModel = (m) => {
    setSelected((prev) => {
      const n = new Set(prev);
      if (n.has(m)) n.delete(m);
      else n.add(m);
      return n;
    });
  };

  const handleApply = async () => {
    if (!diff) return;
    setApplying(true);
    try {
      // Merge: start from current list, add selected new, remove missing that are deselected.
      const currentSet = new Set(
        (channel.models || '')
          .split(',')
          .map((m) => m.trim())
          .filter(Boolean),
      );
      // Add selected new models.
      for (const m of selected) {
        currentSet.add(m);
      }
      const newModels = [...currentSet].join(',');
      const res = await API.put(
        `/api/v2/${tenantSlug}/channels/${channel.id}`,
        { models: newModels },
      );
      if (res?.data?.success) {
        showSuccess(t('同步上游模型'));
        onApply();
      }
    } catch (_) {
    } finally {
      setApplying(false);
    }
  };

  const pillStyle = (active) => ({
    display: 'inline-block',
    fontFamily: 'var(--hf-mono)',
    fontSize: 10,
    padding: '2px 6px',
    borderRadius: 3,
    border: `1px solid ${active ? 'var(--hf-ok)' : 'var(--hf-rule)'}`,
    background: active ? 'rgba(40,200,90,0.1)' : 'var(--hf-sunken)',
    color: active ? 'var(--hf-ok)' : 'var(--hf-ink)',
    cursor: 'pointer',
    userSelect: 'none',
  });

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
      <div
        style={{
          background: 'var(--hf-paper)',
          border: '1px solid var(--hf-rule)',
          borderRadius: 4,
          padding: 28,
          width: 560,
          maxHeight: '80vh',
          overflowY: 'auto',
          display: 'flex',
          flexDirection: 'column',
          gap: 14,
        }}
      >
        <div className='strong' style={{ fontSize: 15 }}>
          {t('同步上游模型')} · {channel.name}
        </div>

        {loading ? (
          <div className='muted' style={{ fontSize: 12 }}>
            {t('正在获取上游模型…')}
          </div>
        ) : diff ? (
          <>
            <div>
              <div className='lbl' style={{ marginBottom: 6 }}>
                {t('上游模型 (共 {{count}} 个)', {
                  count: diff.upstream?.length ?? 0,
                })}
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                {(diff.upstream ?? []).map((m) => (
                  <span key={m} style={pillStyle(false)}>
                    {m}
                  </span>
                ))}
              </div>
            </div>

            {diff.new?.length > 0 && (
              <div>
                <div
                  className='lbl'
                  style={{ marginBottom: 6, color: 'var(--hf-ok)' }}
                >
                  {t('新增 {{count}} 个模型', { count: diff.new.length })} —
                  点击切换
                </div>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                  {diff.new.map((m) => (
                    <span
                      key={m}
                      data-testid={`sync-new-${m}`}
                      style={pillStyle(selected.has(m))}
                      onClick={() => toggleModel(m)}
                    >
                      + {m}
                    </span>
                  ))}
                </div>
              </div>
            )}

            {diff.missing?.length > 0 && (
              <div>
                <div
                  className='lbl'
                  style={{ marginBottom: 6, color: 'var(--hf-warn)' }}
                >
                  {t('已移除 {{count}} 个模型', { count: diff.missing.length })}
                </div>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                  {diff.missing.map((m) => (
                    <span
                      key={m}
                      style={{
                        ...pillStyle(false),
                        color: 'var(--hf-warn)',
                        borderColor: 'var(--hf-warn)',
                        textDecoration: 'line-through',
                      }}
                    >
                      {m}
                    </span>
                  ))}
                </div>
              </div>
            )}

            {diff.new?.length === 0 && diff.missing?.length === 0 && (
              <div className='muted' style={{ fontSize: 12 }}>
                模型列表已是最新，无需同步。
              </div>
            )}
          </>
        ) : null}

        <div
          style={{
            display: 'flex',
            gap: 8,
            justifyContent: 'flex-end',
            marginTop: 4,
          }}
        >
          <button type='button' className='btn ghost' onClick={onClose}>
            取消
          </button>
          <button
            type='button'
            className='btn primary'
            data-testid='sync-apply-btn'
            disabled={applying || loading || selected.size === 0}
            onClick={handleApply}
          >
            {applying ? '应用中…' : t('应用所选模型')}
          </button>
        </div>
      </div>
    </div>
  );
};

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
  if (ch.status === 1) return 'ok';
  if (ch.status === 2) return 'disabled';
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
          name: existing.name ?? '',
          key: '',
          type: existing.type ?? 1,
          baseURL: existing.base_url ?? '',
          models: Array.isArray(existing.models)
            ? existing.models.join(',')
            : (existing.models ?? ''),
          group: existing.group ?? 'default',
          weight: existing.weight ?? 1,
          priority: existing.priority ?? 0,
          modelMapping: existing.model_mapping ?? '',
          tag: existing.tag ?? '',
          remark: existing.remark ?? '',
        }
      : { ...EMPTY_FORM },
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
        base_url: form.baseURL.trim(),
        models: form.models.trim(),
        group: form.group.trim() || 'default',
        weight: Number(form.weight) || 1,
        priority: Number(form.priority) || 0,
        model_mapping: form.modelMapping.trim(),
        tag: form.tag.trim(),
        remark: form.remark.trim(),
      };
      if (!existing && form.key.trim()) body.key = form.key.trim();

      let res;
      if (existing) {
        res = await API.put(
          `/api/v2/${tenantSlug}/channels/${existing.id}`,
          body,
        );
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
          {existing ? `Edit · ${existing.name}` : 'New channel'}
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

        {field('base url', 'baseURL', {
          placeholder: 'https://api.openai.com/v1',
        })}

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

        <div
          style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}
        >
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
          <span className='lbl'>
            model mapping (from=to, comma-separated or JSON)
          </span>
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

        <div
          style={{
            display: 'flex',
            gap: 8,
            justifyContent: 'flex-end',
            marginTop: 4,
          }}
        >
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
  const { t } = useTranslation();
  const [editField, setEditField] = useState(null);
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const mappings = parseMapping(channel.model_mapping);

  const modelsList = (() => {
    if (!channel.models) return [];
    if (Array.isArray(channel.models)) return channel.models;
    return channel.models
      .split(',')
      .map((m) => m.trim())
      .filter(Boolean);
  })();

  const saveField = async (field, value) => {
    setEditField(null);
    const body = {};
    if (field === 'baseURL') body.base_url = value;
    else if (field === 'group') body.group = value || 'default';
    else if (field === 'weight') body.weight = Number(value) || 1;
    else if (field === 'priority') body.priority = Number(value) || 0;
    else if (field === 'remark') body.remark = value;
    else if (field === 'models') body.models = value;
    else if (field === 'modelMapping') body.model_mapping = value;
    if (Object.keys(body).length === 0) return;
    setSaving(true);
    try {
      const res = await API.put(
        `/api/v2/${tenantSlug}/channels/${channel.id}`,
        body,
      );
      if (res?.data?.success) {
        showSuccess('Saved');
        onRefresh();
      }
    } catch (_) {
    } finally {
      setSaving(false);
    }
  };

  // Tier 1.3: open the typed-confirmation dialog instead of window.confirm.
  const handleDelete = () => setConfirmDelete(true);

  const performDelete = async () => {
    setSaving(true);
    try {
      const res = await API.delete(
        `/api/v2/${tenantSlug}/channels/${channel.id}`,
      );
      if (res?.data?.success) {
        showSuccess('Channel deleted');
        setConfirmDelete(false);
        onRefresh();
      }
    } catch (_) {
    } finally {
      setSaving(false);
    }
  };

  const toggleStatus = async () => {
    const newStatus = channel.status === 1 ? 2 : 1;
    setSaving(true);
    try {
      const res = await API.put(
        `/api/v2/${tenantSlug}/channels/${channel.id}`,
        { status: newStatus },
      );
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
    <div
      style={{
        padding: 20,
        display: 'grid',
        gridTemplateColumns: '1.4fr 1fr 1fr',
        gap: 20,
      }}
    >
      {/* Column 1: settings */}
      <div>
        <div className='lbl' style={{ marginBottom: 8 }}>
          channel settings
        </div>
        <div>
          {row('base url', channel.base_url, 'baseURL')}
          {row('group', channel.group, 'group')}
          {row('weight', channel.weight, 'weight')}
          {row('priority', channel.priority, 'priority')}
          {row('remark', channel.remark, 'remark')}
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
        <div className='lbl' style={{ marginBottom: 8 }}>
          model mapping
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
          {mappings.length === 0 && (
            <span className='muted' style={{ fontSize: 11 }}>
              none
            </span>
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
              <span className='faint' style={{ textAlign: 'center' }}>
                →
              </span>
              <span className='pill mono'>{to}</span>
            </div>
          ))}
          {editField === 'modelMapping' ? (
            <div style={{ marginTop: 4 }}>
              <InlineEdit
                value={channel.model_mapping ?? ''}
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

        <div
          style={{ display: 'flex', gap: 6, marginTop: 16, flexWrap: 'wrap' }}
        >
          <button
            type='button'
            className='btn sm'
            disabled={saving}
            onClick={toggleStatus}
          >
            {channel.status === 1 ? 'disable' : 'enable'}
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

      <ConfirmDialog
        visible={confirmDelete}
        title={t('删除渠道 "{{name}}" ?', { name: channel.name })}
        consequenceList={[t('停止通过此渠道路由请求'), t('此操作无法撤销')]}
        confirmText={channel.name}
        onConfirm={performDelete}
        onCancel={() => !saving && setConfirmDelete(false)}
      />
    </div>
  );
};

// ─── Main page ────────────────────────────────────────────────────────────────

const HFChannel = () => {
  const { t } = useTranslation();
  const tenantSlug = useTenantSlug();

  const [channels, setChannels] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [sel, setSel] = useState(new Set());
  const [open, setOpen] = useState(-1);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState(null); // channel object being edited
  const [syncTarget, setSyncTarget] = useState(null); // channel being synced
  // testState: { [channelId]: 'testing' | { latency_ms, success, error? } }
  const [testState, setTestState] = useState({});

  const fetchChannels = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get(
        `/api/v2/${tenantSlug}/channels?page=1&page_size=100`,
      );
      if (res?.data?.success) {
        const d = res.data.data;
        setChannels(d.channels ?? []);
        setTotal(d.total ?? d.channels?.length ?? 0);
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

  const handleTestChannel = async (e, ch) => {
    e.stopPropagation();
    const id = ch.id;
    setTestState((prev) => ({ ...prev, [id]: 'testing' }));
    try {
      const res = await API.post(
        `/api/v2/${tenantSlug}/channels/${id}/test`,
        {},
      );
      const data = res?.data ?? {};
      setTestState((prev) => ({ ...prev, [id]: data }));
      if (data.success) {
        showSuccess(t('渠道延迟 {{ms}}ms', { ms: data.latency_ms ?? 0 }));
      } else {
        showError(t('渠道测试失败') + (data.error ? `: ${data.error}` : ''));
      }
    } catch (err) {
      setTestState((prev) => ({
        ...prev,
        [id]: { success: false, error: String(err) },
      }));
      showError(t('渠道测试失败'));
    }
  };

  // Derived summary counts
  const okCount = channels.filter((c) => channelStatus(c) === 'ok').length;
  const disabledCount = channels.filter(
    (c) => channelStatus(c) === 'disabled',
  ).length;
  const errorCount = channels.filter(
    (c) => channelStatus(c) === 'error',
  ).length;

  // Batch enable/disable
  const batchSetStatus = async (status) => {
    const targets = [...sel].map((i) => channels[i]).filter(Boolean);
    if (targets.length === 0) return;
    try {
      await Promise.all(
        targets.map((ch) =>
          API.put(`/api/v2/${tenantSlug}/channels/${ch.id}`, { status }),
        ),
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
          <div className='lbl' style={{ marginBottom: 6 }}>
            channels
          </div>
          <h1>
            {loading
              ? '…'
              : `${total} upstream channel${total !== 1 ? 's' : ''}`}
          </h1>
          <div className='sub'>channel management · live data</div>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 28 }}>
          {[
            ['healthy', loading ? '…' : String(okCount), 'var(--hf-ok)'],
            [
              'disabled',
              loading ? '…' : String(disabledCount),
              'var(--hf-warn)',
            ],
            ['error', loading ? '…' : String(errorCount), 'var(--hf-err)'],
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
              style={{
                background: 'rgba(255,255,255,0.15)',
                borderColor: 'rgba(255,255,255,0.3)',
                color: '#fff',
              }}
              onClick={() => batchSetStatus(1)}
            >
              enable
            </button>
            <button
              type='button'
              className='btn'
              style={{
                background: 'rgba(255,255,255,0.15)',
                borderColor: 'rgba(255,255,255,0.3)',
                color: '#fff',
              }}
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
          <span>
            tip · select rows for batch operations · or click any row to expand
          </span>
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
                if (!ch.models) return [];
                if (Array.isArray(ch.models)) return ch.models;
                return ch.models
                  .split(',')
                  .map((m) => m.trim())
                  .filter(Boolean);
              })();

              return (
                <Fragment key={ch.id ?? i}>
                  <tr
                    style={{
                      background: sel.has(i)
                        ? 'rgba(255,93,31,0.08)'
                        : undefined,
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
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 10,
                        }}
                      >
                        <span className={statusDot(st)} />
                        <div>
                          <div className='strong'>{ch.name}</div>
                          <div className='faint mono' style={{ fontSize: 9 }}>
                            {ch.group || 'default'}
                            {ch.tag ? ` · ${ch.tag}` : ''}
                          </div>
                        </div>
                      </div>
                    </td>

                    {/* Type */}
                    <td
                      className='mono muted'
                      style={{ fontSize: 11 }}
                      onClick={() => setOpen(isOpen ? -1 : i)}
                    >
                      {ch.type ?? '—'}
                    </td>

                    {/* Model count */}
                    <td
                      className='mono'
                      onClick={() => setOpen(isOpen ? -1 : i)}
                    >
                      {modelsList.length > 0 ? modelsList.length : '—'}
                    </td>

                    {/* QPS — no backend aggregation yet */}
                    <td
                      className='muted'
                      style={{ fontSize: 11 }}
                      onClick={() => setOpen(isOpen ? -1 : i)}
                    >
                      —
                    </td>

                    {/* Latency — no backend aggregation yet */}
                    <td
                      className='muted'
                      style={{ fontSize: 11 }}
                      onClick={() => setOpen(isOpen ? -1 : i)}
                    >
                      —
                    </td>

                    {/* Success rate */}
                    <td onClick={() => setOpen(isOpen ? -1 : i)}>
                      {ch.success_rate != null && ch.success_rate > 0 ? (
                        <div
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 8,
                          }}
                        >
                          <div
                            style={{
                              width: 60,
                              height: 4,
                              background: 'var(--hf-sunken)',
                            }}
                          >
                            <div
                              style={{
                                width: `${Math.min(ch.success_rate, 100)}%`,
                                height: '100%',
                                background:
                                  ch.success_rate >= 99
                                    ? 'var(--hf-ok)'
                                    : ch.success_rate >= 95
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
                                ch.success_rate >= 99
                                  ? 'var(--hf-ok)'
                                  : ch.success_rate >= 95
                                    ? 'var(--hf-warn)'
                                    : 'var(--hf-err)',
                            }}
                          >
                            {Number(ch.success_rate).toFixed(1)}%
                          </span>
                        </div>
                      ) : (
                        <span className='muted' style={{ fontSize: 11 }}>
                          —
                        </span>
                      )}
                    </td>

                    {/* Cost — no backend aggregation yet */}
                    <td
                      className='muted'
                      style={{ fontSize: 11 }}
                      onClick={() => setOpen(isOpen ? -1 : i)}
                    >
                      —
                    </td>

                    {/* Status badge */}
                    <td onClick={() => setOpen(isOpen ? -1 : i)}>
                      <span className={statusTag(st)}>{st}</span>
                    </td>

                    {/* Row actions: test + expand */}
                    <td style={{ whiteSpace: 'nowrap' }}>
                      {(() => {
                        const ts = testState[ch.id];
                        const isTesting = ts === 'testing';
                        const testLabel = isTesting
                          ? t('测试中…')
                          : t('测试渠道');
                        const testColor =
                          ts && ts !== 'testing'
                            ? ts.success
                              ? 'var(--hf-ok)'
                              : 'var(--hf-err)'
                            : undefined;
                        return (
                          <button
                            type='button'
                            className='btn ghost sm'
                            data-testid={`test-btn-${ch.id}`}
                            disabled={isTesting}
                            style={
                              testColor
                                ? { color: testColor, borderColor: testColor }
                                : undefined
                            }
                            onClick={(e) => handleTestChannel(e, ch)}
                          >
                            {testLabel}
                          </button>
                        );
                      })()}
                      <button
                        type='button'
                        className='btn ghost'
                        style={{ marginLeft: 4 }}
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
                          <button
                            type='button'
                            className='btn sm'
                            data-testid={`sync-btn-${ch.id}`}
                            onClick={() => setSyncTarget(ch)}
                          >
                            {t('同步上游模型')}
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

      {/* Sync upstream models modal */}
      {syncTarget && (
        <SyncModelsModal
          tenantSlug={tenantSlug}
          channel={syncTarget}
          onClose={() => setSyncTarget(null)}
          onApply={async () => {
            setSyncTarget(null);
            await fetchChannels();
          }}
        />
      )}
    </HFShell>
  );
};

export default HFChannel;
