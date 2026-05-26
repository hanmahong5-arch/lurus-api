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

// β5: render-only smoke tests for static / illustrative v2 pages.
// These pages carry no backend API calls; we assert they mount without
// throwing and render at least one DOM node.

import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';

// Stub HFShell so static pages don't drag in the full shell dependency tree.
vi.mock('../../components/hifi/HFShell', () => ({
  default: ({ children, actions }) =>
    React.createElement(
      'div',
      { 'data-testid': 'hf-shell' },
      actions,
      children,
    ),
}));

// AccountDisabled uses Semi UI — stub the used components.
vi.mock('@douyinfe/semi-ui', () => {
  const Empty = ({ title, description, children }) =>
    React.createElement(
      'div',
      { 'data-testid': 'semi-empty' },
      title,
      description,
      children,
    );
  const Button = ({ children, onClick }) =>
    React.createElement('button', { onClick }, children);
  const Typography = {
    Title: ({ children }) => React.createElement('h4', null, children),
    Text: ({ children }) => React.createElement('span', null, children),
  };
  return { Empty, Button, Typography };
});

// AccountDisabled uses Semi illustrations — stub with empty divs.
vi.mock('@douyinfe/semi-illustrations', () => ({
  IllustrationNoAccess: () =>
    React.createElement('div', { 'data-testid': 'illus' }),
  IllustrationNoAccessDark: () =>
    React.createElement('div', { 'data-testid': 'illus-dark' }),
}));

// react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k) => k }),
}));

// Variants uses TweaksPanel from hifi — stub all exported components.
vi.mock('../../components/hifi/TweaksPanel', () => ({
  useTweaks: (defaults) => {
    // eslint-disable-next-line react-hooks/rules-of-hooks
    const [state, setState] = React.useState(defaults);
    return [state, setState];
  },
  TweaksPanel: ({ children }) =>
    React.createElement('div', { 'data-testid': 'tweaks-panel' }, children),
  TweakSection: ({ children }) => React.createElement('div', null, children),
  TweakRadio: ({ children }) => React.createElement('div', null, children),
  TweakColor: ({ children }) => React.createElement('div', null, children),
  TweakToggle: ({ children }) => React.createElement('div', null, children),
  TweakSlider: ({ children }) => React.createElement('div', null, children),
  TweakSelect: ({ children }) => React.createElement('div', null, children),
}));

// ─── imports after mocks ──────────────────────────────────────────────────────

import AccountDisabled from './AccountDisabled/index';
import CommandPalette from './CommandPalette/index';
import DesignSystem from './DesignSystem/index';
import States from './States/index';
import Variants from './Variants/index';

// ─── tests ───────────────────────────────────────────────────────────────────

describe('AccountDisabled page', () => {
  it('renders without error', () => {
    const { container } = render(React.createElement(AccountDisabled));
    expect(container.firstChild).toBeTruthy();
  });

  it('contains contact admin CTA', () => {
    const { getByText } = render(React.createElement(AccountDisabled));
    expect(getByText('联系管理员')).toBeTruthy();
  });
});

describe('CommandPalette page', () => {
  it('renders without error', () => {
    const { container } = render(React.createElement(CommandPalette));
    expect(container.firstChild).toBeTruthy();
  });

  it('renders the shell wrapper', () => {
    const { getByTestId } = render(React.createElement(CommandPalette));
    expect(getByTestId('hf-shell')).toBeTruthy();
  });
});

describe('DesignSystem page', () => {
  it('renders without error', () => {
    const { container } = render(React.createElement(DesignSystem));
    expect(container.firstChild).toBeTruthy();
  });
});

describe('States page', () => {
  it('renders without error', () => {
    const { container } = render(React.createElement(States));
    expect(container.firstChild).toBeTruthy();
  });

  it('renders the shell wrapper', () => {
    const { getByTestId } = render(React.createElement(States));
    expect(getByTestId('hf-shell')).toBeTruthy();
  });
});

describe('Variants page', () => {
  it('renders without error', () => {
    const { container } = render(React.createElement(Variants));
    expect(container.firstChild).toBeTruthy();
  });

  it('renders the shell wrapper', () => {
    const { getByTestId } = render(React.createElement(Variants));
    expect(getByTestId('hf-shell')).toBeTruthy();
  });
});
