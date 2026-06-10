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
      React.createElement('div', { 'data-testid': 'hf-actions' }, actions),
      children,
    ),
}));

// ConfirmDialog — use real implementation so armed/disarmed logic is tested.
// It imports Semi Modal; stub that out.
vi.mock('@douyinfe/semi-ui', () => {
  // Semi Button passes extra props (data-testid etc) through.
  const Button = ({ children, onClick, disabled, loading, ...rest }) =>
    React.createElement(
      'button',
      { onClick, disabled: disabled || loading, ...rest },
      children,
    );
  const Modal = ({ visible, children, footer, title, onCancel, closable }) =>
    visible
      ? React.createElement(
          'div',
          { 'data-testid': 'modal' },
          React.createElement('div', null, title),
          children,
          footer,
          closable !== false &&
            React.createElement(
              'button',
              { 'data-testid': 'modal-close', onClick: onCancel },
              'close',
            ),
        )
      : null;
  // Semi Input calls onChange(value) not onChange(event).
  // Wrap to accept both DOM event form (from fireEvent.change) and the raw-value
  // form that ConfirmDialog passes.
  const Input = ({ value, onChange, autoFocus: _af, ...rest }) =>
    React.createElement('input', {
      type: 'text',
      value: value ?? '',
      onChange: (e) => onChange && onChange(e.target.value),
      ...rest,
    });
  const Typography = {
    Text: ({ children }) => React.createElement('span', null, children),
  };
  return { Button, Input, Modal, Typography };
});

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
          out = out.split('{{' + k + '}}').join(String(v));
        }
      }
      return out;
    },
  }),
  initReactI18next: { type: '3rdParty', init: () => {} },
}));

vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
}));

import HFFlows from './index';
import { API, showError } from '../../../helpers';

beforeEach(() => {
  API.get.mockReset();
  API.post.mockReset();
  showError.mockReset();
  window.localStorage.clear();
  window.localStorage.setItem('tenant_slug', 'acme');
  // Reset form draft between tests.
  window.localStorage.removeItem('flow-newtoken-draft');
});

afterEach(() => {
  vi.useRealTimers();
});

// Helper: click the newToken tab.
function switchToNewToken() {
  fireEvent.click(screen.getByTestId('flows-tab-newToken'));
}

// Helper: fill step-1 fields.
function fillStep1(name, group = '') {
  fireEvent.change(screen.getByTestId('newtoken-name'), {
    target: { value: name },
  });
  if (group) {
    fireEvent.change(screen.getByTestId('newtoken-group'), {
      target: { value: group },
    });
  }
}

// Helper: advance to the next step via nav button.
function clickNext() {
  fireEvent.click(screen.getByTestId('flows-next'));
}

