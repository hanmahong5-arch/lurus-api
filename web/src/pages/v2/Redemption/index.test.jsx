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

// ─── mocks ───────────────────────────────────────────────────────────────────

vi.mock('../../../helpers', () => ({
  API: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

vi.mock('../../../components/hifi/HFShell', () => ({
  default: ({ children, actions }) =>
    React.createElement(
      'div',
      { 'data-testid': 'hf-shell' },
      React.createElement('div', { 'data-testid': 'hf-actions' }, actions),
      children,
    ),
}));

// ConfirmDialog mock — renders when visible=true with confirm/cancel buttons.
vi.mock('../../../components/common/ConfirmDialog', () => ({
  default: ({ visible, onConfirm, onCancel, title }) =>
    visible
      ? React.createElement(
          'div',
          { 'data-testid': 'confirm-dialog' },
          React.createElement(
            'span',
            { 'data-testid': 'confirm-dialog-title' },
            title,
          ),
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

// Mirror i18next's en behaviour: return the English defaultValue (2nd arg)
// with {{var}} interpolation, falling back to the key when no default given.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key, fallback, opts) => {
      const vars =
        typeof fallback === 'object' && fallback !== null ? fallback : opts;
      let out = typeof fallback === 'string' ? fallback : key;
      if (vars) {
        for (const [k, v] of Object.entries(vars)) {
          out = out.split(`{{${k}}}`).join(String(v));
        }
      }
      return out;
    },
  }),
}));

import HFRedemption from './index';
import { API, showError, showSuccess } from '../../../helpers';

// ─── fixtures ────────────────────────────────────────────────────────────────

const makeRedemption = (id, name, status = 1) => ({
  id,
  key: `abcd****************************${id}`.slice(0, 32),
  name,
  quota: 500000,
  status,
  created_time: 1716000000,
  expired_time: 0,
  used_user_id: status === 3 ? 7 : 0,
});

const mockListOk = (items) => ({
  data: {
    success: true,
    data: { redemptions: items, total: items.length, page: 1, page_size: 50 },
  },
});

beforeEach(() => {
  API.get.mockReset();
  API.post.mockReset();
  API.delete.mockReset();
  showError.mockReset();
  showSuccess.mockReset();
  window.localStorage.clear();
  window.localStorage.setItem('tenant_slug', 'acme');
});

// ─── tests ───────────────────────────────────────────────────────────────────

describe('Redemption page', () => {
  // 1. Renders the redemption list from API.
  it('renders redemption list', async () => {
    const items = [
      makeRedemption(1, 'promo-2025'),
      makeRedemption(2, 'partner-code', 3),
    ];
    API.get.mockResolvedValue(mockListOk(items));

    render(React.createElement(HFRedemption));

    await waitFor(() => {
      expect(screen.getByTestId('redemption-row-1')).toBeDefined();
      expect(screen.getByTestId('redemption-row-2')).toBeDefined();
    });

    // Verify API was called with correct tenant slug.
    expect(API.get).toHaveBeenCalledWith(
      expect.stringContaining('/api/v2/acme/redemptions'),
    );

    // Delete buttons use data-testid with row id.
    expect(screen.getByTestId('redemption-delete-btn-1')).toBeDefined();
    expect(screen.getByTestId('redemption-delete-btn-2')).toBeDefined();
  });

  // 2. Empty state shown when list is empty.
  it('shows empty state when no redemptions', async () => {
    API.get.mockResolvedValue(mockListOk([]));

    render(React.createElement(HFRedemption));

    await waitFor(() => {
      expect(screen.getByTestId('redemption-empty')).toBeDefined();
    });
  });

  // 3. Create flow — opens modal, submits, shows keys banner, refreshes list.
  it('create flow shows keys banner after success', async () => {
    // First fetch: empty list. Second fetch (after create): list with new item.
    API.get
      .mockResolvedValueOnce(mockListOk([]))
      .mockResolvedValueOnce(mockListOk([makeRedemption(10, 'batch-promo')]));

    API.post.mockResolvedValue({
      data: {
        success: true,
        data: {
          codes: [{ id: 10, key: 'a'.repeat(32), name: 'batch-promo' }],
          count: 1,
          requested: 1,
        },
      },
    });

    render(React.createElement(HFRedemption));

    // Wait for initial load.
    await waitFor(() => expect(API.get).toHaveBeenCalledTimes(1));

    // Open create modal.
    fireEvent.click(screen.getByTestId('redemption-create-btn'));

    // Fill name field.
    await waitFor(() => screen.getByTestId('redemption-name-input'));
    fireEvent.change(screen.getByTestId('redemption-name-input'), {
      target: { value: 'batch-promo' },
    });

    // Submit.
    fireEvent.click(screen.getByTestId('redemption-create-submit'));

    await waitFor(() => {
      expect(API.post).toHaveBeenCalledWith(
        `/api/v2/acme/redemptions`,
        expect.objectContaining({ name: 'batch-promo' }),
      );
    });

    // Keys banner should appear showing the new codes.
    await waitFor(() => {
      expect(screen.getByTestId('redemption-keys-list')).toBeDefined();
    });

    expect(showSuccess).toHaveBeenCalledWith('Redemption codes generated');

    // List should be refreshed — row 10 visible.
    await waitFor(() => {
      expect(screen.getByTestId('redemption-row-10')).toBeDefined();
    });
  });

  // 4. Delete with confirm dialog — happy path.
  it('delete with confirm dialog calls DELETE and refreshes list', async () => {
    const item = makeRedemption(5, 'to-delete');
    API.get
      .mockResolvedValueOnce(mockListOk([item]))
      .mockResolvedValueOnce(mockListOk([]));
    API.delete.mockResolvedValue({ data: { success: true } });

    render(React.createElement(HFRedemption));

    await waitFor(() => screen.getByTestId('redemption-delete-btn-5'));

    // Click delete button — confirm dialog should open.
    fireEvent.click(screen.getByTestId('redemption-delete-btn-5'));

    await waitFor(() => {
      expect(screen.getByTestId('confirm-dialog')).toBeDefined();
    });

    // Confirm deletion.
    fireEvent.click(screen.getByTestId('confirm-ok'));

    await waitFor(() => {
      expect(API.delete).toHaveBeenCalledWith(`/api/v2/acme/redemptions/5`);
    });

    expect(showSuccess).toHaveBeenCalledWith('Redemption code deleted');

    // After delete, list is refreshed and empty state shown.
    await waitFor(() => {
      expect(screen.getByTestId('redemption-empty')).toBeDefined();
    });
  });

  // 5. Delete cancel — does NOT call API.delete.
  it('cancel confirm dialog does not call delete API', async () => {
    const item = makeRedemption(6, 'keep-me');
    API.get.mockResolvedValue(mockListOk([item]));

    render(React.createElement(HFRedemption));

    await waitFor(() => screen.getByTestId('redemption-delete-btn-6'));

    fireEvent.click(screen.getByTestId('redemption-delete-btn-6'));

    await waitFor(() => screen.getByTestId('confirm-dialog'));

    // Cancel the dialog.
    fireEvent.click(screen.getByTestId('confirm-cancel'));

    // Dialog should be gone.
    expect(screen.queryByTestId('confirm-dialog')).toBeNull();

    // No delete call made.
    expect(API.delete).not.toHaveBeenCalled();
  });

  // 6. API error on list fetch — does not crash.
  it('handles API error on list fetch gracefully', async () => {
    API.get.mockRejectedValue(new Error('network error'));

    // Should not throw.
    render(React.createElement(HFRedemption));

    // Loading state resolves without crash.
    await waitFor(() => {
      expect(screen.getByTestId('hf-shell')).toBeDefined();
    });
  });
});
