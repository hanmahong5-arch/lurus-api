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
import React, { useCallback, useEffect, useRef, useState } from 'react';
import HFShell from '../../../components/hifi/HFShell';
import ConfirmDialog from '../../../components/common/ConfirmDialog';
import { API, showError, showSuccess } from '../../../helpers';

// Reads tenant slug from localStorage — same pattern as Token/Channel/Models pages.
const useTenantSlug = () => {
  const [slug, setSlug] = useState('default');
  useEffect(() => {
    try {
      const s = localStorage.getItem('tenant_slug');
      if (s) setSlug(s);
    } catch (_) {}
  }, []);
  return slug;
};

const fmtTime = (ts) => {
  if (!ts || ts === 0) return '—';
  return new Date(ts * 1000).toLocaleString();
};

const statusLabel = (status) => {
  if (status === 1) return '可用';
  if (status === 3) return '已使用';
  return '禁用';
};

const statusClass = (status) => {
  if (status === 1) return 'tag ok';
  if (status === 3) return 'tag';
  return 'tag';
};

// ─── Create modal ─────────────────────────────────────────────────────────────

const CreateModal = ({ tenantSlug, onCreated, onClose }) => {
  const [form, setForm] = useState({ name: '', count: 1, quota: 500000 });
  const [saving, setSaving] = useState(false);
  const nameRef = useRef(null);

  useEffect(() => {
    nameRef.current?.focus();
  }, []);

  const submit = async (e) => {
    e.preventDefault();
    if (!form.name.trim()) return;
    setSaving(true);
    try {
      const res = await API.post(`/api/v2/${tenantSlug}/redemptions`, {
        name: form.name.trim(),
        count: Number(form.count),
        quota: Number(form.quota),
      });
      if (res?.data?.success) {
        const codes = res.data.data.codes ?? [];
        showSuccess('兑换码生成成功');
        onCreated(codes);
      }
    } catch (_) {
      // error toast handled by API interceptor
    } finally {
      setSaving(false);
    }
  };

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.45)',
        zIndex: 500,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
      onClick={(e) => e.target === e.currentTarget && onClose()}
    >
      <form
        onSubmit={submit}
        style={{
          background: 'var(--hf-paper)',
          border: '1px solid var(--hf-rule)',
          borderRadius: 4,
          padding: 28,
          width: 400,
          display: 'flex',
          flexDirection: 'column',
          gap: 14,
        }}
      >
        <div className='strong' style={{ fontSize: 15 }}>
          新建兑换码
        </div>

        <label style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
          <span className='lbl'>兑换码名称 *</span>
          <input
            ref={nameRef}
            style={{
              fontFamily: 'var(--hf-mono)',
              fontSize: 12,
              padding: '5px 8px',
              border: '1px solid var(--hf-rule)',
              background: 'var(--hf-sunken)',
              color: 'var(--hf-ink)',
              borderRadius: 2,
              outline: 'none',
              width: '100%',
            }}
            placeholder='例如：促销活动2025'
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            required
            data-testid='redemption-name-input'
          />
        </label>

        <label style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
          <span className='lbl'>生成数量（最多100）</span>
          <input
            style={{
              fontFamily: 'var(--hf-mono)',
              fontSize: 12,
              padding: '5px 8px',
              border: '1px solid var(--hf-rule)',
              background: 'var(--hf-sunken)',
              color: 'var(--hf-ink)',
              borderRadius: 2,
              outline: 'none',
              width: '100%',
            }}
            type='number'
            min='1'
            max='100'
            value={form.count}
            onChange={(e) => setForm((f) => ({ ...f, count: e.target.value }))}
            data-testid='redemption-count-input'
          />
        </label>

        <label style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
          <span className='lbl'>额度（quota 单位）</span>
          <input
            style={{
              fontFamily: 'var(--hf-mono)',
              fontSize: 12,
              padding: '5px 8px',
              border: '1px solid var(--hf-rule)',
              background: 'var(--hf-sunken)',
              color: 'var(--hf-ink)',
              borderRadius: 2,
              outline: 'none',
              width: '100%',
            }}
            type='number'
            min='1'
            value={form.quota}
            onChange={(e) => setForm((f) => ({ ...f, quota: e.target.value }))}
            data-testid='redemption-quota-input'
          />
        </label>

        <div
          style={{
            display: 'flex',
            gap: 8,
            justifyContent: 'flex-end',
            marginTop: 4,
          }}
        >
          <button type='button' className='btn ghost' onClick={onClose}>
            取消
          </button>
          <button
            type='submit'
            className='btn primary'
            disabled={saving}
            data-testid='redemption-create-submit'
          >
            {saving ? '生成中…' : '生成兑换码'}
          </button>
        </div>
      </form>
    </div>
  );
};

// ─── Newly created keys banner ─────────────────────────────────────────────────

const CreatedKeysBanner = ({ codes, onClose }) => {
  if (!codes || codes.length === 0) return null;
  return (
    <div
      style={{
        marginBottom: 20,
        padding: '14px 16px',
        border: '1px solid var(--hf-ok)',
        background: 'rgba(31,122,79,0.06)',
        borderRadius: 4,
      }}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: 10,
        }}
      >
        <span
          className='strong'
          style={{ fontSize: 12, color: 'var(--hf-ok)' }}
        >
          生成成功 — 请立即复制，关闭后将无法再查看原始密钥
        </span>
        <button
          type='button'
          className='btn ghost sm'
          onClick={onClose}
          data-testid='redemption-keys-close'
        >
          ✕
        </button>
      </div>
      <div
        style={{ display: 'flex', flexDirection: 'column', gap: 6 }}
        data-testid='redemption-keys-list'
      >
        {codes.map((c) => (
          <div
            key={c.id ?? c.key}
            style={{ display: 'flex', alignItems: 'center', gap: 10 }}
          >
            <span className='lbl' style={{ minWidth: 80 }}>
              {c.name}
            </span>
            <code
              className='mono'
              style={{ fontSize: 11, flex: 1, wordBreak: 'break-all' }}
            >
              {c.key}
            </code>
          </div>
        ))}
      </div>
    </div>
  );
};

