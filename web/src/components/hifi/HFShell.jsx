import React, { useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import TenantSwitcher from './TenantSwitcher';

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

const HFShell = ({ active, crumbs = [], actions, children }) => {
  const autoActive = useV2ActiveId();
  const activeId = active != null ? active : autoActive;
  const [theme, toggleTheme] = useThemeToggle();
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
          <TenantSwitcher />
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
          </div>
        </div>
        <div className='hf-body'>{children}</div>
      </div>
    </div>
  );
};

export default HFShell;
