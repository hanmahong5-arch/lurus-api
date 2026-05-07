import React from 'react';
import HFShell from '../../../components/hifi/HFShell';

/* HiFi 9 — Tenants admin. Ported from hifi/hf9-tenants.jsx (2026-05-07 variants pack). */

const TENANTS = [
  {
    name: 'acme',
    plan: 'enterprise',
    users: 42,
    qps: 142,
    mtd: 8420,
    cap: 10000,
    state: 'near-cap',
  },
  {
    name: 'contoso',
    plan: 'team',
    users: 18,
    qps: 88,
    mtd: 6210,
    cap: 8000,
    state: 'ok',
  },
  {
    name: 'globex',
    plan: 'team',
    users: 12,
    qps: 41,
    mtd: 4180,
    cap: 5000,
    state: 'ok',
  },
  {
    name: 'initech',
    plan: 'startup',
    users: 8,
    qps: 18,
    mtd: 3122,
    cap: 3500,
    state: 'near-cap',
  },
  {
    name: 'umbrella',
    plan: 'startup',
    users: 4,
    qps: 12,
    mtd: 812,
    cap: 2000,
    state: 'ok',
  },
  {
    name: 'wayne',
    plan: 'free',
    users: 1,
    qps: 2,
    mtd: 18,
    cap: 100,
    state: 'ok',
  },
];

const HFTenants = () => {
  return (
    <HFShell
      active='users'
      crumbs={['platform · admin', 'tenants']}
      actions={
        <>
          <button type='button' className='btn'>
            plan: all ▾
          </button>
          <button type='button' className='btn primary'>
            + invite tenant
          </button>
        </>
      }
    >
      <div className='hf-page-head'>
        <div>
          <div className='lbl' style={{ marginBottom: 6 }}>
            tenants
          </div>
          <h1>
            23 tenants{' '}
            <span className='muted' style={{ fontWeight: 400 }}>
              · $24.1k mtd
            </span>
          </h1>
          <div className='sub'>
            isolation · per-tenant keys · per-tenant budgets
          </div>
        </div>
      </div>

      <div style={{ padding: 24 }}>
        <div className='panel'>
          <table className='t'>
            <thead>
              <tr>
                <th>tenant</th>
                <th>plan</th>
                <th>users</th>
                <th>qps</th>
                <th>spend · mtd</th>
                <th>budget · mtd</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {TENANTS.map((t, i) => {
                const pct = t.mtd / t.cap;
                return (
                  <tr key={i}>
                    <td>
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 10,
                        }}
                      >
                        <div
                          style={{
                            width: 28,
                            height: 28,
                            background: 'var(--hf-sunken)',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            fontFamily: 'var(--hf-display)',
                            fontWeight: 600,
                            color: 'var(--hf-ink)',
                          }}
                        >
                          {t.name[0].toUpperCase()}
                        </div>
                        <div>
                          <div className='strong'>{t.name}</div>
                          <div className='faint mono' style={{ fontSize: 10 }}>
                            tenant_{t.name}.lurus
                          </div>
                        </div>
                      </div>
                    </td>
                    <td>
                      <span
                        className={
                          'tag ' +
                          (t.plan === 'enterprise'
                            ? 'acc'
                            : t.plan === 'free'
                              ? ''
                              : 'info')
                        }
                      >
                        {t.plan}
                      </span>
                    </td>
                    <td className='mono'>{t.users}</td>
                    <td className='mono'>{t.qps}</td>
                    <td>
                      <span className='display' style={{ fontSize: 16 }}>
                        ${t.mtd.toLocaleString()}
                      </span>
                    </td>
                    <td>
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 8,
                        }}
                      >
                        <div
                          style={{
                            width: 100,
                            height: 4,
                            background: 'var(--hf-sunken)',
                          }}
                        >
                          <div
                            style={{
                              width: pct * 100 + '%',
                              height: '100%',
                              background:
                                pct > 0.9
                                  ? 'var(--hf-err)'
                                  : pct > 0.75
                                    ? 'var(--hf-warn)'
                                    : 'var(--hf-ok)',
                            }}
                          />
                        </div>
                        <span
                          className='mono'
                          style={{
                            fontSize: 10,
                            color:
                              pct > 0.9 ? 'var(--hf-err)' : 'var(--hf-ink-3)',
                          }}
                        >
                          {(pct * 100).toFixed(0)}%
                        </span>
                      </div>
                    </td>
                    <td>
                      <button type='button' className='btn ghost sm'>
                        manage →
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </HFShell>
  );
};

export default HFTenants;
