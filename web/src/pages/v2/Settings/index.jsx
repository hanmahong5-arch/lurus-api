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
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import HFShell from '../../../components/hifi/HFShell';
import WIPBanner from '../../../components/hifi/WIPBanner';
import ConfirmDialog from '../../../components/common/ConfirmDialog';
import { API, showError, showSuccess } from '../../../helpers';

// Wave 2: only security section is wired; notifications/team/integrations/MFA remain stubs pending infra.
// Wave 3 Phase 1 (2026-05-20): revoke session wired to DELETE /sessions/current.

/*
 * HiFi 12 — Settings.
 * Profile: wired to GET/PUT /api/v2/:tenant_slug/user/me
 * Security/Sessions: wired to GET /api/v2/:tenant_slug/sessions (Wave 2).
 *   Returns single synthetic session (auth_method + active_tokens + request_count).
 *   Single-device revoke wired (Wave 3 Phase 1). Multi-device tracking deferred to v3.
 * Notifications + Team: mocked; see adr-2026-05-18-budget-alerts.md / -tenant-credit-pool.md.
 */

const QUOTA_PER_USD = 500_000;

const fmtCNY = (v) =>
  typeof v === 'number'
    ? '¥' +
      v.toLocaleString(undefined, {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      })
    : '—';

// formatRelativeTime converts a Unix timestamp (seconds) to a human-readable
// relative string (e.g. "just now", "3 minutes ago"). No external dependency —
// avoids adding date-fns for a single call site.
const formatRelativeTime = (unixSec) => {
  if (!unixSec) return '—';
  const diffSec = Math.floor(Date.now() / 1000) - unixSec;
  if (diffSec < 60) return '刚刚';
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)} 分钟前`;
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} 小时前`;
  return `${Math.floor(diffSec / 86400)} 天前`;
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

const SECTIONS = [
  ['profile', 'Profile', 'name, email, avatar'],
  ['security', 'Security', 'password, mfa, sessions'],
  ['subscription', 'Subscription', 'plan tier & entitlements'],
  ['billing', 'Billing', 'wallet balance & usage'],
  ['notifications', 'Notifications', 'email & webhook alerts'],
  ['team', 'Team & roles', 'members and permissions'],
  ['integrations', 'Integrations', 'webhooks, slack, observability'],
  ['region', 'Region & data', 'where data lives'],
  ['danger', 'Danger zone', 'export, transfer, delete'],
];

// Wave A Squad 5A (2026-05-20): read-only entitlement summary. Values are
// derived locally from the existing /user/me `group` field — backend
// entitlement registry not yet implemented. See caveats in commit body.
// Order: [label, value]. Tooltip on the upgrade button explains scope.
const ENTITLEMENT_BY_GROUP = {
  default: {
    label: 'Free',
    routing: 'shared pool',
    sla: 'best effort',
    auditDays: 7,
    support: 'community',
  },
  vip: {
    label: 'Pro',
    routing: 'priority routing',
    sla: '99.5%',
    auditDays: 30,
    support: 'business hours',
  },
  pro: {
    label: 'Pro',
    routing: 'priority routing',
    sla: '99.5%',
    auditDays: 30,
    support: 'business hours',
  },
  enterprise: {
    label: 'Enterprise',
    routing: 'dedicated pool',
    sla: '99.95%',
    auditDays: 365,
    support: '24/7 dedicated',
  },
};

