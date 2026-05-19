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
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import WIPBanner from './WIPBanner';

describe('WIPBanner', () => {
  it('renders the WIP marker text unconditionally', () => {
    render(<WIPBanner />);
    expect(screen.getByText(/work in progress/i)).toBeInTheDocument();
  });

  it('shows reason and todo when provided', () => {
    render(
      <WIPBanner
        reason='No backend endpoint for X'
        todo='See planning artifact Y'
      />,
    );
    expect(screen.getByText(/no backend endpoint for x/i)).toBeInTheDocument();
    expect(screen.getByText(/see planning artifact y/i)).toBeInTheDocument();
  });

  it('has role=status for accessibility', () => {
    render(<WIPBanner reason='r' />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('omits reason/todo lines when props absent', () => {
    render(<WIPBanner />);
    expect(screen.queryByText(/^reason:/i)).toBeNull();
    expect(screen.queryByText(/^todo:/i)).toBeNull();
  });
});
