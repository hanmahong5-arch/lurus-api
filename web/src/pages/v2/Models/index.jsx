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
import React, { useEffect, useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';
import WIPBanner from '../../../components/hifi/WIPBanner';
import { API, showError } from '../../../helpers';

/* HiFi 7 — Models catalog. Wired to GET /api/v2/:tenant_slug/models (2026-05-19). */

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
  const [vendor, setVendor] = useState('');
  const [models, setModels] = useState([]);
  const [total, setTotal] = useState(0);
  const [vendorCounts, setVendorCounts] = useState({});
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const fetchModels = async () => {
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
    fetchModels();
    return () => {
      cancelled = true;
    };
  }, [tenantSlug, vendor]);

  // Build vendor filter pills from vendor_counts; add "all" pseudo-entry.
  const vendorNames = Object.keys(vendorCounts).filter(Boolean).sort();

  return (
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
            disabled
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
          style={{ padding: 48, textAlign: 'center', color: 'var(--hf-ink-2)' }}
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
              style={{ padding: 18, position: 'relative', overflow: 'hidden' }}
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
                <button type='button' className='btn sm'>
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
  );
};

// Minimal t() shim — keys are used as display text until Opus wires i18n.
// Keys map to the i18n table in the report. Replace with useTranslation() when
// en.json / zh.json entries are merged.
function t(key) {
  return key;
}

export default HFModels;
