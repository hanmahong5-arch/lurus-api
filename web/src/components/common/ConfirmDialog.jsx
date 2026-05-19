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

import React, { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Input, Modal } from '@douyinfe/semi-ui';

// Tier 1.3 (2026-05-19) — ConfirmDialog replaces window.confirm() at the
// three destructive surfaces in the v2 console (token revoke / rotate,
// channel delete, tenant enable/disable/suspend).
//
// The plain confirm() popup made it too easy to nuke prod tokens by
// reflex. Tier 1 customer-experience plan classified this as pain
// point #2 ("我不敢按这个按钮").
//
// Required-typed-confirmation pattern (e.g. type the resource name to
// arm the Confirm button) gives the user a forcing function without
// being patronising — they CAN go through, but only after a moment of
// attention. The pattern is borrowed from GitHub's repo-delete dialog
// and Stripe's account-close flow, both validated on similar audiences.
//
// Behaviour contract:
//
//   - Confirm button is disabled until the input value === confirmText
//     (strict, case-sensitive equality — no trim, no fuzzy match).
//   - Esc cancels; Enter (while input focused + matched) confirms.
//   - onConfirm may return a Promise — during pending the Confirm
//     button shows loading, Cancel + close-X are disabled so a
//     double-click can't trigger a second mutation.
//   - Input autoFocus on open; Semi Modal handles tab-trap.

const ConfirmDialog = ({
  visible,
  title,
  consequenceList = [],
  confirmText,
  confirmButtonText,
  confirmButtonType = 'danger',
  onConfirm,
  onCancel,
}) => {
  const { t } = useTranslation();
  const [inputValue, setInputValue] = useState('');
  const [pending, setPending] = useState(false);
  const inputRef = useRef(null);

  // Reset the typed value every time the dialog opens. Without this a
  // user who hit Cancel after typing "alpha" would find the field still
  // primed when reopening the dialog for a DIFFERENT resource.
  useEffect(() => {
    if (visible) {
      setInputValue('');
      setPending(false);
    }
  }, [visible]);

  const armed = inputValue === confirmText && !pending;

  const handleConfirm = async () => {
    if (!armed) return;
    setPending(true);
    try {
      const ret = onConfirm();
      if (ret && typeof ret.then === 'function') {
        await ret;
      }
    } finally {
      // The caller is responsible for closing the dialog on success.
      // We only flip pending back so a failed onConfirm leaves the
      // button armed for retry rather than stuck in loading.
      setPending(false);
    }
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter' && armed) {
      e.preventDefault();
      handleConfirm();
    }
  };

  return (
    <Modal
      visible={visible}
      title={title}
      onCancel={pending ? undefined : onCancel}
      closable={!pending}
      maskClosable={!pending}
      footer={
        <>
          <Button onClick={onCancel} disabled={pending}>
            {t('取消')}
          </Button>
          <Button
            type={confirmButtonType}
            theme='solid'
            disabled={!armed}
            loading={pending}
            onClick={handleConfirm}
            data-testid='confirm-dialog-confirm'
          >
            {confirmButtonText || t('删除')}
          </Button>
        </>
      }
      width={460}
    >
      {consequenceList.length > 0 && (
        <ul
          data-testid='confirm-dialog-consequences'
          style={{
            margin: '0 0 14px',
            padding: '10px 12px 10px 28px',
            borderLeft: '3px solid var(--semi-color-danger, #fa4d56)',
            background: 'var(--semi-color-danger-light-default, #fff2f1)',
            color: 'var(--semi-color-danger, #fa4d56)',
            fontSize: 13,
            lineHeight: 1.6,
            listStyle: 'disc',
          }}
        >
          {consequenceList.map((line, i) => (
            <li key={i}>{line}</li>
          ))}
        </ul>
      )}
      <div style={{ marginBottom: 6, fontSize: 13 }}>
        {t('请输入 {{name}} 以确认操作:', { name: confirmText })}
      </div>
      <Input
        ref={inputRef}
        autoFocus
        value={inputValue}
        onChange={setInputValue}
        onKeyDown={handleKeyDown}
        disabled={pending}
        data-testid='confirm-dialog-input'
        placeholder={confirmText}
      />
    </Modal>
  );
};

export default ConfirmDialog;
