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

vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
}));

vi.mock('react-i18next', () => ({
  // Honor a string defaultValue (t(key, 'English')) like real i18next, so
  // console.* wrapping renders readable English in tests instead of bare keys.
  useTranslation: () => ({
    t: (k, d) => (typeof d === 'string' ? d : k),
  }),
  // formatting.js imports the real i18n instance, which registers this
  // plugin at module load — the mock must expose it or the suite can't load.
  initReactI18next: { type: '3rdParty', init: () => {} },
}));

vi.mock('../../../components/hifi/HFShell', () => ({
  default: ({ children, actions }) =>
    React.createElement(
      'div',
      { 'data-testid': 'hf-shell' },
      React.createElement('div', null, actions),
      children,
    ),
}));

// Interactive stub: renders a confirm button only when visible so tests can
// drive the typed-confirmation flows without reproducing the Semi Modal.
vi.mock('../../../components/common/ConfirmDialog', () => ({
  default: ({ visible, onConfirm }) =>
    visible
      ? React.createElement(
          'button',
          { 'data-testid': 'confirm-ok', onClick: onConfirm },
          'confirm',
        )
      : null,
}));

import HFToken from './index';
import { API } from '../../../helpers';

const fakeToken = {
  id: 1,
  name: 'prod',
  key: 'abcd1234efgh',
  status: 1,
  unlimited_quota: true,
  used_quota: 0,
  remain_quota: 0,
  expired_time: -1,
  created_time: Math.floor(Date.now() / 1000) - 3600,
  accessed_time: Math.floor(Date.now() / 1000) - 60,
};

beforeEach(() => {
  API.get.mockReset();
  API.post.mockReset();
  window.localStorage.clear();
  window.localStorage.setItem('tenant_slug', 'acme');
  // jsdom has no clipboard by default — provide a spy.
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
  });
});

describe('Token page — multi-format client URLs', () => {
  it('renders the client base URLs and copies the OpenAI base url', async () => {
    API.get.mockResolvedValue({
      data: { success: true, data: { items: [fakeToken] } },
    });

    render(<HFToken />);

    await waitFor(() => screen.getByText('client base urls'));

    // Gemini base url is unique to the client-endpoints panel.
    expect(screen.getByText('https://api.lurus.cn/v1beta')).toBeTruthy();

    // Copy the OpenAI-compatible endpoint.
    const copyBtn = screen.getByTestId('copy-endpoint-OpenAI / compatible');
    fireEvent.click(copyBtn);

    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
        'https://api.lurus.cn/v1',
      );
    });
  });

  it('offers a copy control for each supported SDK base url', async () => {
    API.get.mockResolvedValue({
      data: { success: true, data: { items: [fakeToken] } },
    });

    render(<HFToken />);
    await waitFor(() => screen.getByText('client base urls'));

    for (const label of [
      'OpenAI / compatible',
      'Anthropic · Claude SDK',
      'Gemini',
    ]) {
      expect(screen.getByTestId(`copy-endpoint-${label}`)).toBeTruthy();
    }
  });
});

describe('Token page — batch operations', () => {
  const t1 = { ...fakeToken, id: 1, name: 'one' };
  const t2 = { ...fakeToken, id: 2, name: 'two' };

  it('hides the batch bar until tokens are selected, then deletes via one POST', async () => {
    API.get.mockResolvedValue({
      data: { success: true, data: { items: [t1, t2] } },
    });
    API.post.mockResolvedValue({ data: { success: true, deleted: 2 } });

    render(<HFToken />);
    await waitFor(() => screen.getByText('one'));

    // No batch bar with an empty selection.
    expect(screen.queryByTestId('token-batch-bar')).toBeNull();

    // Select both tokens.
    fireEvent.click(screen.getByTestId('token-check-1'));
    fireEvent.click(screen.getByTestId('token-check-2'));

    // Batch bar now visible.
    expect(screen.getByTestId('token-batch-bar')).toBeTruthy();

    // Delete → confirm.
    fireEvent.click(screen.getByTestId('token-batch-delete-btn'));
    fireEvent.click(screen.getByTestId('confirm-ok'));

    await waitFor(() => {
      const postCall = API.post.mock.calls.find(([url]) =>
        String(url).includes('/tokens/batch-delete'),
      );
      expect(postCall).toBeTruthy();
      expect(postCall[1].ids).toEqual([1, 2]);
    });
  });

  it('keeps batch copy disabled with an honest deferral reason', async () => {
    API.get.mockResolvedValue({
      data: { success: true, data: { items: [t1] } },
    });

    render(<HFToken />);
    await waitFor(() => screen.getByText('one'));
    fireEvent.click(screen.getByTestId('token-check-1'));

    const copyBtn = screen.getByTestId('token-batch-copy-btn');
    expect(copyBtn.disabled).toBe(true);
    expect(copyBtn.getAttribute('title')).toMatch(/key-reveal endpoint/i);
  });
});
