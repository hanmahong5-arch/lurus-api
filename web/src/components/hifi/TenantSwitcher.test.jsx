import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import TenantSwitcher from './TenantSwitcher';

const tenants = [
  { id: 'acme', name: 'acme · prod', mode: 'Reseller' },
  { id: 'beta', name: 'beta · stage', mode: 'EndUser' },
];

describe('TenantSwitcher — real-data path (Lane B 2026-05-18)', () => {
  it('renders provided tenantName + mode without demo placeholders', () => {
    render(
      <TenantSwitcher
        tenants={tenants}
        tenantName='acme · prod'
        mode='Reseller'
      />,
    );
    // Trigger shows the active tenant name.
    expect(screen.getByText('acme · prod')).toBeInTheDocument();
    // Mode pill renders the mode label uppercased — find "Reseller" specifically.
    expect(screen.getAllByText(/reseller/i).length).toBeGreaterThanOrEqual(1);
    // Demo "personal workspace" placeholder must not leak through.
    expect(screen.queryByText(/personal workspace/i)).toBeNull();
  });

  it('handles empty tenants list without crashing or showing demo data', () => {
    render(
      <TenantSwitcher
        tenants={[]}
        tenantName=''
        mode='Personal'
      />,
    );
    // No "acme" leak from removed DEMO_TENANTS.
    expect(screen.queryByText(/acme/i)).toBeNull();
    expect(screen.queryByText(/contoso/i)).toBeNull();
  });

  it('opens dropdown on trigger click and lists tenants by name', () => {
    render(
      <TenantSwitcher
        tenants={tenants}
        tenantName='acme · prod'
        mode='Reseller'
      />,
    );
    fireEvent.click(screen.getByRole('button'));

    // Both tenant names visible in the dropdown options.
    expect(screen.getAllByText('acme · prod').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('beta · stage')).toBeInTheDocument();
  });

  it('fires onSelect with tenant id when an option is clicked', () => {
    const onSelect = vi.fn();
    render(
      <TenantSwitcher
        tenants={tenants}
        tenantName='acme · prod'
        mode='Reseller'
        onSelect={onSelect}
      />,
    );
    fireEvent.click(screen.getByRole('button'));
    fireEvent.click(screen.getByText('beta · stage'));
    expect(onSelect).toHaveBeenCalledWith('beta', 'EndUser');
  });

  it('syncs to updated tenantName prop after first paint', () => {
    const { rerender } = render(
      <TenantSwitcher
        tenants={[]}
        tenantName=''
        mode='Personal'
      />,
    );
    // Simulate caller (HFShell) finishing /api/v2/admin/tenants fetch.
    rerender(
      <TenantSwitcher
        tenants={tenants}
        tenantName='acme · prod'
        mode='Reseller'
      />,
    );
    expect(screen.getByText('acme · prod')).toBeInTheDocument();
  });
});
