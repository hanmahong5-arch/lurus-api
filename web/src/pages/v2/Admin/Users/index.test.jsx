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
    delete: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

vi.mock('../../../../components/hifi/HFShell', () => ({
  default: ({ children, actions }) =>
    React.createElement(
      'div',
      { 'data-testid': 'hf-shell' },
      React.createElement('div', { 'data-testid': 'hf-actions' }, actions),
      children,
    ),
}));

vi.mock('../../../../components/common/ConfirmDialog', () => ({
  default: ({ visible, onConfirm, onCancel, title }) =>
    visible
      ? React.createElement(
          'div',
          { 'data-testid': 'confirm-dialog' },
          React.createElement('span', null, title),
          React.createElement(
            'button',
            { 'data-testid': 'confirm-ok', onClick: onConfirm },
            'ok',
          ),
          React.createElement(
            'button',
            { 'data-testid': 'confirm-cancel', onClick: onCancel },
            'cancel',
          ),
        )
      : null,
}));

import HFAdminUsers from './index';
import { API } from '../../../../helpers';

const makeUser = (overrides = {}) => ({
  id: 7,
  username: 'alice',
  display_name: 'Alice',
  email: 'alice@example.com',
  role: 1,
  status: 1,
  group: 'default',
  quota: 500000,
  used_quota: 250000,
  request_count: 5,
  ...overrides,
});

const listResponse = (users) => ({
  data: {
    success: true,
    data: { users, total: users.length, page: 1, page_size: 50 },
  },
});

beforeEach(() => {
  API.get.mockReset();
  API.put.mockReset();
  API.delete.mockReset();
});

describe('Admin Users page', () => {
  it('renders users from GET with no secret columns', async () => {
    API.get.mockResolvedValue(listResponse([makeUser()]));

    render(<HFAdminUsers />);

    await waitFor(() => screen.getByText('Alice'));
    expect(screen.getByText('alice@example.com')).toBeTruthy();

    // The table must not expose any secret column.
    expect(screen.queryByText(/access.?token/i)).toBeNull();
    expect(screen.queryByText(/password/i)).toBeNull();
    expect(screen.queryByText(/secret/i)).toBeNull();
  });

  it('edits a user and PUTs role/status/quota/group', async () => {
    API.get.mockResolvedValue(listResponse([makeUser()]));
    API.put.mockResolvedValue({ data: { success: true, data: makeUser() } });

    render(<HFAdminUsers />);
    await waitFor(() => screen.getByTestId('user-edit-btn-7'));

    fireEvent.click(screen.getByTestId('user-edit-btn-7'));
    await waitFor(() => screen.getByTestId('edit-save'));

    // Change quota cap to $3 and save.
    fireEvent.change(screen.getByTestId('edit-quota'), {
      target: { value: '3' },
    });
    fireEvent.click(screen.getByTestId('edit-save'));

    await waitFor(() => {
      expect(API.put).toHaveBeenCalledWith(
        '/api/v2/admin/users/7',
        expect.objectContaining({
          role: 1,
          status: 1,
          quota: 1500000, // 3 * 500_000
          group: 'default',
        }),
      );
    });
  });

  it('deletes a user behind the typed-confirm dialog', async () => {
    API.get.mockResolvedValue(listResponse([makeUser()]));
    API.delete.mockResolvedValue({ data: { success: true } });

    render(<HFAdminUsers />);
    await waitFor(() => screen.getByTestId('user-delete-btn-7'));

    fireEvent.click(screen.getByTestId('user-delete-btn-7'));
    await waitFor(() => screen.getByTestId('confirm-dialog'));
    fireEvent.click(screen.getByTestId('confirm-ok'));

    await waitFor(() => {
      expect(API.delete).toHaveBeenCalledWith('/api/v2/admin/users/7');
    });
  });

  it('shows the forbidden panel on 403', async () => {
    API.get.mockRejectedValue({ response: { status: 403 } });

    render(<HFAdminUsers />);

    await waitFor(() => {
      expect(
        screen.getAllByText('Admin access required').length,
      ).toBeGreaterThan(0);
    });
  });

  it('keeps the create control disabled with an honest reason', async () => {
    API.get.mockResolvedValue(listResponse([]));

    render(<HFAdminUsers />);
    await waitFor(() => screen.getByTestId('new-user-btn'));

    const btn = screen.getByTestId('new-user-btn');
    expect(btn.disabled).toBe(true);
    expect(btn.getAttribute('title')).toMatch(/password\/invite flow/i);
  });
});
