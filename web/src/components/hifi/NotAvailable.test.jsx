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
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';

import NotAvailable from './NotAvailable';

describe('NotAvailable cell', () => {
  it('renders an honest "n/a" with the reason surfaced as a title', () => {
    render(<NotAvailable reason='no metrics backend: QPS not aggregated' />);
    const el = screen.getByTestId('na-cell');
    expect(el.textContent).toBe('n/a');
    expect(el.title).toBe('no metrics backend: QPS not aggregated');
    expect(el.getAttribute('data-na-reason')).toBe(
      'no metrics backend: QPS not aggregated',
    );
  });

  it('supports a custom label (e.g. an em-dash in numeric columns)', () => {
    render(<NotAvailable reason='r' label='—' />);
    expect(screen.getByTestId('na-cell').textContent).toBe('—');
  });
});
