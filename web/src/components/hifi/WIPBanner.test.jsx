import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import WIPBanner from './WIPBanner';

describe('WIPBanner', () => {
  it('renders the WIP marker text unconditionally', () => {
    render(<WIPBanner />);
    expect(
      screen.getByText(/work in progress/i),
    ).toBeInTheDocument();
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
