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
  const { t } = useTranslation();
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
          account · suspended
        </div>
        <h1 className='display' style={{ fontSize: 28, margin: 0 }}>
          {t('账户已被禁用')}
        </h1>
        <div className='sub muted' style={{ marginTop: 10, fontSize: 13 }}>
          {t(
            '您的账户当前处于禁用状态，无法访问此服务。如需恢复访问，请联系系统管理员。',
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
            {t('联系管理员')}
          </button>
          <button
            type='button'
            className='btn'
            onClick={() => {
              window.location.href = '/api/v2/auth/zita-logout';
            }}
          >
            {t('切换账号')}
          </button>
        </div>
      </div>
    </div>
  );
};

export default AccountDisabled;
