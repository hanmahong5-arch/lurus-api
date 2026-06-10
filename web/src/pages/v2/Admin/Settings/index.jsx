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
import HFShell from '../../../../components/hifi/HFShell';
import NotAvailable from '../../../../components/hifi/NotAvailable';
import { API, showSuccess } from '../../../../helpers';

/*
 * v2 admin — system settings panels. Wired to /api/v2/admin/options
 * (GET filters secret-suffixed keys; PUT delegates per-key validation + audit).
 * This is a curated, high-value SUBSET — NOT New-API's full 2000-key page. The
 * long tail (ratio JSON, advanced relay tuning) is honestly noted as managed in
 * v1 / advanced rather than half-surfaced here. Each write is one key per call.
 */

// Field types: text | textarea | number | toggle. Each panel curates a subset.
const PANELS = [
  {
    id: 'general',
    label: 'General',
    fields: [
      { key: 'SystemName', label: 'System name', type: 'text' },
      { key: 'Footer', label: 'Footer', type: 'text' },
      { key: 'Notice', label: 'Notice', type: 'textarea' },
      { key: 'About', label: 'About', type: 'textarea' },
    ],
  },
  {
    id: 'branding',
    label: 'Branding',
    fields: [
      { key: 'Logo', label: 'Logo URL', type: 'text' },
      { key: 'HomePageContent', label: 'Home page content', type: 'textarea' },
      {
        key: 'console_setting.announcements',
        label: 'Announcements (JSON — validated)',
        type: 'textarea',
      },
      {
        key: 'console_setting.faq',
        label: 'FAQ (JSON — validated)',
        type: 'textarea',
      },
    ],
  },
  {
    id: 'auth',
    label: 'Auth',
    fields: [
      { key: 'PasswordLoginEnabled', label: 'Password login', type: 'toggle' },
      {
        key: 'PasswordRegisterEnabled',
        label: 'Password registration',
        type: 'toggle',
      },
      { key: 'GitHubOAuthEnabled', label: 'GitHub OAuth', type: 'toggle' },
      { key: 'TelegramOAuthEnabled', label: 'Telegram OAuth', type: 'toggle' },
      { key: 'WeChatAuthEnabled', label: 'WeChat login', type: 'toggle' },
      {
        key: 'TurnstileCheckEnabled',
        label: 'Turnstile check',
        type: 'toggle',
      },
    ],
  },
  {
    id: 'security',
    label: 'Security',
    fields: [
      {
        key: 'SensitiveActionRequire2FA',
        label: 'Require 2FA for sensitive actions',
        type: 'toggle',
      },
      {
        key: 'SessionTimeoutMinutes',
        label: 'Session timeout (minutes)',
        type: 'number',
      },
      {
        key: 'EmailDomainRestrictionEnabled',
        label: 'Email domain restriction',
        type: 'toggle',
      },
    ],
  },
  {
    id: 'quota',
    label: 'Quota',
    fields: [
      { key: 'QuotaForNewUser', label: 'Quota for new user', type: 'number' },
      { key: 'QuotaForInviter', label: 'Quota for inviter', type: 'number' },
      { key: 'USDExchangeRate', label: 'USD exchange rate', type: 'number' },
    ],
    // Ratio JSON editors (ModelRatio/GroupRatio) are large and validated
    // server-side — deferred here with an honest note rather than half-built.
    note: 'Model/Group ratio JSON editors are large and live in v1 / advanced — not surfaced here. Validation lives server-side.',
  },
  { id: 'monitoring', label: 'Monitoring', readOnly: true },
];

const inputStyle = {
  fontFamily: 'var(--hf-mono)',
  fontSize: 12,
  padding: '6px 8px',
  border: '1px solid var(--hf-rule)',
  background: 'var(--hf-sunken)',
  color: 'var(--hf-ink)',
  borderRadius: 2,
  outline: 'none',
  width: '100%',
};

