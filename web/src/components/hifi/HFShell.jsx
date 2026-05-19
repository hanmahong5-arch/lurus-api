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
import React, { useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import TenantSwitcher from './TenantSwitcher';
import { API } from '../../helpers';

// Single source of truth: pathname suffix → nav item id.
// HFShell uses this to auto-highlight the active item when caller doesn't
// pass an `active` prop, so each page doesn't have to hardcode it.
const PATH_TO_ID = {
  dashboard: 'dashboard',
  playground: 'playground',
  chat: 'chat',
  token: 'tokens',
  log: 'logs',
  billing: 'billing',
  channel: 'channels',
  models: 'models',
  tenants: 'users',
  pricing: 'pricing',
  settings: 'settings',
  flows: 'channels',
  states: 'logs',
  variants: 'dashboard',
  cmdk: 'tokens',
  'design-system': 'settings',
};

const useV2ActiveId = () => {
  const { pathname } = useLocation();
  const m = pathname.match(/\/console\/v2\/([^/?#]+)/);
  return m ? PATH_TO_ID[m[1]] || '' : '';
};

const useThemeToggle = () => {
  const [theme, setTheme] = useState(() => {
    if (typeof document === 'undefined') return 'light';
    return document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light';
  });
  useEffect(() => {
    if (typeof document === 'undefined') return;
    document.documentElement.dataset.theme = theme;
    try {
      localStorage.setItem('lurus-hf-theme', theme);
    } catch (e) {
      // ignore — private browsing / disabled storage
    }
  }, [theme]);
  // On first mount, hydrate from storage if no theme yet set.
  useEffect(() => {
    try {
      const saved = localStorage.getItem('lurus-hf-theme');
      if (saved === 'light' || saved === 'dark') setTheme(saved);
    } catch (e) {
      // ignore
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return [theme, () => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))];
};

/*
 * HFShell — sidebar + topbar wrapper for hi-fi v2 screens.
 * Ported from design canvas hifi/_shell.jsx (2026-05-07).
 *
 * The hi-fi screens render their own chrome; PageLayout bypasses HeaderBar/SiderBar
 * for /console/v2/* paths (see PageLayout.jsx).
 */

const NAV_SECTIONS = [
  {
    h: 'workspace',
    items: [
      {
        id: 'dashboard',
        href: '/console/v2/dashboard',
        glyph: '▣',
        label: 'Dashboard',
        badge: '',
      },
      {
        id: 'playground',
        href: '/console/v2/playground',
        glyph: '◇',
        label: 'Playground',
        badge: '',
      },
      {
        id: 'chat',
        href: '/console/v2/chat',
        glyph: '◌',
        label: 'Chat',
        badge: '',
      },
    ],
  },
  {
    h: 'my account',
    items: [
      {
        id: 'tokens',
        href: '/console/v2/token',
        glyph: '⚿',
        label: 'Tokens',
        badge: '5',
      },
      {
        id: 'logs',
        href: '/console/v2/log',
        glyph: '≣',
        label: 'Usage & logs',
        badge: '',
      },
      {
        id: 'billing',
        href: '/console/v2/billing',
        glyph: '$',
        label: 'Billing',
        badge: '$241',
      },
    ],
  },
  {
    h: 'platform · admin',
    items: [
      {
        id: 'channels',
        href: '/console/v2/channel',
        glyph: '⏚',
        label: 'Channels',
        badge: '8',
      },
      {
        id: 'models',
        href: '/console/v2/models',
        glyph: '◧',
        label: 'Models',
        badge: '54',
      },
      {
        id: 'users',
        href: '/console/v2/tenants',
        glyph: '◍',
        label: 'Tenants',
        badge: '',
      },
      {
        id: 'pricing',
        href: '/console/v2/pricing',
        glyph: '▥',
        label: 'Pricing',
        badge: '',
      },
      {
        id: 'settings',
        href: '/console/v2/settings',
        glyph: '✱',
        label: 'Settings',
        badge: '',
      },
    ],
  },
];

// Pull the bridged user identity directly from localStorage — set by
// ZitadelRedirect/zita-bootstrap. The v2 hi-fi shell intentionally
// avoids the StatusContext/UserContext used by the legacy chrome (those
// pull a chunk of v1 state we don't need here); a thin localStorage
// read is enough to surface "who am I" + a logout escape hatch.
const useBridgedUser = () => {
  const [user, setUser] = useState(null);
  useEffect(() => {
    try {
      const raw = localStorage.getItem('user');
      if (raw) setUser(JSON.parse(raw));
    } catch (e) {
      // malformed payload — leave user null, the logout button still works
    }
  }, []);
  return user;
};

const readTenantSlug = () => {
  try {
    return localStorage.getItem('tenant_slug') || 'default';
  } catch (_) {
    return 'default';
  }
};

const inferModeFromRole = (role) => {
  // Map v1 role ints to TenantSwitcher mode buckets.
  // role 100 = root, 10 = admin → manage tenant pool; else Personal/EndUser.
  if (role === 100 || role === 10) return 'Reseller';
  return 'Personal';
};

// Real tenants + active tenant for TenantSwitcher.
// Admin (root) attempts /api/v2/admin/tenants; everyone falls back to a
// single-item list derived from the bridged user. Empty/error → still
// renders a non-DEMO list so users never see "acme · prod" placeholder.
const useRealTenants = (user) => {
  const [tenants, setTenants] = useState(null);

  useEffect(() => {
    if (!user) return;
    let cancelled = false;

    const slug = readTenantSlug();
    const fallbackName =
      user.tenant_name || user.display_name || user.username || slug;
    const fallback = [
      {
        id: slug,
        name: fallbackName,
        mode: inferModeFromRole(user.role),
      },
    ];

    // Only root (100) can list all tenants. Skip the call otherwise.
    if (user.role !== 100) {
      if (!cancelled) setTenants(fallback);
      return () => {
        cancelled = true;
      };
    }

    API.get('/api/v2/admin/tenants?page_size=50', { skipErrorHandler: true })
      .then((res) => {
        if (cancelled) return;
        const rows = res?.data?.data?.tenants;
        if (!Array.isArray(rows) || rows.length === 0) {
          setTenants(fallback);
          return;
        }
        setTenants(
          rows.map((t) => ({
            id: t.slug || t.id,
            name: t.name || t.slug || String(t.id),
            mode: 'Reseller',
          })),
        );
      })
      .catch(() => {
        if (!cancelled) setTenants(fallback);
      });

    return () => {
      cancelled = true;
    };
  }, [user]);

  return tenants;
};

const switchTenantSlug = (slug) => {
  try {
    localStorage.setItem('tenant_slug', slug);
  } catch (_) {
    // ignore — private mode
  }
  // Trigger a re-fetch of tenant-scoped data by reloading.
  window.location.reload();
};

const handleLogout = async () => {
  try {
    await API.post('/api/v2/auth/zita-logout');
  } catch (e) {
    // session may already be expired; the cookie clear still happened
  }
  try {
    localStorage.removeItem('user');
    localStorage.removeItem('tenant_slug');
  } catch (e) {
    // ignore — private mode / disabled storage
  }
  window.location.href = '/login';
};

const HFShell = ({ active, crumbs = [], actions, children }) => {
  const autoActive = useV2ActiveId();
  const activeId = active != null ? active : autoActive;
  const [theme, toggleTheme] = useThemeToggle();
  const user = useBridgedUser();
  const tenants = useRealTenants(user);
  const currentSlug = readTenantSlug();
  const currentTenant =
    (tenants && tenants.find((t) => t.id === currentSlug)) ||
    (tenants && tenants[0]) ||
    null;
  return (
    <div className='hf hf-shell'>
      <aside className='hf-side'>
        <div className='brand'>
          <div className='brand-mark' />
          <div>
            <div className='brand-name'>Lurus Hub</div>
            <div className='brand-tag'>data processing layer</div>
          </div>
        </div>

        <button
          type='button'
          className='btn'
          style={{
            width: '100%',
            justifyContent: 'space-between',
            marginBottom: 14,
          }}
        >
          <span style={{ color: 'var(--hf-ink-3)' }}>⌕ search anything</span>
          <span>
            <span className='kbd'>⌘</span>
            <span className='kbd'>K</span>
          </span>
        </button>

        {NAV_SECTIONS.map((s) => (
          <div className='nav-section' key={s.h}>
            <div className='nav-h'>{s.h}</div>
            {s.items.map((it) => {
              const className = 'nav-i' + (activeId === it.id ? ' active' : '');
              const inner = (
                <>
                  <span className='nav-glyph'>{it.glyph}</span>
                  <span>{it.label}</span>
                  {it.badge && <span className='nav-badge'>{it.badge}</span>}
                </>
              );
              return it.href ? (
                <Link key={it.id} to={it.href} className={className}>
                  {inner}
                </Link>
              ) : (
                <div key={it.id} className={className}>
                  {inner}
                </div>
              );
            })}
          </div>
        ))}

        <div className='footer'>
          <TenantSwitcher
            tenants={tenants ?? []}
            tenantName={currentTenant?.name ?? ''}
            mode={currentTenant?.mode ?? 'Personal'}
            onSelect={(tenantId) => {
              if (tenantId && tenantId !== currentSlug) {
                switchTenantSlug(tenantId);
              }
            }}
          />
        </div>
      </aside>

      <div className='hf-main'>
        <div className='hf-top'>
          <div className='crumb'>
            {crumbs.map((c, i) => (
              <React.Fragment key={i}>
                {i > 0 && <span className='faint'>/</span>}
                {i === crumbs.length - 1 ? <b>{c}</b> : <span>{c}</span>}
              </React.Fragment>
            ))}
          </div>
          <div style={{ flex: 1 }} />
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            {actions}
            <button
              type='button'
              className='btn ghost'
              onClick={toggleTheme}
              title={theme === 'dark' ? 'switch to light' : 'switch to dark'}
              aria-label='toggle theme'
              style={{ fontSize: 14, padding: '0 8px' }}
            >
              {theme === 'dark' ? '☀' : '◐'}
            </button>
            {user && (
              <span
                style={{
                  fontSize: 12,
                  color: 'var(--hf-ink-2)',
                  padding: '0 6px',
                  maxWidth: 160,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
                title={user.email || user.username}
              >
                {user.display_name || user.username}
              </span>
            )}
            <button
              type='button'
              className='btn ghost'
              onClick={
                user
                  ? handleLogout
                  : () => {
                      window.location.href = '/login';
                    }
              }
              title={user ? 'log out' : 'log in'}
              aria-label={user ? 'log out' : 'log in'}
              style={{ fontSize: 12, padding: '0 10px' }}
            >
              {user ? 'logout' : 'login'}
            </button>
          </div>
        </div>
        <div className='hf-body'>{children}</div>
      </div>
    </div>
  );
};

export default HFShell;
