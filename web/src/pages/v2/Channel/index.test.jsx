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
    put: vi.fn(),
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

vi.mock('../../../components/common/ConfirmDialog', () => ({
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

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k, _opts) => k }),
}));

import HFChannel from './index';
import { API, showError, showSuccess } from '../../../helpers';

// ─── fixtures ────────────────────────────────────────────────────────────────

const makeChannel = (id, name) => ({
  Id: id,
  Name: name,
  Type: 1,
  Models: 'gpt-4,gpt-3.5-turbo',
  Status: 1,
  Group: 'default',
});

const mockListResponse = (channels) => ({
  data: {
    success: true,
    data: { channels, total: channels.length, page: 1, page_size: 100 },
  },
});

beforeEach(() => {
  API.get.mockReset();
  API.post.mockReset();
  API.put.mockReset();
  API.delete.mockReset();
  showError.mockReset();
  showSuccess.mockReset();
  window.localStorage.clear();
  window.localStorage.setItem('tenant_slug', 'acme');
});

// ─── test button shows latency on success ─────────────────────────────────────

describe('channel test button', () => {
  it('shows latency toast on success', async () => {
    const ch = makeChannel(42, 'OpenAI Main');
    API.get.mockResolvedValue(mockListResponse([ch]));
    API.post.mockResolvedValue({
      data: { success: true, latency_ms: 137 },
    });

    render(React.createElement(HFChannel));

    // Wait for channel list to render.
    await waitFor(() => screen.getByTestId('test-btn-42'));

    fireEvent.click(screen.getByTestId('test-btn-42'));

    await waitFor(() => {
      expect(API.post).toHaveBeenCalledWith(
        '/api/v2/acme/channels/42/test',
        {},
      );
    });

    await waitFor(() => {
      expect(showSuccess).toHaveBeenCalledWith(
        expect.stringContaining('渠道延迟 {{ms}}ms'),
      );
    });
  });

  it('shows error toast on failure', async () => {
    const ch = makeChannel(55, 'Broken Channel');
    API.get.mockResolvedValue(mockListResponse([ch]));
    API.post.mockResolvedValue({
      data: { success: false, latency_ms: 0, error: 'invalid api key' },
    });

    render(React.createElement(HFChannel));

    await waitFor(() => screen.getByTestId('test-btn-55'));
    fireEvent.click(screen.getByTestId('test-btn-55'));

    await waitFor(() => {
      expect(showError).toHaveBeenCalled();
    });
  });
});

// ─── sync modal shows model diff ─────────────────────────────────────────────

describe('sync upstream models modal', () => {
  it('shows model diff and applies selection', async () => {
    const ch = makeChannel(99, 'Sync Channel');
    API.get.mockImplementation((url) => {
      if (url.includes('/channels/99/upstream-models')) {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              upstream: ['gpt-4', 'gpt-3.5-turbo', 'gpt-4o'],
              new: ['gpt-4o'],
              missing: [],
            },
          },
        });
      }
      // Default: channel list
      return Promise.resolve(mockListResponse([ch]));
    });
    API.put.mockResolvedValue({ data: { success: true } });

    render(React.createElement(HFChannel));

    // Wait for channel name to appear then expand.
    await waitFor(() => screen.getByText('Sync Channel'));
    const expandBtn = await waitFor(() =>
      screen.getAllByRole('button').find((b) => b.textContent === '▸'),
    );
    expect(expandBtn).toBeTruthy();
    fireEvent.click(expandBtn);

    // Click sync button.
    const syncBtn = await waitFor(() => screen.getByTestId('sync-btn-99'));
    fireEvent.click(syncBtn);

    // Modal should appear with new model pill.
    await waitFor(() => screen.getByTestId('sync-new-gpt-4o'));

    // Apply button should be enabled (gpt-4o is pre-selected).
    const applyBtn = screen.getByTestId('sync-apply-btn');
    expect(applyBtn).not.toBeDisabled();

    fireEvent.click(applyBtn);

    await waitFor(() => {
      expect(API.put).toHaveBeenCalledWith(
        '/api/v2/acme/channels/99',
        expect.objectContaining({ models: expect.stringContaining('gpt-4o') }),
      );
    });
  });

  it('shows error when upstream fetch fails', async () => {
    const ch = makeChannel(77, 'Failing Sync');
    API.get.mockImplementation((url) => {
      if (url.includes('/channels/77/upstream-models')) {
        return Promise.resolve({
          data: { success: false, message: 'timeout' },
        });
      }
      return Promise.resolve(mockListResponse([ch]));
    });

    render(React.createElement(HFChannel));

    await waitFor(() => screen.getByText('Failing Sync'));
    const expandBtn = await waitFor(() =>
      screen.getAllByRole('button').find((b) => b.textContent === '▸'),
    );
    expect(expandBtn).toBeTruthy();
    fireEvent.click(expandBtn);

    const syncBtn = await waitFor(() => screen.getByTestId('sync-btn-77'));
    fireEvent.click(syncBtn);

    await waitFor(() => {
      expect(showError).toHaveBeenCalled();
    });
  });
});