// ─── Main page ────────────────────────────────────────────────────────────────

const HFRedemption = () => {
  const tenantSlug = useTenantSlug();

  const [redemptions, setRedemptions] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newCodes, setNewCodes] = useState(null); // codes shown in banner after create
  const [deleteTarget, setDeleteTarget] = useState(null); // { id, name }

  const fetchRedemptions = useCallback(async () => {
    if (!tenantSlug || tenantSlug === 'default') return;
    setLoading(true);
    try {
      const res = await API.get(
        `/api/v2/${tenantSlug}/redemptions?page=1&page_size=50`,
      );
      if (res?.data?.success) {
        const d = res.data.data;
        setRedemptions(d.redemptions ?? []);
        setTotal(d.total ?? 0);
      }
    } catch (_) {
      // error toast handled by API interceptor
    } finally {
      setLoading(false);
    }
  }, [tenantSlug]);

  useEffect(() => {
    fetchRedemptions();
  }, [fetchRedemptions]);

  const handleCreated = async (codes) => {
    setCreating(false);
    setNewCodes(codes);
    await fetchRedemptions();
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      const res = await API.delete(
        `/api/v2/${tenantSlug}/redemptions/${deleteTarget.id}`,
      );
      if (res?.data?.success) {
        showSuccess('兑换码已删除');
        setDeleteTarget(null);
        await fetchRedemptions();
      }
    } catch (_) {
      // error toast handled by API interceptor
    }
  };

  return (
    <HFShell
      active='redemption'
      crumbs={['管理', '兑换码']}
      actions={
        <>
          {loading ? (
            <span className='muted mono' style={{ fontSize: 11 }}>
              加载中…
            </span>
          ) : (
            <span className='muted mono' style={{ fontSize: 11 }}>
              共 {total} 条
            </span>
          )}
          <button
            type='button'
            className='btn primary'
            onClick={() => setCreating(true)}
            data-testid='redemption-create-btn'
          >
            + 新建兑换码
          </button>
        </>
      }
    >
      <div style={{ padding: 24 }}>
        <CreatedKeysBanner codes={newCodes} onClose={() => setNewCodes(null)} />

        {!loading && redemptions.length === 0 && (
          <div
            className='muted'
            style={{ fontSize: 13, padding: '40px 0' }}
            data-testid='redemption-empty'
          >
            暂无兑换码。点击「新建兑换码」生成。
          </div>
        )}

        {redemptions.length > 0 && (
          <table
            style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}
          >
            <thead>
              <tr style={{ borderBottom: '1px solid var(--hf-rule)' }}>
                {[
                  'key',
                  '名称',
                  '额度',
                  '状态',
                  '创建时间',
                  '到期时间',
                  '使用者',
                  '操作',
                ].map((h) => (
                  <th
                    key={h}
                    className='lbl'
                    style={{ padding: '8px 10px', textAlign: 'left' }}
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {redemptions.map((r) => (
                <tr
                  key={r.id}
                  style={{ borderBottom: '1px solid var(--hf-rule)' }}
                  data-testid={`redemption-row-${r.id}`}
                >
                  <td
                    className='mono'
                    style={{ padding: '8px 10px', fontSize: 11 }}
                  >
                    {r.key}
                  </td>
                  <td style={{ padding: '8px 10px' }}>{r.name}</td>
                  <td className='mono' style={{ padding: '8px 10px' }}>
                    {r.quota}
                  </td>
                  <td style={{ padding: '8px 10px' }}>
                    <span className={statusClass(r.status)}>
                      {statusLabel(r.status)}
                    </span>
                  </td>
                  <td className='mono' style={{ padding: '8px 10px' }}>
                    {fmtTime(r.created_time)}
                  </td>
                  <td className='mono' style={{ padding: '8px 10px' }}>
                    {r.expired_time ? fmtTime(r.expired_time) : '永不'}
                  </td>
                  <td className='mono' style={{ padding: '8px 10px' }}>
                    {r.used_user_id ? `#${r.used_user_id}` : '—'}
                  </td>
                  <td style={{ padding: '8px 10px' }}>
                    <button
                      type='button'
                      className='btn ghost sm'
                      style={{
                        color: 'var(--hf-err)',
                        borderColor: 'var(--hf-err)',
                      }}
                      onClick={() =>
                        setDeleteTarget({ id: r.id, name: r.name })
                      }
                      data-testid={`redemption-delete-btn-${r.id}`}
                    >
                      删除
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {creating && (
        <CreateModal
          tenantSlug={tenantSlug}
          onCreated={handleCreated}
          onClose={() => setCreating(false)}
        />
      )}

      <ConfirmDialog
        visible={!!deleteTarget}
        title={`删除兑换码 "${deleteTarget?.name ?? ''}"?`}
        consequenceList={['兑换码将永久删除，此操作无法撤销']}
        confirmText={deleteTarget?.name ?? ''}
        confirmButtonText='删除'
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </HFShell>
  );
};

export default HFRedemption;
