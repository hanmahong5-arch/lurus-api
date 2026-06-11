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
import { useTranslation } from 'react-i18next';

// Terminal page reached when zita-bootstrap returns 403 (user.Status !=
// UserStatusEnabled). The visitor holds a valid platform identity but
// their newhub account is suspended/disabled — sending them back through
// /login would loop because bootstrap rejects again on every retry.
// Surface the state explicitly with a contact path instead.
//
// Intentionally does NOT mount HFShell: a suspended account must not render
// navigation or the tenant switcher. This is a minimal `.hf`-scoped error
// card — the `.hf` class is what activates the hi-fi CSS variables/shared
// classes (HFShell itself relies on the same `<div className='hf …'>` root).
const AccountDisabled = () => {
  const { t: tr } = useTranslation();
  return (
    <div
      className='hf'
      style={{
        minHeight: '100vh',
        display: 'grid',
        placeItems: 'center',
        background: 'var(--hf-bg)',
        padding: 24,
      }}
    >
      <div
        className='panel'
        style={{ maxWidth: 440, padding: '32px 28px', textAlign: 'center' }}
      >
        <div
          className='display acc'
          aria-hidden='true'
          style={{ fontSize: 40, lineHeight: 1, marginBottom: 14 }}
        >
          ⊘
        </div>
        <div className='lbl' style={{ marginBottom: 10 }}>
          {tr('console.account.badge', 'account · suspended')}
        </div>
        <h1 className='display' style={{ fontSize: 28, margin: 0 }}>
          {tr('console.account.title', 'Account disabled')}
        </h1>
        <div className='sub muted' style={{ marginTop: 10, fontSize: 13 }}>
          {tr(
            'console.account.body',
            'Your account is currently disabled and cannot access this service. To restore access, please contact the system administrator.',
          )}
        </div>
        <div
          style={{
            marginTop: 22,
            display: 'flex',
            gap: 10,
            justifyContent: 'center',
          }}
        >
          <button
            type='button'
            className='btn primary'
            onClick={() => {
              window.location.href = 'mailto:support@lurus.cn';
            }}
          >
            {tr('console.account.contact_admin', 'Contact administrator')}
          </button>
          <button
            type='button'
            className='btn'
            onClick={() => {
              window.location.href = '/api/v2/auth/zita-logout';
            }}
          >
            {tr('console.account.switch_account', 'Switch account')}
          </button>
        </div>
      </div>
    </div>
  );
};

export default AccountDisabled;
