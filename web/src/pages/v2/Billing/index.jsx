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
import WIPBanner from '../../../components/hifi/WIPBanner';
import { API, showError } from '../../../helpers';

// HiFi 11 — Billing. Wired to real APIs (2026-05-19):
//   GET /api/v2/:slug/billing/invoices  → monthly spend buckets
//   GET /api/v2/user/billing/summary    → balance / MTD from platform

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

const fmtCNY = (v) =>
  typeof v === 'number'
    ? '¥' +
      v.toLocaleString(undefined, {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      })
    : '—';

const fmtQuota = (v) =>
  typeof v === 'number' ? (v / QUOTA_PER_USD).toFixed(4) + ' USD eq.' : '—';

const HFBilling = () => {
  const tenantSlug = useTenantSlug();

  const [invoices, setInvoices] = useState([]);
  const [summary, setSummary] = useState(null);
  const [loading, setLoading] = useState(true);
  const [recharging, setRecharging] = useState(false);

  const fetchAll = useCallback(async () => {
    if (!tenantSlug) return;
    setLoading(true);
    try {
      const [invRes, sumRes] = await Promise.all([
        API.get(`/api/v2/${tenantSlug}/billing/invoices`),
        API.get('/api/v2/user/billing/summary'),
      ]);

      if (invRes?.data?.success) {
        setInvoices(invRes.data.data?.items ?? []);
      } else {
        showError(invRes?.data?.message || '加载账单失败');
      }

      if (sumRes?.data?.success) {
        setSummary(sumRes.data.data);
      }
      // summary failure is non-fatal: page still works without it
    } catch (_) {
      // network errors surfaced by API interceptor
    } finally {
      setLoading(false);
    }
  }, [tenantSlug]);

  useEffect(() => {
    fetchAll();
  }, [fetchAll]);

  const handleRecharge = async () => {
    setRecharging(true);
    try {
      const res = await API.post('/api/v2/user/billing/checkout', {
        amount_cny: 200,
        payment_method: 'alipay',
        return_url: window.location.href,
      });
      if (res?.data?.success && res.data.data?.checkout_url) {
        window.location.href = res.data.data.checkout_url;
      } else {
        showError(res?.data?.message || '创建充值订单失败');
      }
    } catch (_) {
      // interceptor handles toast
    } finally {
      setRecharging(false);
    }
  };

  // Trend: last 6 invoices in chronological order for the bar chart.
  const trend = invoices.slice(0, 6).slice().reverse();
  const trendMax = trend.reduce((m, b) => Math.max(m, b.amount_cny ?? 0), 1);

  const balanceDisplay =
    summary?.wallet_balance_cny != null
      ? fmtCNY(summary.wallet_balance_cny)
      : '—';
  const mtdDisplay =
    summary?.mtd_spend_cny != null
      ? fmtCNY(summary.mtd_spend_cny)
      : invoices[0]
        ? fmtCNY(invoices[0].amount_cny)
        : '—';

  return (
    <HFShell
      active='billing'
      crumbs={['my account', 'billing']}
      actions={
        <>
          <button
            type='button'
            className='btn primary'
            onClick={handleRecharge}
            disabled={recharging}
            data-testid='billing-recharge'
          >
            {recharging ? '处理中…' : '+ 充值'}
          </button>
        </>
      }
    >
      <div className='hf-page-head'>
        <div>
          <div className='lbl' style={{ marginBottom: 6 }}>
            billing
          </div>
          <h1>
            {loading ? '…' : mtdDisplay}{' '}
            <span className='muted' style={{ fontWeight: 400 }}>
              · this month
            </span>
          </h1>
          <div className='sub'>prepaid balance · api consumption</div>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 28 }}>
          {[
            ['balance', balanceDisplay, 'var(--hf-ink)'],
            ['mtd spend', mtdDisplay, 'var(--hf-accent)'],
          ].map(([l, v, col], i) => (
            <div key={i}>
              <div className='lbl'>{l}</div>
              <div
                className='display'
                style={{ fontSize: 26, color: col, marginTop: 2 }}
              >
                {loading ? '…' : v}
              </div>
            </div>
          ))}
        </div>
      </div>

      <div
        style={{
          padding: 24,
          display: 'grid',
          gridTemplateColumns: '1fr 360px',
          gap: 18,
        }}
      >
        {/* Invoice table */}
        <div className='panel'>
          <div
            style={{
              padding: '14px 18px',
              borderBottom: '1px solid var(--hf-rule)',
            }}
          >
            <div className='lbl'>invoices · monthly</div>
          </div>

          {loading ? (
            <div
              style={{
                padding: 24,
                textAlign: 'center',
                color: 'var(--hf-ink-2)',
              }}
            >
              loading…
            </div>
          ) : invoices.length === 0 ? (
            <div
              style={{
                padding: 24,
                textAlign: 'center',
                color: 'var(--hf-ink-2)',
              }}
            >
              no invoice data
            </div>
          ) : (
            <table className='t'>
              <thead>
                <tr>
                  <th>period</th>
                  <th>amount (CNY)</th>
                  <th>quota used</th>
                  <th>requests</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {invoices.map((inv, i) => (
                  <tr key={i}>
                    <td className='mono strong'>{inv.month}</td>
                    <td>
                      <span className='display' style={{ fontSize: 16 }}>
                        {fmtCNY(inv.amount_cny)}
                      </span>
                    </td>
                    <td className='mono muted'>{fmtQuota(inv.quota)}</td>
                    <td className='mono muted'>{inv.request_count ?? '—'}</td>
                    <td>
                      {/* Download PDF — deferred to v2 */}
                      <span
                        data-testid={`billing-download-${i}`}
                        style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: 4,
                        }}
                      >
                        <button type='button' className='btn ghost sm'>
                          PDF
                        </button>
                        <WIPBanner
                          reason='PDF generation deferred to v2'
                          todo=''
                        />
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Side panel */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {/* Payment method — edit deferred */}
          <div className='panel' style={{ padding: 18 }}>
            <div className='lbl'>payment method</div>
            <div
              className='panel-paper'
              style={{
                marginTop: 10,
                padding: 14,
                display: 'flex',
                alignItems: 'center',
                gap: 10,
              }}
            >
              <div
                style={{
                  width: 36,
                  height: 24,
                  background: 'var(--hf-ink)',
                  color: 'var(--hf-bg)',
                  fontFamily: 'var(--hf-mono)',
                  fontSize: 9,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  letterSpacing: '0.1em',
                }}
              >
                ALI
              </div>
              <div>
                <div className='mono strong'>支付宝</div>
                <div className='faint mono' style={{ fontSize: 10 }}>
                  default
                </div>
              </div>
            </div>
            <div style={{ marginTop: 8 }}>
              <button
                type='button'
                className='btn sm'
                data-testid='billing-edit-payment'
                disabled
              >
                edit
              </button>
              <WIPBanner
                reason='payment method editing deferred to v2'
                todo=''
              />
            </div>
          </div>

          {/* Spend trend */}
          <div className='panel' style={{ padding: 18 }}>
            <div className='lbl'>trend · last 6 months</div>
            {loading ? (
              <div
                style={{
                  height: 100,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: 'var(--hf-ink-2)',
                }}
              >
                loading…
              </div>
            ) : (
              <div
                style={{
                  display: 'flex',
                  alignItems: 'flex-end',
                  gap: 8,
                  height: 100,
                  marginTop: 14,
                }}
              >
                {trend.map((b, i) => (
                  <div key={i} style={{ flex: 1, textAlign: 'center' }}>
                    <div
                      style={{
                        height: ((b.amount_cny ?? 0) / trendMax) * 80 + 'px',
                        background:
                          i === trend.length - 1
                            ? 'var(--hf-accent)'
                            : 'var(--hf-ink-2)',
                        opacity: i === trend.length - 1 ? 1 : 0.6,
                      }}
                    />
                    <div
                      className='faint mono'
                      style={{ fontSize: 9, marginTop: 4 }}
                    >
                      {b.month ? b.month.slice(5) : ''}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Summary stats from platform */}
          {summary && (
            <div className='panel' style={{ padding: 18 }}>
              <div className='lbl'>platform summary</div>
              <div
                style={{
                  marginTop: 10,
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 6,
                }}
              >
                {summary.subscription_plan && (
                  <div
                    style={{ display: 'flex', justifyContent: 'space-between' }}
                  >
                    <span className='muted'>plan</span>
                    <span className='mono strong'>
                      {summary.subscription_plan}
                    </span>
                  </div>
                )}
                {summary.wallet_balance_cny != null && (
                  <div
                    style={{ display: 'flex', justifyContent: 'space-between' }}
                  >
                    <span className='muted'>wallet</span>
                    <span className='mono strong'>
                      {fmtCNY(summary.wallet_balance_cny)}
                    </span>
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </HFShell>
  );
};

export default HFBilling;
