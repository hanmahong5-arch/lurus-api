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
import React, { useEffect, useMemo, useState } from 'react';
import { Spin } from '@douyinfe/semi-ui';
import HFShell from '../../../components/hifi/HFShell';
import WIPBanner from '../../../components/hifi/WIPBanner';
import { API, showError } from '../../../helpers';

/* v2 Pricing — wired to GET /api/v2/:tenant_slug/pricing (2026-05-19).
   Write path (markup multiplier + save) deferred to Epic 12. */

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

const PricingPage = () => {
  const tenantSlug = useTenantSlug();
  const [loading, setLoading] = useState(false);
  const [pricing, setPricing] = useState([]);
  const [vendors, setVendors] = useState([]);
  const [groupRatio, setGroupRatio] = useState({});
  const [vendorFilter, setVendorFilter] = useState('');

  useEffect(() => {
    if (!tenantSlug) return;
    setLoading(true);
    API.get(`/api/v2/${tenantSlug}/pricing`)
      .then((res) => {
        const d = res?.data?.data ?? {};
        setPricing(Array.isArray(d.pricing) ? d.pricing : []);
        setVendors(Array.isArray(d.vendors) ? d.vendors : []);
        setGroupRatio(
          d.group_ratio && typeof d.group_ratio === 'object'
            ? d.group_ratio
            : {},
        );
      })
      .catch((err) => {
        const msg = err?.response?.data?.message ?? '加载定价数据失败';
        showError(msg);
      })
      .finally(() => setLoading(false));
  }, [tenantSlug]);

  const filteredPricing = useMemo(() => {
    if (!vendorFilter) return pricing;
    return pricing.filter((p) => p.vendor === vendorFilter);
  }, [pricing, vendorFilter]);

  return (
    <HFShell
      active='pricing'
      crumbs={['平台', '定价管理']}
      actions={
        <>
          <button
            type='button'
            className='btn primary'
            disabled
            data-testid='pricing-save'
            title='write API pending Epic 12'
          >
            保存
          </button>
          <WIPBanner
            reason='write API pending Epic 12'
            todo='Epic 12 SKU model — POST /api/v2/:tenant_slug/pricing'
          />
        </>
      }
    >
      <Spin spinning={loading}>
        <div className='hf-page-head'>
          <div>
            <div className='lbl' style={{ marginBottom: 6 }}>
              定价管理
            </div>
            <h1>模型定价</h1>
            <div className='sub'>供应商成本 · 分组倍率 · 计费类型</div>
          </div>
        </div>

        {/* Vendor filter toolbar */}
        <div
          style={{
            padding: '0 24px 12px',
            display: 'flex',
            gap: 8,
            flexWrap: 'wrap',
          }}
        >
          <button
            type='button'
            className={`btn${!vendorFilter ? ' primary' : ''}`}
            onClick={() => setVendorFilter('')}
          >
            全部
          </button>
          {vendors.map((v) => (
            <button
              key={v}
              type='button'
              className={`btn${vendorFilter === v ? ' primary' : ''}`}
              onClick={() => setVendorFilter(v)}
            >
              {v}
            </button>
          ))}
        </div>

        <div style={{ padding: '0 24px 24px' }}>
          <div className='panel'>
            <div
              style={{
                padding: '14px 18px',
                borderBottom: '1px solid var(--hf-rule)',
                display: 'flex',
                alignItems: 'baseline',
              }}
            >
              <div className='lbl'>模型列表</div>
              <span
                className='muted mono'
                style={{ fontSize: 10, marginLeft: 'auto' }}
              >
                {filteredPricing.length} 个模型
              </span>
            </div>
            <table className='t' data-testid='pricing-table'>
              <thead>
                <tr>
                  <th>模型名称</th>
                  <th>供应商</th>
                  <th>计费类型</th>
                  <th>模型倍率</th>
                  <th>模型单价</th>
                  <th>
                    Markup multiplier
                    <span
                      className='muted'
                      style={{ fontSize: 10, marginLeft: 6, fontWeight: 400 }}
                    >
                      (write API pending Epic 12)
                    </span>
                  </th>
                  <th>启用分组</th>
                </tr>
              </thead>
              <tbody>
                {filteredPricing.map((row, i) => (
                  <tr key={row.model_name ?? i}>
                    <td className='strong mono' style={{ fontSize: 12 }}>
                      {row.model_name}
                    </td>
                    <td>{row.vendor ?? '—'}</td>
                    <td>
                      <span className='tag'>
                        {row.quota_type === 1 ? '单价计费' : '倍率计费'}
                      </span>
                    </td>
                    <td className='mono muted'>
                      {row.quota_type === 0
                        ? `×${(row.model_ratio ?? 0).toFixed(4)}`
                        : '—'}
                    </td>
                    <td className='mono muted'>
                      {row.quota_type === 1
                        ? `¥${(row.model_price ?? 0).toFixed(6)}`
                        : '—'}
                    </td>
                    <td>
                      <input
                        type='text'
                        className='field'
                        readOnly
                        disabled
                        placeholder='×1.00'
                        style={{ width: 80, height: 24, fontSize: 11 }}
                        title='write API pending Epic 12'
                      />
                    </td>
                    <td>
                      <span className='muted' style={{ fontSize: 11 }}>
                        {Array.isArray(row.enable_groups)
                          ? row.enable_groups.join(', ') || '—'
                          : '—'}
                      </span>
                    </td>
                  </tr>
                ))}
                {filteredPricing.length === 0 && !loading && (
                  <tr>
                    <td
                      colSpan={7}
                      className='muted'
                      style={{ textAlign: 'center', padding: 24 }}
                    >
                      暂无数据
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          {/* Group ratio — readonly display */}
          {Object.keys(groupRatio).length > 0 && (
            <div className='panel' style={{ marginTop: 18, padding: 18 }}>
              <div className='lbl' style={{ marginBottom: 10 }}>
                分组倍率（只读）
              </div>
              <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
                {Object.entries(groupRatio).map(([group, ratio]) => (
                  <div
                    key={group}
                    style={{
                      display: 'flex',
                      flexDirection: 'column',
                      alignItems: 'center',
                      padding: '8px 14px',
                      border: '1px solid var(--hf-rule)',
                      borderRadius: 4,
                      minWidth: 80,
                    }}
                  >
                    <span className='tag' style={{ marginBottom: 4 }}>
                      {group}
                    </span>
                    <span className='display mono' style={{ fontSize: 16 }}>
                      ×{Number(ratio).toFixed(2)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </Spin>
    </HFShell>
  );
};

export default PricingPage;
