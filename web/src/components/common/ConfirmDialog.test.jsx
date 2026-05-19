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

// Mock react-i18next — returns the key with simple {{var}} interpolation.
// Avoids dragging in the full i18n config which transitively imports
// Semi-UI internals that break under jsdom (lottie-web → canvas).
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key, vars) =>
      vars
        ? Object.entries(vars).reduce(
            (s, [k, v]) => s.replace(new RegExp(`{{\\s*${k}\\s*}}`, 'g'), v),
            key,
          )
        : key,
    i18n: { language: 'en' },
  }),
  Trans: ({ children }) => children,
  initReactI18next: { type: '3rdParty', init: () => {} },
}));

// Mock Semi UI primitives with minimal shims — the component contract under
// test is "input matches confirmText → Confirm is armed + onConfirm fires";
// that's independent of Semi's portal / animation / a11y layers, and
// loading the real package breaks under jsdom (lottie-web canvas error).
vi.mock('@douyinfe/semi-ui', () => ({
  Modal: ({ visible, title, footer, children, onCancel }) =>
    visible
      ? React.createElement(
          'div',
          { 'data-testid': 'mock-modal', role: 'dialog' },
          React.createElement('div', null, title),
          children,
          footer,
          React.createElement(
            'button',
            {
              type: 'button',
              'data-testid': 'mock-modal-close',
              onClick: onCancel,
            },
            'close',
          ),
        )
      : null,
  Button: ({ disabled, loading, onClick, children, ...rest }) =>
    React.createElement(
      'button',
      {
        type: 'button',
        disabled: disabled || loading,
        onClick,
        ...rest,
      },
      loading ? 'loading…' : children,
    ),
  Input: React.forwardRef(
    ({ value, onChange, onKeyDown, disabled, ...rest }, ref) =>
      React.createElement('input', {
        ref,
        value,
        disabled,
        onChange: (e) => onChange?.(e.target.value),
        onKeyDown,
        ...rest,
      }),
  ),
}));

import ConfirmDialog from './ConfirmDialog';

const CONFIRM_TEXT = 'production-key';

const baseProps = () => ({
  visible: true,
  title: 'Revoke token "production-key"?',
  consequenceList: ['Will stop working immediately', 'Cannot be undone'],
  confirmText: CONFIRM_TEXT,
  onConfirm: vi.fn(),
  onCancel: vi.fn(),
});

// Semi Modal portals its children into a node under document.body. Helpers
// below all query document.body to find them — no need to wrap render().
const getInput = () => screen.getByTestId('confirm-dialog-input');
const getConfirmButton = () => screen.getByTestId('confirm-dialog-confirm');

beforeEach(() => {
  // Semi Modal injects portals into document.body; isolate per-test.
  document.body.innerHTML = '';
});

describe('ConfirmDialog', () => {
  // 1. Input autofocuses on open so the user can start typing immediately
  //    without grabbing the mouse — half the point of the typed-confirm UX.
  it('autoFocuses the input on open', async () => {
    render(<ConfirmDialog {...baseProps()} />);
    await waitFor(() => {
      expect(document.activeElement).toBe(getInput());
    });
  });

  // 2. Confirm button is disabled at mount (input is empty, no match yet).
  it('confirm button is disabled on initial render', () => {
    render(<ConfirmDialog {...baseProps()} />);
    expect(getConfirmButton()).toBeDisabled();
  });

  // 3. Typing a wrong name keeps the confirm button disabled — strict
  //    case-sensitive equality, not a substring or fuzzy match.
  it('keeps confirm disabled while typed text does not match', () => {
    render(<ConfirmDialog {...baseProps()} />);
    fireEvent.change(getInput(), { target: { value: 'PRODUCTION-KEY' } });
    expect(getConfirmButton()).toBeDisabled();

    fireEvent.change(getInput(), { target: { value: 'production-ke' } });
    expect(getConfirmButton()).toBeDisabled();
  });

  // 4. Typing the exact confirmText arms the confirm button.
  it('enables confirm when typed text exactly equals confirmText', () => {
    render(<ConfirmDialog {...baseProps()} />);
    fireEvent.change(getInput(), { target: { value: CONFIRM_TEXT } });
    expect(getConfirmButton()).not.toBeDisabled();
  });

  // 5. Cancel button (or the modal close X — both wired to onCancel) fires
  //    onCancel exactly once. Real Esc-key handling is owned by Semi
  //    Modal and not unit-tested here; the contract this test guards is
  //    "every cancel path → onCancel".
  it('Cancel button triggers onCancel', () => {
    const props = baseProps();
    render(<ConfirmDialog {...props} />);
    fireEvent.click(screen.getByTestId('mock-modal-close'));
    expect(props.onCancel).toHaveBeenCalledTimes(1);
  });

  // 6. Enter on the input while armed triggers onConfirm exactly once.
  it('Enter confirms when armed', () => {
    const props = baseProps();
    render(<ConfirmDialog {...props} />);
    fireEvent.change(getInput(), { target: { value: CONFIRM_TEXT } });
    fireEvent.keyDown(getInput(), { key: 'Enter' });
    expect(props.onConfirm).toHaveBeenCalledTimes(1);
  });

  // 7. While onConfirm's Promise is pending, the confirm button shows
  //    loading (and is no longer click-armed) so a second click cannot
  //    fire a second mutation. Resolve to release the await.
  it('shows loading and blocks re-fire while onConfirm is pending', async () => {
    let resolveFn;
    const pendingPromise = new Promise((res) => {
      resolveFn = res;
    });
    const props = {
      ...baseProps(),
      onConfirm: vi.fn(() => pendingPromise),
    };
    render(<ConfirmDialog {...props} />);
    fireEvent.change(getInput(), { target: { value: CONFIRM_TEXT } });

    fireEvent.click(getConfirmButton());
    // Second click while pending — should be absorbed by `!armed`.
    fireEvent.click(getConfirmButton());

    expect(props.onConfirm).toHaveBeenCalledTimes(1);

    // Release the promise and wait for the loading flag to clear.
    resolveFn();
    await waitFor(() => {
      // Once pending flips back, the button is armed again for retry.
      expect(getConfirmButton()).not.toBeDisabled();
    });
  });
});
