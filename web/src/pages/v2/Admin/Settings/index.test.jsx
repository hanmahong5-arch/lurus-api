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
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';

vi.mock('../../../../helpers', () => ({
  API: {
    get: vi.fn(),
    put: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

vi.mock('../../../../components/hifi/HFShell', () => ({
  default: ({ children }) =>
    React.createElement('div', { 'data-testid': 'hf-shell' }, children),
}));

import HFAdminSettings from './index';
import { API } from '../../../../helpers';

const optionsResponse = (pairs) => ({
  data: {
    success: true,
    data: Object.entries(pairs).map(([key, value]) => ({ key, value })),
  },
});

beforeEach(() => {
  API.get.mockReset();
  API.put.mockReset();
});

describe('Admin Settings page', () => {
  it('renders live values from GET /options', async () => {
    API.get.mockResolvedValue(
      optionsResponse({ SystemName: 'My Hub', Footer: 'foot' }),
    );

    render(<HFAdminSettings />);

    await waitFor(() => {
      expect(screen.getByTestId('field-SystemName').value).toBe('My Hub');
    });
  });

  it('toggling a key PUTs that single key with the new boolean', async () => {
    API.get.mockResolvedValue(optionsResponse({ GitHubOAuthEnabled: 'false' }));
    API.put.mockResolvedValue({ data: { success: true } });

    render(<HFAdminSettings />);
    await waitFor(() => screen.getByTestId('panel-auth'));

    // Switch to the Auth panel and flip the GitHub OAuth toggle.
    fireEvent.click(screen.getByTestId('panel-auth'));
    await waitFor(() => screen.getByTestId('toggle-GitHubOAuthEnabled'));
    fireEvent.click(screen.getByTestId('toggle-GitHubOAuthEnabled'));

    await waitFor(() => {
      const call = API.put.mock.calls.find(
        ([url]) => url === '/api/v2/admin/options',
      );
      expect(call).toBeTruthy();
      expect(call[1]).toEqual({ key: 'GitHubOAuthEnabled', value: true });
    });
  });

  it('surfaces the backend validation-failure message inline', async () => {
    API.get.mockResolvedValue(optionsResponse({ GitHubOAuthEnabled: 'false' }));
    API.put.mockResolvedValue({
      data: { success: false, message: 'please set GitHub Client Id first' },
    });

    render(<HFAdminSettings />);
    await waitFor(() => screen.getByTestId('panel-auth'));
    fireEvent.click(screen.getByTestId('panel-auth'));
    await waitFor(() => screen.getByTestId('toggle-GitHubOAuthEnabled'));
    fireEvent.click(screen.getByTestId('toggle-GitHubOAuthEnabled'));

    await waitFor(() => {
      const err = screen.getByTestId('error-GitHubOAuthEnabled');
      expect(err.textContent).toMatch(/GitHub Client Id/i);
    });
  });

  it('shows the forbidden panel on 403', async () => {
    API.get.mockRejectedValue({ response: { status: 403 } });

    render(<HFAdminSettings />);

    await waitFor(() => {
      expect(
        screen.getAllByText('Admin access required').length,
      ).toBeGreaterThan(0);
    });
  });
});
