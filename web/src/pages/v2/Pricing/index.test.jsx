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
import { render, screen, waitFor } from '@testing-library/react';

// Mock helpers BEFORE importing the component.
vi.mock('../../../helpers', () => ({
  API: {
    get: vi.fn(),
    post: vi.fn(),
  },
  showError: vi.fn(),
}));

// Stub HFShell — avoids react-router / TenantSwitcher chain.
vi.mock('../../../components/hifi/HFShell', () => ({
  default: ({ children, actions }) =>
    React.createElement(
      'div',
      { 'data-testid': 'hf-shell' },
      React.createElement(
        'div',
        { 'data-testid': 'hf-shell-actions' },
        actions,
      ),
      children,
    ),
}));

// Stub WIPBanner to a simple div so we can assert its presence.
vi.mock('../../../components/hifi/WIPBanner', () => ({
  default: ({ reason }) =>
    React.createElement(
      'div',
      { 'data-testid': 'wip-banner' },
      reason ?? 'WIP',
    ),
}));

// Stub Semi Spin — just renders children.
vi.mock('@douyinfe/semi-ui', () => ({
  Spin: ({ children }) => React.createElement('div', null, children),
}));

import PricingPage from './index';
import { API } from '../../../helpers';

const fakePricingResponse = (models) => ({
  data: {
    success: true,
    data: {
      pricing: models,
      vendors: [...new Set(models.map((m) => m.vendor).filter(Boolean))],
      group_ratio: { default: 1.0, vip: 0.85 },
    },
  },
});

beforeEach(() => {
  API.get.mockReset();
  API.post.mockReset();
  // Default: empty pricing so tests that don't exercise data still mount cleanly.
  API.get.mockResolvedValue(fakePricingResponse([]));
  window.localStorage.clear();
  window.localStorage.setItem('tenant_slug', 'acme');
});

describe('Pricing page', () => {
  // 1. Fetches pricing on mount — 3 model rows render in the table.
  it('fetches pricing on mount', async () => {
    // mockResolvedValue (not Once) so all calls — including the second
    // triggered when useTenantSlug reads localStorage and updates slug
    // from 'default' → 'acme' — return the same 3-model payload.
    API.get.mockResolvedValue(
      fakePricingResponse([
        {
          model_name: 'gpt-4o',
          vendor: 'OpenAI',
          quota_type: 0,
          model_ratio: 2.5,
          model_price: 0,
          enable_groups: ['default', 'vip'],
          supported_endpoint_types: ['chat'],
        },
        {
          model_name: 'claude-3.5-sonnet',
          vendor: 'Anthropic',
          quota_type: 1,
          model_ratio: 0,
          model_price: 3.0,
          enable_groups: ['vip'],
          supported_endpoint_types: ['chat'],
        },
        {
          model_name: 'gemini-1.5-pro',
          vendor: 'Google',
          quota_type: 0,
          model_ratio: 1.25,
          model_price: 0,
          enable_groups: ['default'],
          supported_endpoint_types: ['chat'],
        },
      ]),
    );

    render(<PricingPage />);

    await waitFor(() => {
      expect(API.get).toHaveBeenCalledWith('/api/v2/acme/pricing');
    });

    await waitFor(() => {
      expect(screen.getByTestId('pricing-table').textContent).toContain(
        'gpt-4o',
      );
      expect(screen.getByTestId('pricing-table').textContent).toContain(
        'claude-3.5-sonnet',
      );
      expect(screen.getByTestId('pricing-table').textContent).toContain(
        'gemini-1.5-pro',
      );
    });
  });

  // 2. Save button is always disabled — write API pending Epic 12.
  it('disables save button', async () => {
    API.get.mockResolvedValueOnce(fakePricingResponse([]));
    render(<PricingPage />);

    // Button rendered in actions area (synchronous — no data dependency).
    const saveBtn = screen.getByTestId('pricing-save');
    expect(saveBtn).toBeDisabled();
  });

  // 3. WIPBanner renders near save button (inside actions area).
  it('shows WIPBanner near save', async () => {
    API.get.mockResolvedValueOnce(fakePricingResponse([]));
    render(<PricingPage />);

    const actionsArea = screen.getByTestId('hf-shell-actions');
    // WIPBanner is inside the actions area, next to the save button.
    const banner = actionsArea.querySelector('[data-testid="wip-banner"]');
    expect(banner).not.toBeNull();
  });
});