const HFAdminSettings = () => {
  const [options, setOptions] = useState({});
  const [drafts, setDrafts] = useState({});
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [panel, setPanel] = useState('general');
  const [savingKey, setSavingKey] = useState(null);
  const [errors, setErrors] = useState({}); // key → message
  const [stats, setStats] = useState(null);
  const [statsLoading, setStatsLoading] = useState(false);

  const fetchOptions = useCallback(async () => {
    setLoading(true);
    setForbidden(false);
    try {
      const res = await API.get('/api/v2/admin/options', {
        skipErrorHandler: true,
      });
      if (res?.data?.success) {
        const map = {};
        (res.data.data ?? []).forEach((o) => {
          map[o.key] = o.value;
        });
        setOptions(map);
        setDrafts(map);
      }
    } catch (err) {
      if (err?.response?.status === 403) setForbidden(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchOptions();
  }, [fetchOptions]);

  // Monitoring is read-only — pull the system stats already wired at /admin/stats.
  useEffect(() => {
    if (panel !== 'monitoring') return;
    let cancelled = false;
    setStatsLoading(true);
    API.get('/api/v2/admin/stats', { skipErrorHandler: true })
      .then((res) => {
        if (!cancelled && res?.data?.success) setStats(res.data.data);
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setStatsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [panel]);

  // Persist one key per call. Surfaces the backend's honest validation-failure
  // message inline (UpdateOption returns 200 + success:false for those).
  const putOption = useCallback(async (key, value) => {
    setSavingKey(key);
    setErrors((prev) => ({ ...prev, [key]: undefined }));
    try {
      const res = await API.put(
        '/api/v2/admin/options',
        { key, value },
        { skipErrorHandler: true },
      );
      if (res?.data?.success) {
        setOptions((prev) => ({ ...prev, [key]: String(value) }));
        showSuccess('Saved');
        return true;
      }
      setErrors((prev) => ({
        ...prev,
        [key]: res?.data?.message || 'Update rejected',
      }));
      return false;
    } catch (err) {
      setErrors((prev) => ({
        ...prev,
        [key]: err?.response?.data?.message || 'Update failed',
      }));
      return false;
    } finally {
      setSavingKey(null);
    }
  }, []);

  const isOn = (key) => options[key] === 'true';
  const dirty = (key) => (drafts[key] ?? '') !== (options[key] ?? '');

  const renderField = (f) => {
    const err = errors[f.key];
    const saving = savingKey === f.key;

    if (f.type === 'toggle') {
      return (
        <div
          key={f.key}
          style={{ marginBottom: 16 }}
          data-testid={`field-${f.key}`}
        >
          <label
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 10,
              cursor: 'pointer',
            }}
          >
            <input
              type='checkbox'
              data-testid={`toggle-${f.key}`}
              checked={isOn(f.key)}
              disabled={saving}
              onChange={() => putOption(f.key, !isOn(f.key))}
            />
            <span className='strong' style={{ fontSize: 13 }}>
              {f.label}
            </span>
          </label>
          {err && (
            <div
              data-testid={`error-${f.key}`}
              style={{ color: 'var(--hf-err)', fontSize: 11, marginTop: 4 }}
            >
              {err}
            </div>
          )}
        </div>
      );
    }

    const commonProps = {
      'data-testid': `field-${f.key}`,
      style: inputStyle,
      value: drafts[f.key] ?? '',
      onChange: (e) =>
        setDrafts((prev) => ({ ...prev, [f.key]: e.target.value })),
    };

    return (
      <div key={f.key} style={{ marginBottom: 18 }}>
        <div className='lbl' style={{ marginBottom: 5 }}>
          {f.label}
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
          {f.type === 'textarea' ? (
            <textarea
              rows={3}
              {...commonProps}
              style={{ ...inputStyle, resize: 'vertical' }}
            />
          ) : (
            <input
              type={f.type === 'number' ? 'number' : 'text'}
              {...commonProps}
            />
          )}
          <button
            type='button'
            className='btn primary sm'
            data-testid={`save-${f.key}`}
            disabled={saving || !dirty(f.key)}
            onClick={() => putOption(f.key, drafts[f.key] ?? '')}
          >
            {saving ? 'saving…' : 'save'}
          </button>
        </div>
        {err && (
          <div
            data-testid={`error-${f.key}`}
            style={{ color: 'var(--hf-err)', fontSize: 11, marginTop: 4 }}
          >
            {err}
          </div>
        )}
      </div>
    );
  };

  const activePanel = PANELS.find((p) => p.id === panel) || PANELS[0];

  return (
    <HFShell active='admin-settings' crumbs={['platform · admin', 'settings']}>
      <div className='hf-page-head'>
        <div>
          <div className='lbl' style={{ marginBottom: 6 }}>
            admin settings
          </div>
          <h1>{forbidden ? 'Admin access required' : 'System settings'}</h1>
          <div className='sub'>
            curated, high-value subset · one key per save
          </div>
        </div>
      </div>

      {forbidden ? (
        <div style={{ padding: 24 }}>
          <div className='panel' style={{ padding: '20px 24px' }}>
            <div className='strong' style={{ marginBottom: 6 }}>
              Admin access required
            </div>
            <div className='muted' style={{ fontSize: 12 }}>
              You do not have permission to manage system settings. Contact a
              platform administrator.
            </div>
          </div>
        </div>
      ) : (
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '180px 1fr',
            minHeight: 0,
            height: '100%',
          }}
        >
          {/* Left nav */}
          <div
            style={{
              borderRight: '1px solid var(--hf-rule)',
              padding: '16px 0',
              background: 'var(--hf-paper)',
            }}
          >
            {PANELS.map((p) => (
              <button
                key={p.id}
                type='button'
                data-testid={`panel-${p.id}`}
                onClick={() => setPanel(p.id)}
                style={{
                  display: 'block',
                  width: '100%',
                  textAlign: 'left',
                  padding: '8px 20px',
                  border: 0,
                  background: panel === p.id ? 'var(--hf-elev)' : 'transparent',
                  borderLeft:
                    panel === p.id
                      ? '2px solid var(--hf-accent)'
                      : '2px solid transparent',
                  color: panel === p.id ? 'var(--hf-ink)' : 'var(--hf-ink-3)',
                  cursor: 'pointer',
                  fontFamily: 'var(--hf-mono)',
                  fontSize: 12,
                }}
              >
                {p.label}
              </button>
            ))}
          </div>

          {/* Panel body */}
          <div style={{ overflow: 'auto', padding: 28 }}>
            {loading ? (
              <div className='muted' style={{ fontSize: 12 }}>
                Loading…
              </div>
            ) : activePanel.readOnly ? (
              <div>
                <div className='lbl' style={{ marginBottom: 10 }}>
                  system stats · read-only
                </div>
                {statsLoading ? (
                  <div className='muted' style={{ fontSize: 12 }}>
                    Loading…
                  </div>
                ) : stats ? (
                  <pre
                    data-testid='monitoring-stats'
                    className='mono'
                    style={{
                      margin: 0,
                      padding: 16,
                      fontSize: 11,
                      background: 'var(--hf-paper)',
                      border: '1px solid var(--hf-rule)',
                      color: 'var(--hf-ink-2)',
                      overflow: 'auto',
                    }}
                  >
                    {JSON.stringify(stats, null, 2)}
                  </pre>
                ) : (
                  <NotAvailable reason='stats endpoint returned no data' />
                )}
              </div>
            ) : (
              <div style={{ maxWidth: 560 }}>
                <div className='lbl' style={{ marginBottom: 16 }}>
                  {activePanel.label.toLowerCase()}
                </div>
                {activePanel.fields.map(renderField)}
                {activePanel.note && (
                  <div
                    className='muted'
                    style={{
                      fontSize: 11,
                      marginTop: 8,
                      paddingTop: 12,
                      borderTop: '1px dashed var(--hf-rule)',
                    }}
                  >
                    {activePanel.note}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      )}
    </HFShell>
  );
};

export default HFAdminSettings;
