import React, { useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';

/* HiFi 12 — Settings. Ported from hifi/hf12-settings.jsx. */

const SECTIONS = [
  ['profile', 'Profile', 'name, email, avatar'],
  ['security', 'Security', 'password, mfa, sessions'],
  ['notifications', 'Notifications', 'email & webhook alerts'],
  ['team', 'Team & roles', 'members and permissions'],
  ['integrations', 'Integrations', 'webhooks, slack, observability'],
  ['region', 'Region & data', 'where data lives'],
  ['danger', 'Danger zone', 'export, transfer, delete'],
];

const PROFILE = [
  ['name', 'Andy Liu'],
  ['email', 'andy@acme.com'],
  ['handle', '@andyliu'],
  ['avatar', '— upload —'],
  ['locale', 'zh-CN'],
  ['timezone', 'Asia/Shanghai (UTC+8)'],
];

const SESSIONS = [
  ['MacBook Pro · Shanghai', 'current', '2m ago'],
  ['iPhone · Shanghai', '', '3h ago'],
  ['Chrome · Beijing', '', '2d ago'],
];

const NOTIFICATIONS = [
  ['budget threshold reached', 'email · slack', true],
  ['channel goes down', 'email · slack · pagerduty', true],
  ['error rate spike', 'slack', true],
  ['weekly usage digest', 'email', true],
  ['new model available', 'email', false],
  ['invoice issued', 'email', true],
];

const TEAM = [
  ['Andy Liu', 'owner', 'now'],
  ['Mei Chen', 'admin', '12m'],
  ['Raj Patel', 'developer', '2h'],
  ['Lisa Park', 'viewer', '1d'],
];

const INTEGRATIONS = [
  ['Slack', 'connected · #alerts', 'ok'],
  ['PagerDuty', 'connected · sev1 only', 'ok'],
  ['Datadog', 'metrics export', 'ok'],
  ['Webhook', '2 endpoints', 'ok'],
  ['Sentry', 'not connected', 'idle'],
  ['Discord', 'not connected', 'idle'],
];

const REGIONS = [
  ['us-west', false],
  ['eu-frankfurt', false],
  ['ap-shanghai', true],
];

const HFSettings = () => {
  const [section, setSection] = useState('profile');

  return (
    <HFShell
      active='settings'
      crumbs={['platform · admin', 'settings']}
      actions={
        <button type='button' className='btn primary'>
          save
        </button>
      }
    >
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '240px 1fr',
          height: '100%',
          minHeight: 0,
        }}
      >
        <div
          style={{
            borderRight: '1px solid var(--hf-rule)',
            background: 'var(--hf-paper)',
            padding: '20px 0',
          }}
        >
          {SECTIONS.map(([k, l, d]) => (
            <div
              key={k}
              onClick={() => setSection(k)}
              style={{
                padding: '10px 22px',
                cursor: 'pointer',
                background: section === k ? 'var(--hf-elev)' : 'transparent',
                borderLeft:
                  section === k
                    ? '2px solid var(--hf-accent)'
                    : '2px solid transparent',
              }}
            >
              <div className='strong' style={{ fontSize: 12 }}>
                {l}
              </div>
              <div
                className='faint mono'
                style={{ fontSize: 10, marginTop: 2 }}
              >
                {d}
              </div>
            </div>
          ))}
        </div>

        <div style={{ overflow: 'auto', padding: 28, maxWidth: 720 }}>
          <div className='lbl' style={{ marginBottom: 4 }}>
            settings · {section}
          </div>
          <h1
            className='display'
            style={{ fontSize: 32, margin: 0, letterSpacing: '-0.025em' }}
          >
            {SECTIONS.find((s) => s[0] === section)[1]}
          </h1>

          {section === 'profile' && (
            <div style={{ marginTop: 22 }}>
              {PROFILE.map((r, i, a) => (
                <div
                  key={i}
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '160px 1fr auto',
                    padding: '14px 0',
                    borderBottom:
                      i < a.length - 1 ? '1px dashed var(--hf-rule)' : 0,
                    alignItems: 'center',
                  }}
                >
                  <span className='lbl'>{r[0]}</span>
                  <span className='strong' style={{ fontSize: 13 }}>
                    {r[1]}
                  </span>
                  <button type='button' className='btn ghost sm'>
                    edit
                  </button>
                </div>
              ))}
            </div>
          )}

          {section === 'security' && (
            <div style={{ marginTop: 22 }}>
              <div className='panel' style={{ padding: 18, marginBottom: 14 }}>
                <div className='lbl'>multi-factor auth</div>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 10,
                    marginTop: 10,
                  }}
                >
                  <span className='dot ok' />{' '}
                  <span className='strong'>authenticator app · enabled</span>
                  <span style={{ flex: 1 }} />
                  <button type='button' className='btn sm'>
                    regenerate
                  </button>
                </div>
              </div>
              <div className='panel' style={{ padding: 18, marginBottom: 14 }}>
                <div className='lbl'>active sessions · 3</div>
                {SESSIONS.map((r, i) => (
                  <div
                    key={i}
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '1fr auto auto auto',
                      gap: 12,
                      padding: '10px 0',
                      borderBottom:
                        i < SESSIONS.length - 1
                          ? '1px dashed var(--hf-rule)'
                          : 0,
                      alignItems: 'center',
                    }}
                  >
                    <span className='strong' style={{ fontSize: 12 }}>
                      {r[0]}
                    </span>
                    {r[1] && <span className='tag ok'>{r[1]}</span>}
                    <span className='faint mono' style={{ fontSize: 10 }}>
                      {r[2]}
                    </span>
                    {!r[1] && (
                      <button
                        type='button'
                        className='btn ghost sm'
                        style={{ color: 'var(--hf-err)' }}
                      >
                        revoke
                      </button>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {section === 'notifications' && (
            <div style={{ marginTop: 22 }}>
              <div className='panel'>
                {NOTIFICATIONS.map((r, i, a) => (
                  <div
                    key={i}
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '1fr auto auto',
                      padding: '14px 18px',
                      borderBottom:
                        i < a.length - 1 ? '1px solid var(--hf-rule)' : 0,
                      alignItems: 'center',
                      gap: 14,
                    }}
                  >
                    <div>
                      <div className='strong' style={{ fontSize: 12 }}>
                        {r[0]}
                      </div>
                      <div className='faint mono' style={{ fontSize: 10 }}>
                        {r[1]}
                      </div>
                    </div>
                    <button type='button' className='btn ghost sm'>
                      channels →
                    </button>
                    <div
                      style={{
                        width: 32,
                        height: 18,
                        background: r[2]
                          ? 'var(--hf-accent)'
                          : 'var(--hf-rule-strong)',
                        borderRadius: 9,
                        position: 'relative',
                      }}
                    >
                      <div
                        style={{
                          position: 'absolute',
                          width: 14,
                          height: 14,
                          background: '#fff',
                          borderRadius: 7,
                          top: 2,
                          left: r[2] ? 16 : 2,
                        }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {section === 'team' && (
            <div style={{ marginTop: 22 }}>
              <div className='panel'>
                <table className='t'>
                  <thead>
                    <tr>
                      <th>member</th>
                      <th>role</th>
                      <th>last seen</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {TEAM.map((r, i) => (
                      <tr key={i}>
                        <td>
                          <span className='strong'>{r[0]}</span>
                        </td>
                        <td>
                          <span className='tag'>{r[1]}</span>
                        </td>
                        <td className='faint mono'>{r[2]}</td>
                        <td>
                          <button type='button' className='btn ghost sm'>
                            ⋯
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <div
                  style={{ padding: 14, borderTop: '1px solid var(--hf-rule)' }}
                >
                  <button type='button' className='btn primary'>
                    + invite member
                  </button>
                </div>
              </div>
            </div>
          )}

          {section === 'integrations' && (
            <div
              style={{
                marginTop: 22,
                display: 'grid',
                gridTemplateColumns: 'repeat(2, 1fr)',
                gap: 14,
              }}
            >
              {INTEGRATIONS.map((r, i) => (
                <div key={i} className='panel' style={{ padding: 16 }}>
                  <div
                    style={{ display: 'flex', alignItems: 'center', gap: 8 }}
                  >
                    <span className={'dot ' + r[2]} />
                    <span className='display' style={{ fontSize: 16 }}>
                      {r[0]}
                    </span>
                    <span style={{ flex: 1 }} />
                    <button type='button' className='btn sm'>
                      {r[2] === 'ok' ? 'configure' : 'connect'}
                    </button>
                  </div>
                  <div
                    className='faint mono'
                    style={{ fontSize: 10, marginTop: 6 }}
                  >
                    {r[1]}
                  </div>
                </div>
              ))}
            </div>
          )}

          {section === 'region' && (
            <div style={{ marginTop: 22 }}>
              <div className='panel' style={{ padding: 18 }}>
                <div className='lbl'>data residency</div>
                <div
                  style={{
                    display: 'grid',
                    gridTemplateColumns: 'repeat(3, 1fr)',
                    gap: 10,
                    marginTop: 12,
                  }}
                >
                  {REGIONS.map(([r, sel], i) => (
                    <div
                      key={i}
                      className='panel-paper'
                      style={{
                        padding: 12,
                        border: sel
                          ? '2px solid var(--hf-accent)'
                          : '1px solid var(--hf-rule)',
                      }}
                    >
                      <div className='strong' style={{ fontSize: 13 }}>
                        {r}
                      </div>
                      <div
                        className='faint mono'
                        style={{ fontSize: 10, marginTop: 4 }}
                      >
                        {sel ? 'current' : 'available'}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}

          {section === 'danger' && (
            <div style={{ marginTop: 22 }}>
              <div
                className='panel'
                style={{ padding: 18, border: '1px solid var(--hf-err)' }}
              >
                <div
                  className='display'
                  style={{ fontSize: 16, color: 'var(--hf-err)' }}
                >
                  Delete account
                </div>
                <div
                  className='muted'
                  style={{ fontSize: 12, marginTop: 6, lineHeight: 1.6 }}
                >
                  permanently deletes all data: tokens, logs, channels,
                  invoices. cannot be undone.
                </div>
                <button
                  type='button'
                  className='btn'
                  style={{
                    marginTop: 12,
                    color: 'var(--hf-err)',
                    borderColor: 'var(--hf-err)',
                  }}
                >
                  I understand · delete
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </HFShell>
  );
};

export default HFSettings;
