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
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
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

// WIPBanner renders inline — keep real implementation so test can assert on it.
vi.mock('../../../components/hifi/WIPBanner', () => ({
  default: ({ reason }) =>
    React.createElement('span', { 'data-testid': 'wip-banner', 'data-reason': reason }, reason),
}));

import HFBilling from './index';
import { API, showError } from '../../../helpers';

const fakeInvoices = [
  { month: '2026-05', quota: 8420400, amount_cny: 16.84, amount_usd: 16.84, request_count: 120 },
  { month: '2026-04', quota: 9842200, amount_cny: 19.68, amount_usd: 19.68, request_count: 200 },
];
const fakeSummary = {
  wallet_balance_cny: 1210.5,
  mtd_spend_cny: 16.84,
  subscription_plan: 'pro',
};

beforeEach(() => {
  API.get.mockReset();
  API.post.mockReset();
  showError.mockReset();
  window.localStorage.clear();
  window.localStorage.setItem('tenant_slug', 'acme');
});

describe('Billing page', () => {
  // 1. On mount, fetches invoices + summary; rendered content reflects mock data.
  it('fetches invoices + summary on mount', async () => {
    // Use mockResolvedValue (sticky) so repeated calls due to slug re-initialisation
    // from localStorage all resolve successfully without throwing.
    API.get.mockImplementation((url) => {
      if (url.includes('/billing/invoices')) {
        return Promise.resolve({ data: { success: true, data: { items: fakeInvoices } } });
      }
      return Promise.resolve({ data: { success: true, data: fakeSummary } });
    });

    render(<HFBilling />);

    // Wait until the acme-slug fetch has been called (may fire twice due to
    // slug state initialising from localStorage asynchronously).
    await waitFor(() => {
      const calls = API.get.mock.calls.map(([url]) => url);
      expect(calls).toContain('/api/v2/acme/billing/invoices');
    });

    const calls = API.get.mock.calls.map(([url]) => url);
    expect(calls).toContain('/api/v2/user/billing/summary');

    // Invoice row for 2026-05 should appear in the DOM.
    await waitFor(() => {
      expect(screen.getByText('2026-05')).toBeTruthy();
    });
  });

  // 2. Clicking recharge calls POST /api/v2/user/billing/checkout and on
  //    success navigates to checkout_url via window.location.href.
  it('navigates to checkout on recharge click', async () => {
    API.get
      .mockResolvedValue({ data: { success: true, data: { items: [] } } });
    API.post.mockResolvedValueOnce({
      data: { success: true, data: { checkout_url: 'https://pay.example.com/order/123' } },
    });

    // Intercept window.location.href assignment without actual navigation.
    const originalLocation = window.location;
    delete window.location;
    let navigatedTo = null;
    window.location = { ...originalLocation, set href(url) { navigatedTo = url; } };

    render(<HFBilling />);

    const btn = screen.getByTestId('billing-recharge');
    fireEvent.click(btn);

    await waitFor(() => {
      expect(API.post).toHaveBeenCalledTimes(1);
    });

    const [url] = API.post.mock.calls[0];
    expect(url).toBe('/api/v2/user/billing/checkout');

    await waitFor(() => {
      expect(navigatedTo).toBe('https://pay.example.com/order/123');
    });

    window.location = originalLocation;
  });

  // 3. WIPBanner appears near download (PDF) buttons — asserts inline markers
  //    are present in the rendered DOM.
  it('shows WIPBanner near PDF download buttons', async () => {
    API.get
      .mockResolvedValueOnce({ data: { success: true, data: { items: fakeInvoices } } })
      .mockResolvedValueOnce({ data: { success: true, data: fakeSummary } });

    render(<HFBilling />);

    await waitFor(() => {
      expect(screen.getByText('2026-05')).toBeTruthy();
    });

    // At least one WIPBanner with PDF reason should be present per invoice row.
    const pdfBanners = screen
      .getAllByTestId('wip-banner')
      .filter((el) => el.dataset.reason && el.dataset.reason.includes('PDF'));

    expect(pdfBanners.length).toBeGreaterThanOrEqual(1);

    // payment method editing WIPBanner also present
    const paymentBanners = screen
      .getAllByTestId('wip-banner')
      .filter((el) => el.dataset.reason && el.dataset.reason.includes('payment method editing'));

    expect(paymentBanners.length).toBeGreaterThanOrEqual(1);
  });
});
