import React from 'react';
import HFShell from '../../../components/hifi/HFShell';
import WIPBanner from '../../../components/hifi/WIPBanner';

/* HiFi 11 — Billing. Ported from hifi/hf11-billing.jsx. */

const INVOICES = [
  ['INV-2026-05', '2026-05', 8420.4, 'open', 'in progress'],
  ['INV-2026-04', '2026-04', 9842.2, 'paid', 'auto-debit · 2026-05-01'],
  ['INV-2026-03', '2026-03', 11210.8, 'paid', 'auto-debit · 2026-04-01'],
  ['INV-2026-02', '2026-02', 8120.0, 'paid', 'auto-debit · 2026-03-01'],
  ['INV-2026-01', '2026-01', 7240.1, 'overdue', 'retry scheduled'],
];

const TREND = [7240, 8120, 11210, 9842, 8420, 10200];

const HFBilling = () => {
  const max = Math.max(...TREND);
  return (
    <HFShell
      active='billing'
      crumbs={['my account', 'billing']}
      actions={
        <>
          <button type='button' className='btn'>
            download invoice
          </button>
          <button type='button' className='btn primary'>
            + recharge
          </button>
        </>
      }
    >
      <WIPBanner
        reason='Invoices ($8,420.40 / INV-2026-05) + spend trend + balance + projected exceed are static fixtures. Real data requires lurus-platform billing integration (Epic 12 SKU model).'
        todo='Backend: /api/v2/{slug}/billing/* + platform gRPC GetInvoices / GetProjection. Defer until Epic 12 SKU model is decided.'
      />
      <div className='hf-page-head'>
        <div>
          <div className='lbl' style={{ marginBottom: 6 }}>
            billing
          </div>
          <h1>
            $8,420.40{' '}
            <span className='muted' style={{ fontWeight: 400 }}>
              · this month
            </span>
          </h1>
          <div className='sub'>
            prepaid balance · auto-recharge enabled · projected exceed in 3.2d
          </div>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 28 }}>
          {[
            ['balance', '$1,210', 'var(--hf-ink)'],
            ['mtd spend', '$8,420', 'var(--hf-accent)'],
            ['next invoice', '$10,200', 'var(--hf-ink)'],
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
          padding: 24,
          display: 'grid',
          gridTemplateColumns: '1fr 360px',
          gap: 18,
        }}
      >
        <div className='panel'>
          <div
            style={{
              padding: '14px 18px',
              borderBottom: '1px solid var(--hf-rule)',
            }}
          >
            <div className='lbl'>invoices</div>
          </div>
          <table className='t'>
            <thead>
              <tr>
                <th>invoice</th>
                <th>period</th>
                <th>amount</th>
                <th>status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {INVOICES.map((r, i) => (
                <tr key={i}>
                  <td className='mono strong'>{r[0]}</td>
                  <td className='mono muted'>{r[1]}</td>
                  <td>
                    <span className='display' style={{ fontSize: 16 }}>
                      $
                      {r[2].toLocaleString(undefined, {
                        minimumFractionDigits: 2,
                      })}
                    </span>
                  </td>
                  <td>
                    <span
                      className={
                        'tag ' +
                        (r[3] === 'paid'
                          ? 'ok'
                          : r[3] === 'overdue'
                            ? 'err'
                            : 'warn')
                      }
                    >
                      {r[3]}
                    </span>
                  </td>
                  <td>
                    <button type='button' className='btn ghost sm'>
                      →
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div className='panel' style={{ padding: 18 }}>
            <div className='lbl'>auto-recharge</div>
            <div style={{ marginTop: 8 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <span className='dot ok' />{' '}
                <span className='strong'>enabled</span>
                <span style={{ flex: 1 }} />
                <button type='button' className='btn sm'>
                  edit
                </button>
              </div>
              <div
                className='muted'
                style={{ fontSize: 11, marginTop: 8, lineHeight: 1.7 }}
              >
                top up <b style={{ color: 'var(--hf-ink)' }}>$2,000</b> when
                balance drops below{' '}
                <b style={{ color: 'var(--hf-ink)' }}>$500</b>
              </div>
            </div>
          </div>

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
                VISA
              </div>
              <div>
                <div className='mono strong'>•••• •••• •••• 4242</div>
                <div className='faint mono' style={{ fontSize: 10 }}>
                  exp 08/27 · default
                </div>
              </div>
            </div>
            <button type='button' className='btn sm' style={{ marginTop: 8 }}>
              + add method
            </button>
          </div>

          <div className='panel' style={{ padding: 18 }}>
            <div className='lbl'>trend · last 6 months</div>
            <div
              style={{
                display: 'flex',
                alignItems: 'flex-end',
                gap: 8,
                height: 100,
                marginTop: 14,
              }}
            >
              {TREND.map((v, i) => (
                <div key={i} style={{ flex: 1, textAlign: 'center' }}>
                  <div
                    style={{
                      height: (v / max) * 80 + 'px',
                      background:
                        i === TREND.length - 1
                          ? 'var(--hf-accent)'
                          : 'var(--hf-ink-2)',
                      opacity: i === TREND.length - 1 ? 1 : 0.6,
                    }}
                  />
                  <div
                    className='faint mono'
                    style={{ fontSize: 9, marginTop: 4 }}
                  >
                    {['12', '01', '02', '03', '04', '05'][i]}
                  </div>
                </div>
              ))}
            </div>
            <div
              className='faint mono'
              style={{ fontSize: 10, marginTop: 6, textAlign: 'right' }}
            >
              last bar = forecast
            </div>
          </div>
        </div>
      </div>
    </HFShell>
  );
};

export default HFBilling;
