import React from 'react';
import HFShell from '../../../components/hifi/HFShell';

/* HiFi 10 — Pricing admin. Ported from hifi/hf10-pricing.jsx. */

const ROWS = [
  ['gpt-4o', 2.5, 10.0, 1.4],
  ['gpt-4o-mini', 0.15, 0.6, 1.5],
  ['claude-3.5-sonnet', 3.0, 15.0, 1.35],
  ['claude-3.5-haiku', 0.8, 4.0, 1.5],
  ['gemini-1.5-pro', 1.25, 5.0, 1.4],
  ['gemini-1.5-flash', 0.075, 0.3, 1.6],
  ['deepseek-v3', 0.27, 1.1, 1.8],
];

const PLANS = [
  ['enterprise', '×0.85', 'volume discount'],
  ['team', '×1.00', 'standard'],
  ['startup', '×1.10', '+10% premium'],
  ['free', '∞', 'free tier · capped'],
];

const HFPricing = () => {
  return (
    <HFShell
      active='pricing'
      crumbs={['platform · admin', 'pricing']}
      actions={
        <>
          <button type='button' className='btn'>
            currency: USD ▾
          </button>
          <button type='button' className='btn primary'>
            save changes
          </button>
        </>
      }
    >
      <div className='hf-page-head'>
        <div>
          <div className='lbl' style={{ marginBottom: 6 }}>
            pricing rules
          </div>
          <h1>Margin & markup engine</h1>
          <div className='sub'>
            vendor cost → tenant invoice · per-model markup · plan multipliers
          </div>
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
              display: 'flex',
              alignItems: 'baseline',
            }}
          >
            <div className='lbl'>model markup</div>
            <span
              className='muted mono'
              style={{ fontSize: 10, marginLeft: 'auto' }}
            >
              54 models · last edit 3d ago
            </span>
          </div>
          <table className='t'>
            <thead>
              <tr>
                <th>model</th>
                <th>vendor cost · in</th>
                <th>vendor cost · out</th>
                <th>markup</th>
                <th>tenant price · in</th>
                <th>tenant price · out</th>
                <th>margin</th>
              </tr>
            </thead>
            <tbody>
              {ROWS.map((r, i) => {
                const pIn = r[1] * r[3];
                const pOut = r[2] * r[3];
                const mIn = (1 - 1 / r[3]) * 100;
                return (
                  <tr key={i}>
                    <td className='strong'>{r[0]}</td>
                    <td className='mono muted'>${r[1].toFixed(2)}</td>
                    <td className='mono muted'>${r[2].toFixed(2)}</td>
                    <td>
                      <div
                        className='field'
                        style={{ width: 80, height: 24, fontSize: 11 }}
                      >
                        ×{r[3].toFixed(2)}
                      </div>
                    </td>
                    <td className='mono strong'>${pIn.toFixed(2)}</td>
                    <td className='mono strong'>${pOut.toFixed(2)}</td>
                    <td>
                      <span className='tag ok'>{mIn.toFixed(0)}%</span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div className='panel' style={{ padding: 18 }}>
            <div className='lbl'>plan multipliers</div>
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: 8,
                marginTop: 10,
              }}
            >
              {PLANS.map((r, i) => (
                <div
                  key={i}
                  style={{
                    display: 'grid',
                    gridTemplateColumns: 'auto auto 1fr',
                    gap: 10,
                    alignItems: 'center',
                    padding: '6px 0',
                    borderBottom:
                      i < PLANS.length - 1 ? '1px dashed var(--hf-rule)' : 0,
                  }}
                >
                  <span
                    className='tag'
                    style={{ minWidth: 70, justifyContent: 'center' }}
                  >
                    {r[0]}
                  </span>
                  <span className='display' style={{ fontSize: 18 }}>
                    {r[1]}
                  </span>
                  <span
                    className='muted'
                    style={{ fontSize: 11, textAlign: 'right' }}
                  >
                    {r[2]}
                  </span>
                </div>
              ))}
            </div>
          </div>

          <div className='panel' style={{ padding: 18 }}>
            <div className='lbl'>forecast · this month</div>
            <div className='display' style={{ fontSize: 32, marginTop: 6 }}>
              $8,420
            </div>
            <div className='muted' style={{ fontSize: 11 }}>
              gross margin · 32% · vs last month +4pp
            </div>
            <hr
              style={{
                border: 0,
                borderTop: '1px dashed var(--hf-rule)',
                margin: '12px 0',
              }}
            />
            <div style={{ fontSize: 11, lineHeight: 1.7 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span className='muted'>vendor cost</span>
                <span className='mono'>$17,820</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span className='muted'>tenant revenue</span>
                <span className='mono'>$26,240</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span className='strong'>net</span>
                <span className='mono strong'>$8,420</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </HFShell>
  );
};

export default HFPricing;
