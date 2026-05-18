import React, { useCallback, useEffect, useRef, useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';
import WIPBanner from '../../../components/hifi/WIPBanner';
import { API, showError, showSuccess } from '../../../helpers';

/*
 * HiFi 12 — Settings.
 * Profile: wired to GET/PUT /api/v2/:tenant_slug/user/me
 * Security/Sessions: partial — /api/v2/client/sessions returns single-session info
 *   (auth_method + active_tokens + request_count), NOT a device/IP list.
 *   Multi-device tracking + per-session revoke needs backend session store —
 *   surfaced via WIPBanner below.
 * Notifications + Team: mocked; see adr-2026-05-18-budget-alerts.md / -tenant-credit-pool.md.
 */

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

const SECTIONS = [
  ['profile', 'Profile', 'name, email, avatar'],
  ['security', 'Security', 'password, mfa, sessions'],
  ['notifications', 'Notifications', 'email & webhook alerts'],
  ['team', 'Team & roles', 'members and permissions'],
  ['integrations', 'Integrations', 'webhooks, slack, observability'],
  ['region', 'Region & data', 'where data lives'],
  ['danger', 'Danger zone', 'export, transfer, delete'],
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

// Shared inline input style (no hf-input class)
const inputStyle = {
  fontFamily: 'var(--hf-mono)',
  fontSize: 12,
  padding: '3px 6px',
  width: '100%',
  border: '1px solid var(--hf-rule)',
  background: 'var(--hf-sunken)',
  color: 'var(--hf-ink)',
  borderRadius: 2,
  outline: 'none',
};

// Inline field editor — commits on Enter or blur, cancels on Escape
const InlineEdit = ({ value, onSave, onCancel }) => {
  const [v, setV] = useState(value ?? '');
  const ref = useRef(null);

  useEffect(() => {
    ref.current?.select();
  }, []);

  const commit = () => {
    const trimmed = v.trim();
    if (trimmed !== (value ?? '').trim()) onSave(trimmed);
    else onCancel();
  };

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

// "Coming soon" note shown under non-profile section headers
const ComingSoon = () => (
  <p
    className='faint mono'
    style={{ fontSize: 10, marginTop: 6, marginBottom: 0 }}
  >
    available in next release
  </p>
);

const HFSettings = () => {
  const tenantSlug = useTenantSlug();
  const [section, setSection] = useState('profile');

  // Profile state
  const [profile, setProfile] = useState(null); // raw API response data
  const [loadingProfile, setLoadingProfile] = useState(true);
  const [editField, setEditField] = useState(null); // 'display_name' | 'email'
  const [saving, setSaving] = useState(false);

  // Session state — single-session for now (backend has no device list)
  const [sessionInfo, setSessionInfo] = useState(null);
  const [sessionLoading, setSessionLoading] = useState(false);

  const fetchProfile = useCallback(async () => {
    setLoadingProfile(true);
    try {
      const res = await API.get(`/api/v2/${tenantSlug}/user/me`);
      if (res?.data?.success) {
        setProfile(res.data.data);
      }
    } catch (_) {
      // error toast shown by API interceptor
    } finally {
      setLoadingProfile(false);
    }
  }, [tenantSlug]);

  const fetchSession = useCallback(async () => {
    setSessionLoading(true);
    try {
      const res = await API.get('/api/v2/client/sessions');
      if (res?.data?.success) {
        setSessionInfo(res.data.data);
      }
    } catch (_) {
      // error toast shown by API interceptor
    } finally {
      setSessionLoading(false);
    }
  }, []);

  useEffect(() => {
    if (tenantSlug) fetchProfile();
  }, [fetchProfile, tenantSlug]);

  useEffect(() => {
    if (section === 'security' && !sessionInfo && !sessionLoading) {
      fetchSession();
    }
  }, [section, sessionInfo, sessionLoading, fetchSession]);

  const handleSaveField = async (field, value) => {
    setEditField(null);
    if (!value) return;
    setSaving(true);
    try {
      const body = { [field]: value };
      const res = await API.put(`/api/v2/${tenantSlug}/user/me`, body);
      if (res?.data?.success) {
        showSuccess('Saved');
        setProfile((prev) => ({ ...prev, ...res.data.data }));
      }
    } catch (_) {
      showError('Save failed');
    } finally {
      setSaving(false);
    }
  };

  // Profile rows: [label, fieldKey | null (read-only), display value, editable]
  const profileRows = profile
    ? [
        ['display name', 'display_name', profile.display_name ?? '—', true],
        ['email', 'email', profile.email ?? '—', true],
        ['username', null, profile.username ?? '—', false],
        ['role', null, profile.role ?? '—', false],
        ['tenant id', null, profile.tenant_id ?? '—', false],
      ]
    : [];

  return (
    <HFShell
      active='settings'
      crumbs={['my account', 'settings']}
    >
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '240px 1fr',
          height: '100%',
          minHeight: 0,
        }}
      >
        {/* Left nav */}
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
              <div className='faint mono' style={{ fontSize: 10, marginTop: 2 }}>
                {d}
              </div>
            </div>
          ))}
        </div>

        {/* Right content */}
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

          {/* ── Profile ── */}
          {section === 'profile' && (
            <div style={{ marginTop: 22 }}>
              {loadingProfile && (
                <div className='muted' style={{ fontSize: 12 }}>Loading…</div>
              )}

              {!loadingProfile && !profile && (
                <div className='muted' style={{ fontSize: 12 }}>
                  Failed to load profile.
                </div>
              )}

              {!loadingProfile && profile && (
                <>
                  {/* Field rows */}
                  <div className='panel'>
                    {profileRows.map(([label, fieldKey, value, editable], i, a) => (
                      <div
                        key={label}
                        style={{
                          display: 'grid',
                          gridTemplateColumns: '160px 1fr auto',
                          padding: '14px 16px',
                          borderBottom:
                            i < a.length - 1 ? '1px dashed var(--hf-rule)' : 0,
                          alignItems: 'center',
                          gap: 12,
                        }}
                      >
                        <span className='lbl'>{label}</span>

                        {editable && editField === fieldKey ? (
                          <InlineEdit
                            value={value === '—' ? '' : value}
                            onSave={(v) => handleSaveField(fieldKey, v)}
                            onCancel={() => setEditField(null)}
                          />
                        ) : (
                          <span className='strong' style={{ fontSize: 13 }}>
                            {value}
                          </span>
                        )}

                        {editable && editField !== fieldKey ? (
                          <button
                            type='button'
                            className='btn ghost sm'
                            disabled={saving}
                            onClick={() => setEditField(fieldKey)}
                          >
                            edit
                          </button>
                        ) : (
                          /* Keep grid alignment for read-only rows */
                          <span />
                        )}
                      </div>
                    ))}
                  </div>

                  {/* Usage summary */}
                  <div
                    className='panel'
                    style={{
                      marginTop: 16,
                      padding: '14px 16px',
                      display: 'flex',
                      gap: 32,
                    }}
                  >
                    <div>
                      <div className='lbl' style={{ marginBottom: 4 }}>spent</div>
                      <div className='display' style={{ fontSize: 22 }}>
                        ${(profile.used_quota / QUOTA_PER_USD).toFixed(2)}
                      </div>
                    </div>
                    <div>
                      <div className='lbl' style={{ marginBottom: 4 }}>requests</div>
                      <div className='display' style={{ fontSize: 22 }}>
                        {(profile.request_count ?? 0).toLocaleString()}
                      </div>
                    </div>
                  </div>
                </>
              )}
            </div>
          )}

          {/* ── Security ── */}
          {section === 'security' && (
            <div style={{ marginTop: 22 }}>
              <ComingSoon />
              <div className='panel' style={{ padding: 18, marginBottom: 14, marginTop: 16 }}>
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
                  <button type='button' className='btn sm' disabled>
                    regenerate
                  </button>
                </div>
              </div>
              <WIPBanner
                reason='Multi-device session tracking is not implemented. The endpoint /api/v2/client/sessions returns only the *current* session (auth_method + active_tokens + request_count), not a per-device list with revoke.'
                todo='Backend: device session store + DELETE /api/v2/client/sessions/:id; then surface list here.'
              />
              <div className='panel' style={{ padding: 18, marginTop: 14, marginBottom: 14 }}>
                <div className='lbl'>current session</div>
                {sessionLoading && (
                  <div
                    className='muted'
                    style={{ fontSize: 12, marginTop: 10 }}
                  >
                    Loading…
                  </div>
                )}
                {!sessionLoading && !sessionInfo && (
                  <div
                    className='muted'
                    style={{ fontSize: 12, marginTop: 10 }}
                  >
                    Failed to load session info.
                  </div>
                )}
                {!sessionLoading && sessionInfo && (
                  <div
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '1fr auto auto',
                      gap: 12,
                      padding: '10px 0 0',
                      alignItems: 'center',
                    }}
                  >
                    <span className='strong' style={{ fontSize: 12 }}>
                      {sessionInfo.username || '—'}
                      <span
                        className='mono muted'
                        style={{ fontSize: 10, marginLeft: 8 }}
                      >
                        · auth: {sessionInfo.auth_method || 'unknown'}
                      </span>
                    </span>
                    <span className='tag ok'>current</span>
                    <span className='faint mono' style={{ fontSize: 10 }}>
                      {(sessionInfo.active_tokens ?? 0)} active tokens
                    </span>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* ── Notifications ── */}
          {section === 'notifications' && (
            <div style={{ marginTop: 22 }}>
              <WIPBanner
                reason='Notification subscription store, dispatch path, and threshold rules not yet implemented. Designed in adr-2026-05-18-budget-alerts.md.'
                todo='Backend: notification_subscription table + /api/v2/{slug}/notifications/subscriptions + Prometheus rule pack.'
              />
              <div
                className='panel'
                style={{
                  marginTop: 14,
                  padding: 24,
                  textAlign: 'center',
                  color: 'var(--hf-ink-3)',
                  fontFamily: 'var(--hf-mono)',
                  fontSize: 12,
                }}
              >
                No subscriptions — subscription API not implemented.
              </div>
            </div>
          )}

          {/* ── Team ── */}
          {section === 'team' && (
            <div style={{ marginTop: 22 }}>
              <WIPBanner
                reason='Team / role management requires tenant membership store + role-permission matrix + invite flow. Not yet implemented.'
                todo='Backend: tenant_member table + role enum + POST /api/v2/{slug}/team/invite; cascade-revoke on member removal.'
              />
              <div
                className='panel'
                style={{
                  marginTop: 14,
                  padding: 24,
                  textAlign: 'center',
                  color: 'var(--hf-ink-3)',
                  fontFamily: 'var(--hf-mono)',
                  fontSize: 12,
                }}
              >
                No team members — endpoint not implemented.
              </div>
            </div>
          )}

          {/* ── Integrations ── */}
          {section === 'integrations' && (
            <div style={{ marginTop: 22 }}>
              <ComingSoon />
              <div
                style={{
                  marginTop: 16,
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
                      <button type='button' className='btn sm' disabled>
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
            </div>
          )}

          {/* ── Region ── */}
          {section === 'region' && (
            <div style={{ marginTop: 22 }}>
              <ComingSoon />
              <div className='panel' style={{ padding: 18, marginTop: 16 }}>
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

          {/* ── Danger ── */}
          {section === 'danger' && (
            <div style={{ marginTop: 22 }}>
              <ComingSoon />
              <div
                className='panel'
                style={{
                  padding: 18,
                  border: '1px solid var(--hf-err)',
                  marginTop: 16,
                }}
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
                  disabled
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