describe('Flows page — newToken wizard', () => {
  // 1. Full happy-path: step 1 → step 2 → step 3 review → typed confirm → POST.
  it('navigates through 3 wizard steps and submits', async () => {
    API.post.mockResolvedValueOnce({
      data: {
        success: true,
        data: { id: 7, name: 'my-token', key: 'sk-abc123' },
      },
    });

    render(<HFFlows />);
    switchToNewToken();

    // Step 1 — fill name + group.
    fillStep1('my-token', 'team-a');
    clickNext();

    // Step 2 — uncheck unlimited, set quota.
    fireEvent.click(screen.getByTestId('newtoken-unlimited'));
    fireEvent.change(screen.getByTestId('newtoken-quota'), {
      target: { value: '250000' },
    });
    clickNext();

    // Step 3 — review visible, open confirm dialog.
    expect(screen.getByTestId('newtoken-open-confirm')).toBeTruthy();
    fireEvent.click(screen.getByTestId('newtoken-open-confirm'));

    // ConfirmDialog renders with input — type the token name to arm it.
    const input = screen.getByRole('textbox');
    fireEvent.change(input, { target: { value: 'my-token' } });

    // Click the confirm button (data-testid from ConfirmDialog).
    fireEvent.click(screen.getByTestId('confirm-dialog-confirm'));

    await waitFor(() => {
      expect(API.post).toHaveBeenCalledTimes(1);
    });

    const [url, body] = API.post.mock.calls[0];
    expect(url).toBe('/api/v2/acme/tokens');
    expect(body.name).toBe('my-token');
    expect(body.group).toBe('team-a');
    expect(body.unlimited_quota).toBe(false);
    expect(body.remain_quota).toBe(250000);

    // Success screen renders the returned key.
    await waitFor(() => {
      expect(screen.getByTestId('newtoken-key').textContent).toBe('sk-abc123');
    });
  });

  // 2. Draft persists across step navigation via useFormDraft.
  it('persists draft via useFormDraft across step navigation', () => {
    render(<HFFlows />);
    switchToNewToken();

    // Type a name on step 1.
    fillStep1('persistent-token');

    // Advance to step 2.
    clickNext();

    // Go back to step 1.
    fireEvent.click(screen.getByTestId('flows-back'));

    // Name must still be present (draft restored from localStorage).
    expect(screen.getByTestId('newtoken-name').value).toBe('persistent-token');
  });

  // 3. Submit button in ConfirmDialog stays disabled until name matches exactly.
  it('blocks submit unless ConfirmDialog typed name matches', async () => {
    render(<HFFlows />);
    switchToNewToken();

    fillStep1('exact-name');
    clickNext(); // step 2
    clickNext(); // step 3 review

    fireEvent.click(screen.getByTestId('newtoken-open-confirm'));

    const input = screen.getByRole('textbox');

    // Wrong name — confirm button must be disabled.
    fireEvent.change(input, { target: { value: 'wrong-name' } });
    expect(screen.getByTestId('confirm-dialog-confirm')).toBeDisabled();
    expect(API.post).not.toHaveBeenCalled();

    // Partial match — still disabled.
    fireEvent.change(input, { target: { value: 'exact' } });
    expect(screen.getByTestId('confirm-dialog-confirm')).toBeDisabled();
    expect(API.post).not.toHaveBeenCalled();
  });

  // 4. newChannel / incident / retry wizards retain WIPBanner.
  it('shows WIPBanner on newChannel/incident/retry wizards', () => {
    render(<HFFlows />);

    // newChannel is default tab — WIPBanner should already be present.
    expect(screen.queryAllByTestId('wip-newChannel').length).toBe(0);
    // WIPBanner renders a container; check by text content instead.
    const allText = document.body.textContent;
    expect(allText).toMatch(/wired in v3/);

    // Switch to incident.
    fireEvent.click(screen.getByTestId('flows-tab-incident'));
    expect(document.body.textContent).toMatch(/wired in v3/);

    // Switch to retry.
    fireEvent.click(screen.getByTestId('flows-tab-retry'));
    expect(document.body.textContent).toMatch(/wired in v3/);
  });

  // 5. Incident action buttons are disabled scope-cut with tooltips.
  it('incident action buttons are disabled with scope-cut tooltips', () => {
    render(<HFFlows />);
    fireEvent.click(screen.getByTestId('flows-tab-incident'));

    const silenceBtn = screen.getByTestId('incident-silence-btn');
    expect(silenceBtn.disabled).toBe(true);
    expect(silenceBtn.title).toMatch(/deferred to v3/);

    const statusBtn = screen.getByTestId('incident-status-update-btn');
    expect(statusBtn.disabled).toBe(true);
    expect(statusBtn.title).toMatch(/deferred to v3/);

    const resolveBtn = screen.getByTestId('incident-resolve-btn');
    expect(resolveBtn.disabled).toBe(true);
    expect(resolveBtn.title).toMatch(/deferred to v3/);

    // "disable channel" is a link navigating to the channel page.
    const disableLink = screen.getByTestId('incident-disable-channel-btn');
    expect(disableLink.tagName.toLowerCase()).toBe('a');
    expect(disableLink.href).toMatch(/channel/);
  });
});
