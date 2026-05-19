import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';

// Mock helpers BEFORE importing the component.
vi.mock('../../../helpers', () => ({
  API: {
    get: vi.fn(),
    put: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

// HFShell passthrough — isolates Settings from shell/router dependencies.
vi.mock('../../../components/hifi/HFShell', () => ({
  default: ({ children }) =>
    React.createElement('div', { 'data-testid': 'hf-shell' }, children),
}));

// WIPBanner passthrough — renders its reason so tests can assert its presence.
vi.mock('../../../components/hifi/WIPBanner', () => ({
  default: ({ reason }) =>
    React.createElement(
      'div',
      { 'data-testid': 'wip-banner' },
      reason ?? 'WIP',
    ),
}));

import HFSettings from './index';
import { API } from '../../../helpers';

const fakeProfile = {
  display_name: 'Alice',
  email: 'alice@test.local',
  username: 'alice',
  role: 'user',
  tenant_id: 'acme',
  used_quota: 0,
  request_count: 0,
};

const fakeSessionsResponse = (items) => ({
  data: {
    success: true,
    data: { items, total: items.length },
  },
});

beforeEach(() => {
  API.get.mockReset();
  API.put.mockReset();
  window.localStorage.clear();
  window.localStorage.setItem('tenant_slug', 'acme');

  // Default: profile fetch succeeds; sessions not fetched until security tab.
  API.get.mockImplementation((url) => {
    if (url.includes('/user/me')) {
      return Promise.resolve({ data: { success: true, data: fakeProfile } });
    }
    if (url.includes('/sessions')) {
      return Promise.resolve(
        fakeSessionsResponse([
          {
            id: 'current',
            current: true,
            auth_method: 'zitadel',
            active_tokens: 3,
            request_count: 42,
            last_seen: Math.floor(Date.now() / 1000) - 30,
          },
        ]),
      );
    }
    return Promise.resolve({ data: { success: false } });
  });
});

describe('Settings page', () => {
  // 1. Security section fetches sessions on mount (when security tab is active).
  //    We render with a controlled tab by clicking the "Security" nav item.
  it('fetches sessions on mount and renders auth_method', async () => {
    render(<HFSettings />);

    // Click the Security section in the left nav.
    const securityNav = screen.getByText('Security');
    securityNav.click();

    await waitFor(() => {
      // Sessions API must have been called with the correct slug.
      const calls = API.get.mock.calls.map((c) => c[0]);
      expect(calls.some((u) => u.includes('/acme/sessions'))).toBe(true);
    });

    await waitFor(() => {
      // auth_method value rendered in the sessions table.
      expect(screen.getByTestId('sessions-table').textContent).toContain(
        'zitadel',
      );
    });
  });

  // 2. Notifications section still has WIPBanner.
  it('shows WIPBanner on notifications section', async () => {
    render(<HFSettings />);

    const notifNav = screen.getByText('Notifications');
    notifNav.click();

    await waitFor(() => {
      const banners = screen.getAllByTestId('wip-banner');
      const texts = banners.map((b) => b.textContent);
      expect(texts.some((t) => /notification/i.test(t))).toBe(true);
    });
  });

  // 3. Team section still has WIPBanner.
  it('shows WIPBanner on team section', async () => {
    render(<HFSettings />);

    const teamNav = screen.getByText('Team & roles');
    teamNav.click();

    await waitFor(() => {
      const banners = screen.getAllByTestId('wip-banner');
      const texts = banners.map((b) => b.textContent);
      expect(texts.some((t) => /team/i.test(t))).toBe(true);
    });
  });
});
