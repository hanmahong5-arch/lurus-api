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
import React, { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import HFShell from '../../../components/hifi/HFShell';
import WIPBanner from '../../../components/hifi/WIPBanner';
import { API, showError, showSuccess } from '../../../helpers';
import { useFormDraft } from '../../../hooks/common/useFormDraft';

/* HiFi 7 — Models catalog. Wired to GET /api/v2/:tenant_slug/models (2026-05-19).
   Wave 3 Phase 1 (2026-05-20): + 添加模型 modal + 试用 ↗ navigate wired. */

// Known vendor names for the add-model select. The list is used for UX
// convenience only — the backend accepts any vendor string.
const KNOWN_VENDORS = [
  'OpenAI',
  'Anthropic',
  'Google',
  'Baidu',
  'Tencent',
  'Zhipu',
  'Minimax',
  'Cohere',
  'Perplexity',
  'Siliconflow',
  'AWS',
  'Other',
];

const QUOTA_TYPE_OPTIONS = [
  { value: 0, label: t('按量 (token)') },
  { value: 1, label: t('按次') },
];

const DRAFT_KEY = 'models-add-form';
const DRAFT_INITIAL = {
  model_name: '',
  vendor: '',
  model_ratio: 1,
  quota_type: 0,
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

const STATUS_LABEL = { 1: 'active', 0: 'disabled' };

const HFModels = () => {
  const tenantSlug = useTenantSlug();
  const navigate = useNavigate();

  const [vendor, setVendor] = useState('');
  const [models, setModels] = useState([]);
  const [total, setTotal] = useState(0);
  const [vendorCounts, setVendorCounts] = useState({});
  const [loading, setLoading] = useState(false);

  // Add-model modal state.
  const [addOpen, setAddOpen] = useState(false);
  const [adding, setAdding] = useState(false);
  const [form, setForm, clearDraft] = useFormDraft(DRAFT_KEY, DRAFT_INITIAL);
  const dialogRef = useRef(null);

  // Open / close modal — sync <dialog> element with state.
  // Guard for jsdom / older browsers that don't implement showModal().
  useEffect(() => {
    const el = dialogRef.current;
    if (!el) return;
    if (addOpen) {
      if (!el.open && typeof el.showModal === 'function') el.showModal();
    } else {
      if (el.open && typeof el.close === 'function') el.close();
    }
  }, [addOpen]);

  const fetchModels = async (slug, vendorFilter) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ limit: '100', offset: '0' });
      if (vendorFilter) params.set('vendor', vendorFilter);
      const res = await API.get(`/api/v2/${slug}/models?${params.toString()}`);
      const d = res?.data?.data ?? {};
      setModels(d.items ?? []);
      setTotal(d.total ?? 0);
      if (d.vendor_counts) setVendorCounts(d.vendor_counts);
    } catch (err) {
      const msg =
        err?.response?.data?.message ?? err?.message ?? t('加载模型失败');
      showError(msg);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    let cancelled = false;
    const run = async () => {
      setLoading(true);
      try {
        const params = new URLSearchParams({ limit: '100', offset: '0' });
        if (vendor) params.set('vendor', vendor);
        const res = await API.get(
          `/api/v2/${tenantSlug}/models?${params.toString()}`,
        );
        if (cancelled) return;
        const d = res?.data?.data ?? {};
        setModels(d.items ?? []);
        setTotal(d.total ?? 0);
        if (d.vendor_counts) setVendorCounts(d.vendor_counts);
      } catch (err) {
        if (cancelled) return;
        const msg =
          err?.response?.data?.message ?? err?.message ?? t('加载模型失败');
        showError(msg);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    run();
    return () => {
      cancelled = true;
    };
  }, [tenantSlug, vendor]);

  // Build vendor filter pills from vendor_counts; add "all" pseudo-entry.
  const vendorNames = Object.keys(vendorCounts).filter(Boolean).sort();

  // ── Add-model submit ──────────────────────────────────────────────────────
  const handleAddSubmit = async (e) => {
    e.preventDefault();
    if (!form.model_name.trim()) {
      showError(t('模型名称不能为空'));
      return;
    }
    setAdding(true);
    try {
      await API.post(`/api/v2/${tenantSlug}/models`, {
        model_name: form.model_name.trim(),
        vendor: form.vendor,
        model_ratio: Number(form.model_ratio) || 1,
        quota_type: Number(form.quota_type),
      });
      showSuccess(t('模型已添加'));
      clearDraft();
      setAddOpen(false);
      // Refresh list.
      await fetchModels(tenantSlug, vendor);
    } catch (err) {
      const msg = err?.response?.data?.message ?? err?.message ?? t('添加失败');
      showError(msg);
    } finally {
      setAdding(false);
    }
  };

  return (
    <>
      <HFShell
        active='models'
        crumbs={[t('平台 · 管理'), t('模型管理')]}
        actions={
          <>
            {/* single-model editing deferred to v3 */}
            <WIPBanner
              reason={t('单模型编辑推迟到 v3')}
              todo='v3 story: per-model enable/disable'
            />
            <button
              type='button'
              className='btn primary'
              data-testid='models-add-btn'
              onClick={() => setAddOpen(true)}
            >
              {t('+ 添加模型')}
            </button>
          </>
        }
      >
        <div className='hf-page-head'>
          <div>
            <div className='lbl' style={{ marginBottom: 6 }}>
              {t('目录')}
            </div>
            <h1>
              {loading ? '…' : total}{' '}
              <span className='muted' style={{ fontWeight: 400 }}>
                {t('个模型')}
              </span>
            </h1>
          </div>
        </div>

        {/* Vendor filter pills */}
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
          <span className='lbl'>{t('供应商')}</span>
          <button
            key='all'
            type='button'
            data-testid='vendor-filter-all'
            onClick={() => setVendor('')}
            className={'pill ' + (!vendor ? 'solid' : '')}
            style={{
              cursor: 'pointer',
              border: '1px solid var(--hf-rule)',
              background: !vendor ? 'var(--hf-ink)' : 'var(--hf-elev)',
              color: !vendor ? 'var(--hf-bg)' : 'var(--hf-ink-2)',
            }}
          >
            {t('全部')} ({total})
          </button>
          {vendorNames.map((v) => (
            <button
              key={v}
              type='button'
              data-testid={`vendor-filter-${v}`}
              onClick={() => setVendor(v)}
              className={'pill ' + (vendor === v ? 'solid' : '')}
              style={{
                cursor: 'pointer',
                border: '1px solid var(--hf-rule)',
                background: vendor === v ? 'var(--hf-ink)' : 'var(--hf-elev)',
                color: vendor === v ? 'var(--hf-bg)' : 'var(--hf-ink-2)',
              }}
            >
              {v} ({vendorCounts[v] ?? 0})
            </button>
          ))}
        </div>

        {/* Model grid */}
        {loading ? (
          <div
            data-testid='models-loading'
            style={{
              padding: 48,
              textAlign: 'center',
              color: 'var(--hf-ink-2)',
            }}
          >
            {t('加载中…')}
          </div>
        ) : (
          <div
            style={{
              padding: 24,
              display: 'grid',
              gridTemplateColumns: 'repeat(3, 1fr)',
              gap: 14,
            }}
          >
            {models.map((m) => (
              <div
                key={m.id}
                className='panel'
                data-testid={`model-card-${m.model_name}`}
                style={{
                  padding: 18,
                  position: 'relative',
                  overflow: 'hidden',
                }}
              >
                <div className='lbl' style={{ color: 'var(--hf-ink-2)' }}>
                  {m.vendor || t('未知供应商')}
                </div>
                <div
                  className='display'
                  style={{
                    fontSize: 20,
                    marginTop: 6,
                    letterSpacing: '-0.025em',
                  }}
                >
                  {m.model_name}
                </div>
                <div
                  className='muted mono'
                  style={{ fontSize: 11, marginTop: 4 }}
                >
                  {t('状态')}: {STATUS_LABEL[m.status] ?? m.status}
                </div>

                <div style={{ display: 'flex', gap: 6, marginTop: 14 }}>
                  <button
                    type='button'
                    className='btn sm'
                    data-testid={`model-try-${m.model_name}`}
                    onClick={() =>
                      navigate(
                        `/console/v2/playground?prefill_model=${encodeURIComponent(m.model_name)}`,
                      )
                    }
                  >
                    {t('试用')} ↗
                  </button>
                  {/* single-model enable/disable deferred to v3 */}
                  <WIPBanner
                    reason={t('单模型启用/禁用推迟到 v3')}
                    todo='v3 story: per-model enable/disable'
                  />
                </div>
              </div>
            ))}
            {models.length === 0 && !loading && (
              <div
                data-testid='models-empty'
                style={{
                  gridColumn: '1/-1',
                  padding: 48,
                  textAlign: 'center',
                  color: 'var(--hf-ink-2)',
                }}
              >
                {t('暂无模型')}
              </div>
            )}
          </div>
        )}
      </HFShell>

      {/* ── Add Model dialog ─────────────────────────────────────────────── */}
      {/* Native <dialog> — progressive enhancement; no external Modal dep needed. */}
      <dialog
        ref={dialogRef}
        data-testid='models-add-dialog'
        style={{
          padding: 32,
          borderRadius: 6,
          border: '1px solid var(--hf-rule)',
          background: 'var(--hf-paper)',
          color: 'var(--hf-ink)',
          minWidth: 400,
          maxWidth: 520,
        }}
        onClose={() => setAddOpen(false)}
      >
        <h2 style={{ margin: '0 0 20px', fontSize: 18 }}>{t('添加模型')}</h2>
        <form method='dialog' onSubmit={handleAddSubmit}>
          <div style={{ display: 'grid', gap: 14 }}>
            <label>
              <span
                className='lbl'
                style={{ display: 'block', marginBottom: 4 }}
              >
                {t('模型名称')} *
              </span>
              <input
                data-testid='add-model-name'
                type='text'
                required
                value={form.model_name}
                onChange={(e) =>
                  setForm({ ...form, model_name: e.target.value })
                }
                placeholder='e.g. gpt-4o'
                style={{
                  width: '100%',
                  padding: '6px 10px',
                  border: '1px solid var(--hf-rule)',
                  background: 'var(--hf-sunken)',
                  color: 'var(--hf-ink)',
                  borderRadius: 2,
                  fontFamily: 'var(--hf-mono)',
                  boxSizing: 'border-box',
                }}
              />
            </label>

            <label>
              <span
                className='lbl'
                style={{ display: 'block', marginBottom: 4 }}
              >
                {t('供应商')}
              </span>
              <select
                data-testid='add-model-vendor'
                value={form.vendor}
                onChange={(e) => setForm({ ...form, vendor: e.target.value })}
                style={{
                  width: '100%',
                  padding: '6px 10px',
                  border: '1px solid var(--hf-rule)',
                  background: 'var(--hf-sunken)',
                  color: 'var(--hf-ink)',
                  borderRadius: 2,
                }}
              >
                <option value=''>{t('— 选择供应商 —')}</option>
                {KNOWN_VENDORS.map((v) => (
                  <option key={v} value={v}>
                    {v}
                  </option>
                ))}
              </select>
            </label>

            <label>
              <span
                className='lbl'
                style={{ display: 'block', marginBottom: 4 }}
              >
                {t('计费类型')}
              </span>
              <select
                data-testid='add-model-quota-type'
                value={form.quota_type}
                onChange={(e) =>
                  setForm({ ...form, quota_type: Number(e.target.value) })
                }
                style={{
                  width: '100%',
                  padding: '6px 10px',
                  border: '1px solid var(--hf-rule)',
                  background: 'var(--hf-sunken)',
                  color: 'var(--hf-ink)',
                  borderRadius: 2,
                }}
              >
                {QUOTA_TYPE_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </label>

            <label>
              <span
                className='lbl'
                style={{ display: 'block', marginBottom: 4 }}
              >
                {t('模型倍率')}
              </span>
              <input
                data-testid='add-model-ratio'
                type='number'
                min='0'
                step='0.001'
                value={form.model_ratio}
                onChange={(e) =>
                  setForm({ ...form, model_ratio: e.target.value })
                }
                style={{
                  width: '100%',
                  padding: '6px 10px',
                  border: '1px solid var(--hf-rule)',
                  background: 'var(--hf-sunken)',
                  color: 'var(--hf-ink)',
                  borderRadius: 2,
                  fontFamily: 'var(--hf-mono)',
                  boxSizing: 'border-box',
                }}
              />
            </label>
          </div>

          <div
            style={{
              display: 'flex',
              justifyContent: 'flex-end',
              gap: 10,
              marginTop: 24,
            }}
          >
            <button
              type='button'
              className='btn'
              data-testid='add-model-cancel'
              onClick={() => {
                setAddOpen(false);
                clearDraft();
              }}
            >
              {t('取消')}
            </button>
            <button
              type='submit'
              className='btn primary'
              data-testid='add-model-submit'
              disabled={adding}
              onClick={handleAddSubmit}
            >
              {adding ? t('添加中…') : t('添加')}
            </button>
          </div>
        </form>
      </dialog>
    </>
  );
};

// Minimal t() shim — keys are used as display text until i18n wires in.
// i18n keys introduced in Wave 3 Phase 1:
//   '+ 添加模型'  '添加模型'  '模型名称'  '供应商'  '计费类型'  '模型倍率'
//   '按量 (token)'  '按次'  '— 选择供应商 —'  '取消'  '添加'  '添加中…'
//   '模型已添加'  '添加失败'  '模型名称不能为空'  '单模型编辑推迟到 v3'
//   '试用'
function t(key) {
  return key;
}

export default HFModels;
