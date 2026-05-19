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

// Mock helpers BEFORE importing the component — vi.mock is hoisted but the
// component module reads API.get at runtime so mocks resolve at first call.
vi.mock('../../../helpers', () => ({
  API: {
    get: vi.fn(),
    post: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

// HFShell pulls TenantSwitcher → API helper chain → react-router. Stub it
// to a passthrough wrapper so the test focuses on Models UI/logic.
vi.mock('../../../components/hifi/HFShell', () => ({
  default: ({ children, actions }) =>
    React.createElement(
      'div',
      { 'data-testid': 'hf-shell' },
      React.createElement('div', { 'data-testid': 'shell-actions' }, actions),
      children,
    ),
}));

// WIPBanner is rendered inline — stub to a simple data-testid marker so tests
// can assert presence without caring about its internal markup.
vi.mock('../../../components/hifi/WIPBanner', () => ({
  default: ({ reason }) =>
    React.createElement(
      'div',
      { 'data-testid': 'wip-banner', role: 'status' },
      reason,
    ),
}));

import HFModels from './index';
import { API, showError } from '../../../helpers';

beforeEach(() => {
  API.get.mockReset();
  API.post.mockReset();
  showError.mockReset();
  window.localStorage.clear();
  window.localStorage.setItem('tenant_slug', 'acme');
});

const fakeModelsResponse = (items, vendorCounts = {}) => ({
  data: {
    success: true,
    data: {
      items,
      total: items.length,
      limit: 100,
      offset: 0,
      vendor_counts: vendorCounts,
    },
  },
});

describe('Models page', () => {
  it('fetches models on mount', async () => {
    const items = [
      { id: 1, model_name: 'gpt-4o', vendor: 'OpenAI', status: 1 },
      {
        id: 2,
        model_name: 'claude-3.5-sonnet',
        vendor: 'Anthropic',
        status: 1,
      },
      { id: 3, model_name: 'gemini-1.5-pro', vendor: 'Google', status: 1 },
    ];
    // useTenantSlug starts at 'default' then updates to 'acme' — two fetches occur.
    API.get.mockResolvedValue(fakeModelsResponse(items));

    render(<HFModels />);

    // API called at least once; the final call must use the slug from localStorage.
    await waitFor(() => {
      expect(API.get).toHaveBeenCalledTimes(2);
    });
    // Last call must target the correct tenant slug.
    const lastCall = API.get.mock.calls.at(-1);
    expect(lastCall[0]).toContain('/api/v2/acme/models');

    // All 3 model names should appear
    await waitFor(() => {
      expect(screen.getByTestId('model-card-gpt-4o')).toBeDefined();
      expect(screen.getByTestId('model-card-claude-3.5-sonnet')).toBeDefined();
      expect(screen.getByTestId('model-card-gemini-1.5-pro')).toBeDefined();
    });
  });

  it('filters by vendor', async () => {
    const allItems = [
      { id: 1, model_name: 'gpt-4o', vendor: 'OpenAI', status: 1 },
      { id: 2, model_name: 'gpt-4o-mini', vendor: 'OpenAI', status: 1 },
      {
        id: 3,
        model_name: 'claude-3.5-sonnet',
        vendor: 'Anthropic',
        status: 1,
      },
    ];
    // useTenantSlug triggers 2 initial fetches (default → acme), then 1 more on
    // vendor click — mock responds with appropriate data for each call.
    // Call 1 (slug=default): unfiltered
    API.get.mockResolvedValueOnce(
      fakeModelsResponse(allItems, { OpenAI: 2, Anthropic: 1 }),
    );
    // Call 2 (slug=acme): unfiltered
    API.get.mockResolvedValueOnce(
      fakeModelsResponse(allItems, { OpenAI: 2, Anthropic: 1 }),
    );
    // Call 3 (slug=acme, vendor=OpenAI): filtered
    API.get.mockResolvedValueOnce(
      fakeModelsResponse(
        allItems.filter((m) => m.vendor === 'OpenAI'),
        { OpenAI: 2, Anthropic: 1 },
      ),
    );

    render(<HFModels />);

    // Wait for vendor pills to appear after initial fetch
    await waitFor(() => {
      expect(screen.getByTestId('vendor-filter-OpenAI')).toBeDefined();
    });

    // Click OpenAI vendor pill
    fireEvent.click(screen.getByTestId('vendor-filter-OpenAI'));

    await waitFor(() => {
      expect(API.get).toHaveBeenCalledTimes(3);
    });

    // Last call must include vendor=OpenAI
    const lastCall = API.get.mock.calls.at(-1);
    expect(lastCall[0]).toContain('vendor=OpenAI');
  });

  it('shows WIPBanner on add button section', async () => {
    // Two fetches due to useTenantSlug default→acme transition
    API.get.mockResolvedValue(fakeModelsResponse([]));

    render(<HFModels />);

    // WIPBanner(s) are rendered; at least one should be in the actions area
    // (the one near the "+add model" button) or inside the cards area.
    // We assert that at least one WIPBanner exists in the rendered output.
    await waitFor(() => {
      const banners = screen.getAllByTestId('wip-banner');
      expect(banners.length).toBeGreaterThanOrEqual(1);
    });

    // The add button itself should be present (even if disabled)
    expect(screen.getByTestId('models-add-btn')).toBeDefined();
  });
});
