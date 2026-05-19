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
import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';

// Mock helpers BEFORE importing the component — vi.mock is hoisted.
vi.mock('../../../helpers', () => ({
  API: {
    get: vi.fn(),
    post: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

// HFShell pulls TenantSwitcher → API helper chain → react-router. Stub to a
// passthrough wrapper so tests focus on Log UI/logic.
vi.mock('../../../components/hifi/HFShell', () => ({
  default: ({ children, actions }) =>
    React.createElement(
      'div',
      { 'data-testid': 'hf-shell' },
      React.createElement('div', null, actions),
      children,
    ),
}));

// WIPBanner stub: renders a div with a detectable attribute.
vi.mock('../../../components/hifi/WIPBanner', () => ({
  default: ({ reason }) =>
    React.createElement('div', { 'data-testid': 'wip-banner' }, reason),
}));

import HFLog from './index';
import { API, showError } from '../../../helpers';

beforeEach(() => {
  API.get.mockReset();
  API.post.mockReset();
  showError.mockReset();
  window.localStorage.clear();
  window.localStorage.setItem('tenant_slug', 'acme');

  // Default mock for Recent tab initial fetch (called on mount).
  API.get.mockResolvedValue({
    data: { success: true, data: { logs: [], total: 0 } },
  });
});

afterEach(() => {
  vi.useRealTimers();
});

describe('Log page', () => {
  // 1. Switching to Cluster tab triggers GET /api/v2/acme/logs/cluster?bucket=hour
  it('switches to Cluster tab and fetches with default bucket=hour', async () => {
    API.get.mockImplementation((url) => {
      if (url.includes('/logs/cluster')) {
        return Promise.resolve({
          data: {
            success: true,
            data: { items: [], total: 0, bucket: 'hour' },
          },
        });
      }
      // Recent tab initial fetch
      return Promise.resolve({
        data: { success: true, data: { logs: [], total: 0 } },
      });
    });

    render(<HFLog />);

    // Click the "Error clusters" tab button.
    fireEvent.click(screen.getByText('Error clusters'));

    await waitFor(() => {
      const calls = API.get.mock.calls.map(([url]) => url);
      const clusterCall = calls.find((u) => u.includes('/logs/cluster'));
      expect(clusterCall).toBeDefined();
      expect(clusterCall).toContain('bucket=hour');
    });
  });

  // 2. Cluster rows are rendered sorted by count descending.
  it('renders cluster rows sorted by count descending', async () => {
    const mockItems = [
      {
        model_name: 'gpt-4o',
        error_code: 'ERR',
        bucket: '2023-11-15 12:00',
        count: 10,
      },
      {
        model_name: 'claude',
        error_code: '',
        bucket: '2023-11-15 12:00',
        count: 50,
      },
      {
        model_name: 'gemini',
        error_code: '500',
        bucket: '2023-11-15 12:00',
        count: 5,
      },
    ];

    API.get.mockImplementation((url) => {
      if (url.includes('/logs/cluster')) {
        return Promise.resolve({
          data: {
            success: true,
            data: { items: mockItems, total: 3, bucket: 'hour' },
          },
        });
      }
      return Promise.resolve({
        data: { success: true, data: { logs: [], total: 0 } },
      });
    });

    render(<HFLog />);
    fireEvent.click(screen.getByText('Error clusters'));

    await waitFor(() => {
      expect(screen.getByTestId('cluster-table')).toBeDefined();
    });

    const rows = screen
      .getByTestId('cluster-table')
      .querySelectorAll('tbody tr');
    // Rows should be sorted by count DESC: 50, 10, 5
    // The count cells are the 4th td in each row.
    const counts = Array.from(rows).map((r) =>
      Number(r.querySelectorAll('td')[3].textContent),
    );
    expect(counts[0]).toBeGreaterThanOrEqual(counts[1]);
    expect(counts[1]).toBeGreaterThanOrEqual(counts[2]);
    // First row must be the highest count (claude / 50).
    expect(counts[0]).toBe(50);
  });

  // 3. Clicking Live tail tab shows WIPBanner (SSE deferred to v3).
  it('shows WIPBanner on Live tail tab', async () => {
    render(<HFLog />);

    fireEvent.click(screen.getByText('Live tail'));

    await waitFor(() => {
      expect(screen.getByTestId('wip-banner')).toBeDefined();
    });
  });
});
