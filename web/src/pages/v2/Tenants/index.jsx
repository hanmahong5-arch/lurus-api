import React, { useCallback, useEffect, useRef, useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';
import { API, showError, showSuccess } from '../../../helpers';
import CreditPoolDrawer from './CreditPoolDrawer';

/* HiFi 9 — Tenants admin. Wired to /api/v2/admin/tenants (2026-05-11). */

const QUOTA_PER_USD = 500_000;

const quotaToUSD = (q) => (q / QUOTA_PER_USD).toFixed(2);

/** Status 1=active, 2=disabled, 3=suspended */
const tenantStatusLabel = (s) => {
  if (s === 1) return 'active';
  if (s === 2) return 'disabled';
  if (s === 3) return 'suspended';
  return 'unknown';
};

const tenantStatusClass = (s) => {
  if (s === 1) return 'tag ok';
  if (s === 3) return 'tag warn';
  return 'tag';
};

const planTagClass = (plan) => {
  if (plan === 'enterprise') return 'tag acc';
  if (plan === 'free') return 'tag';
  return 'tag info';
};

// ─── Create tenant modal ──────────────────────────────────────────────────────

const PLANS = ['free', 'startup', 'team', 'enterprise'];

const CreateModal = ({ onCreated, onClose }) => {
  const [form, setForm] = useState({
    name: '',
    slug: '',
    plan: 'free',
    quota_limit: '',
  });
  const [saving, setSaving] = useState(false);
  const nameRef = useRef(null);

  useEffect(() => {
    nameRef.current?.focus();
  }, []);

  const submit = async (e) => {
    e.preventDefault();
    if (!form.name.trim() || !form.slug.trim()) return;
    setSaving(true);
    try {
      const body = {
        name: form.name.trim(),
        slug: form.slug.trim(),
        plan: form.plan,
        quota_limit: form.quota_limit ? Math.round(parseFloat(form.quota_limit) * QUOTA_PER_USD) : 0,
        status: 1,
      };
      const res = await API.post('/api/v2/admin/tenants', body);
      if (res?.data?.success) {
        showSuccess('Tenant created');
        onCreated();
      }
    } catch (_) {
      // error toast shown by API interceptor
    } finally {
      setSaving(false);
    }
  };

  const inputStyle = {
    fontFamily: 'var(--hf-mono)',
    fontSize: 12,
    padding: '5px 8px',
    border: '1px solid var(--hf-rule)',
    background: 'var(--hf-sunken)',
    color: 'var(--hf-ink)',
    borderRadius: 2,
    outline: 'none',
    width: '100%',
  };

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.45)',
        zIndex: 500,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
      onClick={(e) => e.target === e.currentTarget && onClose()}
    >
      <form
        onSubmit={submit}
        style={{
          background: 'var(--hf-paper)',
          border: '1px solid var(--hf-rule)',
          borderRadius: 4,
          padding: 28,
          width: 420,
          display: 'flex',
          flexDirection: 'column',
          gap: 14,
        }}
      >
        <div className='strong' style={{ fontSize: 15 }}>New tenant</div>

        <label style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
          <span className='lbl'>name *</span>
          <input
            ref={nameRef}
            style={inputStyle}
            placeholder='e.g. Acme Corp'
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            required
          />
        </label>

        <label style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
          <span className='lbl'>slug *</span>
          <input
            style={inputStyle}
            placeholder='e.g. acme'
            value={form.slug}
            onChange={(e) => setForm((f) => ({ ...f, slug: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-') }))}
            required
          />
        </label>

        <label style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
          <span className='lbl'>plan</span>
          <select
            style={{ ...inputStyle, cursor: 'pointer' }}
            value={form.plan}
            onChange={(e) => setForm((f) => ({ ...f, plan: e.target.value }))}
          >
            {PLANS.map((p) => (
              <option key={p} value={p}>{p}</option>
            ))}
          </select>
        </label>

        <label style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
          <span className='lbl'>quota limit ($, 0 = unlimited)</span>
          <input
            style={inputStyle}
            type='number'
            min='0'
            step='0.01'
            placeholder='0'
            value={form.quota_limit}
            onChange={(e) => setForm((f) => ({ ...f, quota_limit: e.target.value }))}
          />
        </label>

        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 4 }}>
          <button type='button' className='btn ghost' onClick={onClose}>
            cancel
          </button>
          <button type='submit' className='btn primary' disabled={saving}>
            {saving ? 'creating…' : 'create tenant'}
          </button>
        </div>
      </form>
    </div>
  );
};

// ─── Stats drawer ─────────────────────────────────────────────────────────────

const StatsDrawer = ({ tenant, onClose }) => {
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await API.get(`/api/v2/admin/tenants/${tenant.Id}/stats`);
        if (!cancelled && res?.data?.success) {
          setStats(res.data.data);
        }
      } catch (_) {
        // silently handled
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [tenant.Id]);

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.45)',
        zIndex: 500,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
      onClick={(e) => e.target === e.currentTarget && onClose()}
    >
      <div
        style={{
          background: 'var(--hf-paper)',
          border: '1px solid var(--hf-rule)',
          borderRadius: 4,
          padding: 28,
          width: 460,
          display: 'flex',
          flexDirection: 'column',
          gap: 14,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div className='strong' style={{ fontSize: 15 }}>{tenant.Name} · stats</div>
          <button type='button' className='btn ghost sm' onClick={onClose}>✕</button>
        </div>

        {loading && <div className='muted' style={{ fontSize: 12 }}>Loading…</div>}

        {!loading && !stats && (
          <div className='muted' style={{ fontSize: 12 }}>No stats available.</div>
        )}

        {!loading && stats && (
          <div className='panel'>
            {Object.entries(stats).map(([k, v], i, arr) => (
              <div
                key={k}
                style={{
                  display: 'grid',
                  gridTemplateColumns: '160px 1fr',
                  padding: '10px 16px',
                  borderBottom: i < arr.length - 1 ? '1px dashed var(--hf-rule)' : 0,
                  fontSize: 12,
                  alignItems: 'center',
                }}
              >
                <span className='lbl'>{k}</span>
                <span className='mono strong'>{String(v)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

// ─── Main page ────────────────────────────────────────────────────────────────

const HFTenants = () => {
  const [tenants, setTenants] = useState([]);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [creating, setCreating] = useState(false);
  const [statsTarget, setStatsTarget] = useState(null); // tenant object for stats drawer
  const [poolTarget, setPoolTarget] = useState(null);   // tenant object for credit-pool drawer
  const [actioning, setActioning] = useState(null); // tenant id being actioned

  const searchRef = useRef(null);

  const fetchTenants = useCallback(async (kw = '') => {
    setLoading(true);
    setForbidden(false);
    try {
      const params = new URLSearchParams({ page: 1, page_size: 50 });
      if (kw.trim()) params.set('keyword', kw.trim());
      const res = await API.get(`/api/v2/admin/tenants?${params}`);
      if (res?.data?.success) {
        setTenants(res.data.data.items ?? []);
      }
    } catch (err) {
      if (err?.response?.status === 403) {
        setForbidden(true);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchTenants();
  }, [fetchTenants]);

  // Debounced keyword search
  useEffect(() => {
    const t = setTimeout(() => fetchTenants(keyword), 350);
    return () => clearTimeout(t);
  }, [keyword, fetchTenants]);

  const handleAction = async (tenant, action) => {
    const label = { enable: 'enable', disable: 'disable', suspend: 'suspend' }[action];
    if (!window.confirm(`${label} tenant "${tenant.Name}"?`)) return;
    setActioning(tenant.Id);
    try {
      const res = await API.post(`/api/v2/admin/tenants/${tenant.Id}/${action}`, {});
      if (res?.data?.success) {
        showSuccess(`Tenant ${label}d`);
        await fetchTenants(keyword);
      }
    } catch (_) {
      // error toast from interceptor
    } finally {
      setActioning(null);
    }
  };

  const handleCreated = async () => {
    setCreating(false);
    await fetchTenants(keyword);
  };

  const totalUsedUSD = tenants.reduce((s, t) => s + (t.UsedQuota || 0) / QUOTA_PER_USD, 0).toFixed(2);

  return (
    <HFShell
      active='users'
      crumbs={['platform · admin', 'tenants']}
      actions={
        <>
          {loading ? (
            <span className='muted mono' style={{ fontSize: 11 }}>loading…</span>
          ) : (
            !forbidden && (
              <span className='muted mono' style={{ fontSize: 11 }}>
                {tenants.length} tenant{tenants.length !== 1 ? 's' : ''} · ${totalUsedUSD} used
              </span>
            )
          )}
          <button
            type='button'
            className='btn primary'
            onClick={() => setCreating(true)}
          >
            + new tenant
          </button>
        </>
      }
    >
      <div className='hf-page-head'>
        <div>
          <div className='lbl' style={{ marginBottom: 6 }}>tenants</div>
          <h1>
            {loading ? '…' : forbidden ? 'Admin access required' : `${tenants.length} tenant${tenants.length !== 1 ? 's' : ''}`}
            {!loading && !forbidden && parseFloat(totalUsedUSD) > 0 && (
              <span className='muted' style={{ fontWeight: 400 }}>
                {' '}· ${totalUsedUSD} used
              </span>
            )}
          </h1>
          <div className='sub'>isolation · per-tenant keys · per-tenant budgets</div>
        </div>
      </div>

      {forbidden ? (
        <div style={{ padding: 24 }}>
          <div className='panel' style={{ padding: '20px 24px' }}>
            <div className='strong' style={{ marginBottom: 6 }}>Admin access required</div>
            <div className='muted' style={{ fontSize: 12 }}>
              You do not have permission to manage tenants. Contact a platform administrator.
            </div>
          </div>
        </div>
      ) : (
        <div style={{ padding: 24 }}>
          {/* Search bar */}
          <div style={{ marginBottom: 16 }}>
            <input
              ref={searchRef}
              style={{
                fontFamily: 'var(--hf-mono)',
                fontSize: 12,
                padding: '6px 10px',
                border: '1px solid var(--hf-rule)',
                background: 'var(--hf-sunken)',
                color: 'var(--hf-ink)',
                borderRadius: 2,
                outline: 'none',
                width: 260,
              }}
              placeholder='search tenants…'
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
            />
          </div>

          <div className='panel'>
            {loading ? (
              <div className='muted' style={{ padding: '20px 24px', fontSize: 12 }}>Loading…</div>
            ) : tenants.length === 0 ? (
              <div className='muted' style={{ padding: '20px 24px', fontSize: 12 }}>
                {keyword ? 'No tenants match your search.' : 'No tenants yet. Create one to get started.'}
              </div>
            ) : (
              <table className='t'>
                <thead>
                  <tr>
                    <th>tenant</th>
                    <th>plan</th>
                    <th>status</th>
                    <th>users</th>
                    <th>used</th>
                    <th>quota cap</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {tenants.map((t) => {
                    const usedUSD = parseFloat(quotaToUSD(t.UsedQuota || 0));
                    const capUSD = t.QuotaLimit > 0 ? parseFloat(quotaToUSD(t.QuotaLimit)) : null;
                    const pct = capUSD ? Math.min(usedUSD / capUSD, 1) : 0;
                    const isActioning = actioning === t.Id;

                    return (
                      <tr key={t.Id}>
                        <td>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
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
                                flexShrink: 0,
                              }}
                            >
                              {(t.Name || t.Slug || '?')[0].toUpperCase()}
                            </div>
                            <div>
                              <div className='strong'>{t.Name}</div>
                              <div className='faint mono' style={{ fontSize: 10 }}>{t.Slug}</div>
                            </div>
                          </div>
                        </td>

                        <td>
                          <span className={planTagClass(t.Plan)}>{t.Plan || '—'}</span>
                        </td>

                        <td>
                          <span className={tenantStatusClass(t.Status)}>
                            {tenantStatusLabel(t.Status)}
                          </span>
                        </td>

                        <td className='mono'>{t.UserCount ?? '—'}</td>

                        <td>
                          <span className='display' style={{ fontSize: 15 }}>
                            ${quotaToUSD(t.UsedQuota || 0)}
                          </span>
                        </td>

                        <td>
                          {capUSD ? (
                            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                              <div style={{ width: 80, height: 4, background: 'var(--hf-sunken)', flexShrink: 0 }}>
                                <div
                                  style={{
                                    width: `${pct * 100}%`,
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
                              <span className='mono muted' style={{ fontSize: 10 }}>
                                ${quotaToUSD(t.QuotaLimit)}
                              </span>
                            </div>
                          ) : (
                            <span className='mono muted' style={{ fontSize: 11 }}>∞</span>
                          )}
                        </td>

                        <td>
                          <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                            <button
                              type='button'
                              className='btn ghost sm'
                              disabled={isActioning}
                              onClick={() => setStatsTarget(t)}
                            >
                              stats
                            </button>
                            <button
                              type='button'
                              className='btn ghost sm'
                              disabled={isActioning}
                              onClick={() => setPoolTarget(t)}
                            >
                              pool
                            </button>
                            {t.Status !== 1 && (
                              <button
                                type='button'
                                className='btn ghost sm'
                                disabled={isActioning}
                                onClick={() => handleAction(t, 'enable')}
                              >
                                {isActioning ? '…' : 'enable'}
                              </button>
                            )}
                            {t.Status === 1 && (
                              <button
                                type='button'
                                className='btn ghost sm'
                                disabled={isActioning}
                                onClick={() => handleAction(t, 'disable')}
                              >
                                {isActioning ? '…' : 'disable'}
                              </button>
                            )}
                            {t.Status !== 3 && (
                              <button
                                type='button'
                                className='btn ghost sm'
                                disabled={isActioning}
                                style={{ color: 'var(--hf-warn)' }}
                                onClick={() => handleAction(t, 'suspend')}
                              >
                                {isActioning ? '…' : 'suspend'}
                              </button>
                            )}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}

      {creating && (
        <CreateModal onCreated={handleCreated} onClose={() => setCreating(false)} />
      )}

      {statsTarget && (
        <StatsDrawer tenant={statsTarget} onClose={() => setStatsTarget(null)} />
      )}

      {poolTarget && (
        <CreditPoolDrawer
          tenantId={poolTarget.Id}
          tenantName={poolTarget.Name}
          onClose={() => setPoolTarget(null)}
        />
      )}
    </HFShell>
  );
};

export default HFTenants;