// Wave A Squad 5A: 3 read-only notification channels with placeholder events.
// Toggle switches are disabled — mutation flow lands in Wave B per
// adr-2026-05-18-budget-alerts.md.
const NOTIFICATION_CHANNELS = [
  {
    key: 'email',
    label: 'Email notifications',
    events: ['Quota threshold', 'Plan limit', 'Security event'],
  },
  {
    key: 'webhook',
    label: 'Webhook',
    events: ['Quota threshold', 'Plan limit', 'Security event'],
  },
  {
    key: 'inapp',
    label: 'In-app',
    events: ['Quota threshold', 'Plan limit', 'Security event'],
  },
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
  const navigate = useNavigate();
  const [section, setSection] = useState('profile');

  // Profile state
  const [profile, setProfile] = useState(null); // raw API response data
  const [loadingProfile, setLoadingProfile] = useState(true);
  const [editField, setEditField] = useState(null); // 'display_name' | 'email'
  const [saving, setSaving] = useState(false);

  // Session state — single-session for now (backend has no device list)
  const [sessions, setSessions] = useState(null);
  const [sessionLoading, setSessionLoading] = useState(false);

  // Revoke session confirm dialog
  const [revokeVisible, setRevokeVisible] = useState(false);
  const [revoking, setRevoking] = useState(false);

  // Subscription tab (Wave A Squad 5A) — derived from /user/me + best-effort
  // /user/billing/summary. No dedicated subscription endpoint yet.
  const [subLoading, setSubLoading] = useState(false);
  const [subError, setSubError] = useState(false);
  const [subData, setSubData] = useState(null); // { tier, group, source }

  // Billing tab (Wave A Squad 5A) — wallet summary + last-30d aggregate +
  // recent transactions (synthesised from /billing/topups; full ClickHouse
  // history is Wave C).
  const [billLoading, setBillLoading] = useState(false);
  const [billError, setBillError] = useState(false);
  const [billSummary, setBillSummary] = useState(null);
  const [billTxns, setBillTxns] = useState([]);

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

  const fetchSessions = useCallback(async () => {
    setSessionLoading(true);
    try {
      const res = await API.get(`/api/v2/${tenantSlug}/sessions`);
      if (res?.data?.success) {
        setSessions(res.data.data.items ?? []);
      }
    } catch (_) {
      // error toast shown by API interceptor
    } finally {
      setSessionLoading(false);
    }
  }, [tenantSlug]);

  // Subscription tab loader — reuses the cached profile when present (Profile
  // tab fetches it on mount), otherwise refetches. Falls back to a placeholder
  // if no group field is exposed. Best-effort enrichment via billing summary
  // (subscription_plan field is currently absent on the Go BillingSummary
  // struct — guard accordingly).
  const fetchSubscription = useCallback(async () => {
    setSubLoading(true);
    setSubError(false);
    try {
      let p = profile;
      if (!p) {
        const res = await API.get(`/api/v2/${tenantSlug}/user/me`);
        if (res?.data?.success) {
          p = res.data.data;
          setProfile(p);
        }
      }
      // Best-effort: try platform billing summary for subscription_plan hint.
      let planHint = null;
      try {
        const sumRes = await API.get('/api/v2/user/billing/summary');
        if (sumRes?.data?.success) {
          planHint = sumRes.data.data?.subscription_plan ?? null;
        }
      } catch (_) {
        // non-fatal — subscription_plan field optional
      }

      if (p) {
        setSubData({
          group: p.group ?? null,
          planHint,
          source: planHint
            ? 'platform'
            : p.group
              ? 'user_group'
              : 'placeholder',
        });
      } else {
        setSubError(true);
      }
    } catch (_) {
      setSubError(true);
    } finally {
      setSubLoading(false);
    }
  }, [tenantSlug, profile]);

  // Billing tab loader — pulls platform balance + tenant-scoped topup history.
  // Both calls are tolerant of 4xx/5xx: if either fails we surface error state
  // but never throw uncaught. Full transaction history (ClickHouse) is Wave C.
  const fetchBilling = useCallback(async () => {
    setBillLoading(true);
    setBillError(false);
    try {
      const [sumRes, txnRes] = await Promise.allSettled([
        API.get('/api/v2/user/billing/summary'),
        API.get(`/api/v2/${tenantSlug}/billing/topups?p=1&size=5`),
      ]);

      let gotSummary = false;
      if (
        sumRes.status === 'fulfilled' &&
        sumRes.value?.data?.success &&
        sumRes.value.data.data
      ) {
        setBillSummary(sumRes.value.data.data);
        gotSummary = true;
      }

      let gotTxns = false;
      if (
        txnRes.status === 'fulfilled' &&
        txnRes.value?.data?.success &&
        Array.isArray(txnRes.value.data.data?.items)
      ) {
        setBillTxns(txnRes.value.data.data.items);
        gotTxns = true;
      }

      // Treat "both failed" as error; one succeeding still renders a useful tab.
      if (!gotSummary && !gotTxns) {
        setBillError(true);
      }
    } catch (_) {
      setBillError(true);
    } finally {
      setBillLoading(false);
    }
  }, [tenantSlug]);

  useEffect(() => {
    if (tenantSlug) fetchProfile();
  }, [fetchProfile, tenantSlug]);

  useEffect(() => {
    if (section === 'security' && !sessions && !sessionLoading) {
      fetchSessions();
    }
  }, [section, sessions, sessionLoading, fetchSessions]);

  useEffect(() => {
    if (section === 'subscription' && !subData && !subLoading) {
      fetchSubscription();
    }
  }, [section, subData, subLoading, fetchSubscription]);

  useEffect(() => {
    if (
      section === 'billing' &&
      !billSummary &&
      !billTxns.length &&
      !billLoading
    ) {
      fetchBilling();
    }
  }, [section, billSummary, billTxns.length, billLoading, fetchBilling]);

  const handleRevokeSession = async () => {
    if (revoking) return;
    setRevoking(true);
    try {
      const res = await API.delete(`/api/v2/${tenantSlug}/sessions/current`);
      if (res?.data?.success) {
        const redirect = res.data.data?.redirect ?? '/console/v2/login';
        navigate(redirect);
      } else {
        showError(res?.data?.message || '撤销会话失败');
      }
    } catch (_) {
      // interceptor handles toast
    } finally {
      setRevoking(false);
      setRevokeVisible(false);
    }
  };

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
    <HFShell active='settings' crumbs={['my account', 'settings']}>
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
              <div
                className='faint mono'
                style={{ fontSize: 10, marginTop: 2 }}
              >
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
                <div className='muted' style={{ fontSize: 12 }}>
                  Loading…
                </div>
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
                    {profileRows.map(
                      ([label, fieldKey, value, editable], i, a) => (
                        <div
                          key={label}
                          style={{
                            display: 'grid',
                            gridTemplateColumns: '160px 1fr auto',
                            padding: '14px 16px',
                            borderBottom:
                              i < a.length - 1
                                ? '1px dashed var(--hf-rule)'
                                : 0,
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
                      ),
                    )}
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
                      <div className='lbl' style={{ marginBottom: 4 }}>
                        spent
                      </div>
                      <div className='display' style={{ fontSize: 22 }}>
                        ${(profile.used_quota / QUOTA_PER_USD).toFixed(2)}
                      </div>
                    </div>
                    <div>
                      <div className='lbl' style={{ marginBottom: 4 }}>
                        requests
                      </div>
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
              <div
                className='panel'
                style={{ padding: 18, marginBottom: 14, marginTop: 8 }}
              >
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
                  <button
                    type='button'
                    className='btn sm'
                    disabled
                    title='MFA infra pending — totp_seed table not yet provisioned'
                  >
                    regenerate
                  </button>
                </div>
              </div>

              <div
                className='panel'
                style={{ padding: 18, marginTop: 14, marginBottom: 14 }}
              >
                <div className='lbl' style={{ marginBottom: 12 }}>
                  sessions
                </div>
                {sessionLoading && (
                  <div
                    className='muted'
                    style={{ fontSize: 12, marginTop: 10 }}
                  >
                    Loading…
                  </div>
                )}
                {!sessionLoading && !sessions && (
                  <div
                    className='muted'
                    style={{ fontSize: 12, marginTop: 10 }}
                  >
                    Failed to load sessions.
                  </div>
                )}
                {!sessionLoading && sessions && sessions.length > 0 && (
                  <table
                    data-testid='sessions-table'
                    style={{
                      width: '100%',
                      borderCollapse: 'collapse',
                      fontFamily: 'var(--hf-mono)',
                      fontSize: 11,
                    }}
                  >
                    <thead>
                      <tr style={{ color: 'var(--hf-ink-3)' }}>
                        <th
                          style={{
                            textAlign: 'left',
                            padding: '4px 8px',
                            fontWeight: 500,
                          }}
                        >
                          auth method
                        </th>
                        <th
                          style={{
                            textAlign: 'left',
                            padding: '4px 8px',
                            fontWeight: 500,
                          }}
                        >
                          current
                        </th>
                        <th
                          style={{
                            textAlign: 'right',
                            padding: '4px 8px',
                            fontWeight: 500,
                          }}
                        >
                          active tokens
                        </th>
                        <th
                          style={{
                            textAlign: 'right',
                            padding: '4px 8px',
                            fontWeight: 500,
                          }}
                        >
                          requests (30d)
                        </th>
                        <th
                          style={{
                            textAlign: 'right',
                            padding: '4px 8px',
                            fontWeight: 500,
                          }}
                        >
                          last seen
                        </th>
                        <th style={{ padding: '4px 8px' }} />
                      </tr>
                    </thead>
                    <tbody>
                      {sessions.map((s, i) => (
                        <tr
                          key={s.id ?? i}
                          style={{ borderTop: '1px dashed var(--hf-rule)' }}
                        >
                          <td style={{ padding: '8px 8px' }}>
                            <span className='strong'>
                              {s.auth_method || 'session'}
                            </span>
                          </td>
                          <td style={{ padding: '8px 8px' }}>
                            {s.current && (
                              <span className='tag ok'>current</span>
                            )}
                          </td>
                          <td
                            style={{ padding: '8px 8px', textAlign: 'right' }}
                          >
                            {s.active_tokens ?? 0}
                          </td>
                          <td
                            style={{ padding: '8px 8px', textAlign: 'right' }}
                          >
                            {(s.request_count ?? 0).toLocaleString()}
                          </td>
                          <td
                            className='faint'
                            style={{ padding: '8px 8px', textAlign: 'right' }}
                          >
                            {formatRelativeTime(s.last_seen)}
                          </td>
                          <td
                            style={{ padding: '8px 8px', textAlign: 'right' }}
                          >
                            <button
                              type='button'
                              className='btn sm'
                              data-testid='revoke-session-btn'
                              onClick={() => setRevokeVisible(true)}
                            >
                              revoke
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>

              {/* Revoke current session confirm dialog */}
              <ConfirmDialog
                visible={revokeVisible}
                title='撤销当前会话'
                consequenceList={[
                  '当前浏览器会话将立即失效。',
                  '你将被重定向到登录页，需要重新登录。',
                ]}
                confirmText='revoke'
                confirmButtonText='撤销会话'
                confirmButtonType='danger'
                onConfirm={handleRevokeSession}
                onCancel={() => setRevokeVisible(false)}
              />
            </div>
          )}

          {/* ── Subscription (Wave A Squad 5A — read-only) ── */}
          {section === 'subscription' && (
            <div style={{ marginTop: 22 }} data-testid='subscription-section'>
              {subLoading && (
                <div className='muted' style={{ fontSize: 12 }}>
                  Loading…
                </div>
              )}

              {!subLoading && subError && (
                <div
                  className='muted'
                  style={{ fontSize: 12 }}
                  data-testid='subscription-error'
                >
                  Failed to load subscription.
                </div>
              )}

              {!subLoading && !subError && subData && (
                <>
                  {(() => {
                    const ent =
                      ENTITLEMENT_BY_GROUP[subData.group] ??
                      ENTITLEMENT_BY_GROUP.default;
                    const tierName = subData.planHint || ent.label;
                    const isPlaceholder = subData.source === 'placeholder';

                    return (
                      <>
                        <div
                          className='panel'
                          style={{ padding: 18, marginBottom: 14 }}
                        >
                          <div className='lbl'>current plan</div>
                          <div
                            style={{
                              display: 'flex',
                              alignItems: 'center',
                              gap: 10,
                              marginTop: 10,
                            }}
                          >
                            <span
                              className='tag ok'
                              data-testid='subscription-tier-badge'
                              style={{
                                fontFamily: 'var(--hf-mono)',
                                fontSize: 12,
                                padding: '3px 10px',
                              }}
                            >
                              {tierName}
                            </span>
                            {isPlaceholder && (
                              <span
                                className='faint mono'
                                style={{ fontSize: 11 }}
                              >
                                Free tier — entitlement API not yet wired
                              </span>
                            )}
                            <span style={{ flex: 1 }} />
                            <button
                              type='button'
                              className='btn sm'
                              disabled
                              data-testid='subscription-upgrade-btn'
                              title='Plan upgrades available in Wave B'
                            >
                              Upgrade plan
                            </button>
                          </div>
                        </div>

                        <div className='panel'>
                          {[
                            ['routing modes', ent.routing],
                            ['SLA tier', ent.sla],
                            ['audit retention', `${ent.auditDays} days`],
                            ['support tier', ent.support],
                          ].map(([k, v], i, a) => (
                            <div
                              key={k}
                              style={{
                                display: 'grid',
                                gridTemplateColumns: '180px 1fr',
                                padding: '12px 16px',
                                borderBottom:
                                  i < a.length - 1
                                    ? '1px dashed var(--hf-rule)'
                                    : 0,
                                alignItems: 'center',
                              }}
                            >
                              <span className='lbl'>{k}</span>
                              <span className='strong' style={{ fontSize: 13 }}>
                                {v}
                              </span>
                            </div>
                          ))}
                        </div>
                      </>
                    );
                  })()}
                </>
              )}
            </div>
          )}

          {/* ── Billing (Wave A Squad 5A — read-only) ── */}
          {section === 'billing' && (
            <div style={{ marginTop: 22 }} data-testid='billing-section'>
              {billLoading && (
                <div className='muted' style={{ fontSize: 12 }}>
                  Loading…
                </div>
              )}

              {!billLoading && billError && (
                <div
                  className='muted'
                  style={{ fontSize: 12 }}
                  data-testid='billing-error'
                >
                  Billing data — backend wiring pending Wave B
                </div>
              )}

              {!billLoading && !billError && (
                <>
                  <div
                    className='panel'
                    style={{
                      padding: 18,
                      marginBottom: 14,
                      display: 'flex',
                      gap: 32,
                    }}
                  >
                    <div>
                      <div className='lbl' style={{ marginBottom: 4 }}>
                        wallet balance
                      </div>
                      <div
                        className='display'
                        style={{ fontSize: 22 }}
                        data-testid='billing-balance'
                      >
                        {billSummary?.balance != null
                          ? fmtCNY(billSummary.balance)
                          : billSummary?.wallet_balance_cny != null
                            ? fmtCNY(billSummary.wallet_balance_cny)
                            : '—'}
                      </div>
                    </div>
                    <div>
                      <div className='lbl' style={{ marginBottom: 4 }}>
                        last 30d consumption
                      </div>
                      <div
                        className='display'
                        style={{ fontSize: 22 }}
                        data-testid='billing-30d'
                      >
                        {billSummary?.mtd_spend_cny != null
                          ? fmtCNY(billSummary.mtd_spend_cny)
                          : billSummary?.lifetime_spend != null
                            ? fmtCNY(billSummary.lifetime_spend)
                            : '—'}
                      </div>
                    </div>
                  </div>

                  <div className='panel'>
                    <div
                      style={{
                        padding: '12px 16px',
                        borderBottom: '1px solid var(--hf-rule)',
                        display: 'flex',
                        alignItems: 'center',
                      }}
                    >
                      <div className='lbl'>recent transactions</div>
                      <span style={{ flex: 1 }} />
                      <button
                        type='button'
                        className='btn ghost sm'
                        disabled
                        data-testid='billing-view-full-btn'
                        title='Full billing history available in Wave C (ClickHouse insights)'
                      >
                        View full history
                      </button>
                    </div>

                    {billTxns.length === 0 ? (
                      <div
                        style={{
                          padding: 22,
                          textAlign: 'center',
                          color: 'var(--hf-ink-3)',
                          fontFamily: 'var(--hf-mono)',
                          fontSize: 12,
                        }}
                        data-testid='billing-empty-state'
                      >
                        No recent transactions.
                      </div>
                    ) : (
                      <table
                        data-testid='billing-txn-table'
                        style={{
                          width: '100%',
                          borderCollapse: 'collapse',
                          fontFamily: 'var(--hf-mono)',
                          fontSize: 11,
                        }}
                      >
                        <thead>
                          <tr style={{ color: 'var(--hf-ink-3)' }}>
                            <th
                              style={{
                                textAlign: 'left',
                                padding: '6px 12px',
                                fontWeight: 500,
                              }}
                            >
                              date
                            </th>
                            <th
                              style={{
                                textAlign: 'left',
                                padding: '6px 12px',
                                fontWeight: 500,
                              }}
                            >
                              type
                            </th>
                            <th
                              style={{
                                textAlign: 'right',
                                padding: '6px 12px',
                                fontWeight: 500,
                              }}
                            >
                              amount
                            </th>
                            <th
                              style={{
                                textAlign: 'right',
                                padding: '6px 12px',
                                fontWeight: 500,
                              }}
                            >
                              status
                            </th>
                          </tr>
                        </thead>
                        <tbody>
                          {billTxns.slice(0, 5).map((t, i) => (
                            <tr
                              key={t.id ?? i}
                              style={{ borderTop: '1px dashed var(--hf-rule)' }}
                            >
                              <td style={{ padding: '8px 12px' }}>
                                {t.created_at
                                  ? formatRelativeTime(t.created_at)
                                  : '—'}
                              </td>
                              <td style={{ padding: '8px 12px' }}>topup</td>
                              <td
                                style={{
                                  padding: '8px 12px',
                                  textAlign: 'right',
                                }}
                              >
                                {typeof t.quota === 'number'
                                  ? `${(t.quota / QUOTA_PER_USD).toFixed(2)} USD eq.`
                                  : '—'}
                              </td>
                              <td
                                style={{
                                  padding: '8px 12px',
                                  textAlign: 'right',
                                }}
                              >
                                <span className='tag ok'>settled</span>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    )}
                  </div>
                </>
              )}
            </div>
          )}

          {/* ── Notifications (Wave A Squad 5A — read-only upgrade) ── */}
          {section === 'notifications' && (
            <div style={{ marginTop: 22 }} data-testid='notifications-section'>
              <WIPBanner
                reason='Notification subscription store, dispatch path, and threshold rules not yet implemented. Designed in adr-2026-05-18-budget-alerts.md.'
                todo='Backend: notification_subscription table + /api/v2/{slug}/notifications/subscriptions + Prometheus rule pack.'
              />
              <div className='panel' style={{ marginTop: 14 }}>
                {NOTIFICATION_CHANNELS.map((ch, i, a) => (
                  <div
                    key={ch.key}
                    style={{
                      padding: '14px 16px',
                      borderBottom:
                        i < a.length - 1 ? '1px dashed var(--hf-rule)' : 0,
                      display: 'grid',
                      gridTemplateColumns: '1fr auto',
                      alignItems: 'center',
                      gap: 16,
                    }}
                  >
                    <div>
                      <div className='strong' style={{ fontSize: 13 }}>
                        {ch.label}
                      </div>
                      <div
                        className='faint mono'
                        style={{ fontSize: 10, marginTop: 4 }}
                      >
                        {ch.events.join(' · ')}
                      </div>
                    </div>
                    <button
                      type='button'
                      className='btn sm'
                      disabled
                      data-testid={`notif-toggle-${ch.key}`}
                      title='Notification preferences editable in Wave B'
                    >
                      off
                    </button>
                  </div>
                ))}
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
                      <button
                        type='button'
                        className='btn sm'
                        disabled
                        title='integration registry deferred to v3'
                      >
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
                  data-testid='danger-delete-btn'
                  title='account deletion requires data-export + cascade-purge design — deferred to v3'
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
