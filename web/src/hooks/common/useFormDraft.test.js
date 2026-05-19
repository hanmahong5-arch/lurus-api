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
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';

import {
  useFormDraft,
  buildFullKey,
  clearAllDrafts,
  DRAFT_NAMESPACE_PREFIX,
  DEFAULT_DEBOUNCE_MS,
} from './useFormDraft';

const USER_ID = '42';
const KEY = 'tenant-credit-pool-form';
const fullKey = `${DRAFT_NAMESPACE_PREFIX}${USER_ID}:${KEY}`;

beforeEach(() => {
  window.localStorage.clear();
  window.localStorage.setItem('user', JSON.stringify({ id: USER_ID }));
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useFormDraft', () => {
  // 1. After setDraft + debounce window, the storage envelope is written
  //    once with the latest value, wrapped with version + timestamp.
  it('persists to localStorage after the debounce window', () => {
    const initial = { amount: '', reason: '' };
    const { result } = renderHook(() => useFormDraft(KEY, initial));

    act(() => {
      result.current[1]({ amount: '100', reason: 'q1 topup' });
    });
    // Pre-debounce: nothing written yet.
    expect(window.localStorage.getItem(fullKey)).toBeNull();

    act(() => {
      vi.advanceTimersByTime(DEFAULT_DEBOUNCE_MS);
    });

    const raw = window.localStorage.getItem(fullKey);
    expect(raw).not.toBeNull();
    const env = JSON.parse(raw);
    expect(env.v).toBe(1);
    expect(env.d).toEqual({ amount: '100', reason: 'q1 topup' });
    expect(typeof env.t).toBe('number');
  });

  // 2. Rapid successive setDraft calls collapse into a SINGLE write —
  //    the persistence layer must not pin localStorage on every keystroke.
  it('debounces multiple setDraft calls into a single write', () => {
    const setItemSpy = vi.spyOn(Storage.prototype, 'setItem');
    const { result } = renderHook(() => useFormDraft(KEY, { amount: '' }));

    act(() => {
      result.current[1]({ amount: '1' });
      result.current[1]({ amount: '12' });
      result.current[1]({ amount: '123' });
      result.current[1]({ amount: '1234' });
    });

    // Capture only the writes targeting OUR key — beforeEach wrote 'user'.
    const drainWrites = () =>
      setItemSpy.mock.calls.filter((c) => c[0] === fullKey).length;

    expect(drainWrites()).toBe(0);
    act(() => {
      vi.advanceTimersByTime(DEFAULT_DEBOUNCE_MS);
    });
    expect(drainWrites()).toBe(1);

    const env = JSON.parse(window.localStorage.getItem(fullKey));
    expect(env.d.amount).toBe('1234');
    setItemSpy.mockRestore();
  });

  // 3. Remounting the hook restores the saved draft and surfaces
  //    restoredFromDraft=true so the caller can render a recovery banner.
  it('restores draft on remount and sets restoredFromDraft', () => {
    // Seed a saved draft directly.
    window.localStorage.setItem(
      fullKey,
      JSON.stringify({ v: 1, d: { amount: '500' }, t: Date.now() }),
    );

    const { result } = renderHook(() => useFormDraft(KEY, { amount: '' }));
    expect(result.current[0]).toEqual({ amount: '500' });
    expect(result.current[4]).toBe(true);
  });

  // 4. clear() removes the storage entry, resets the draft to initial,
  //    and flips restoredFromDraft back to false.
  it('clear() wipes storage and resets state', () => {
    window.localStorage.setItem(
      fullKey,
      JSON.stringify({ v: 1, d: { amount: '99' }, t: Date.now() }),
    );

    const { result } = renderHook(() => useFormDraft(KEY, { amount: '' }));
    expect(result.current[4]).toBe(true);

    act(() => {
      result.current[2](); // clear
    });

    expect(window.localStorage.getItem(fullKey)).toBeNull();
    expect(result.current[0]).toEqual({ amount: '' });
    expect(result.current[4]).toBe(false);
  });

  // 5. A schemaVersion bump invalidates older envelopes silently. The
  //    hook does NOT throw or warn — it just hands back initialValue.
  it('discards envelopes from an older schemaVersion', () => {
    window.localStorage.setItem(
      fullKey,
      JSON.stringify({ v: 1, d: { amount: 'old-shape' }, t: Date.now() }),
    );

    const { result } = renderHook(() =>
      useFormDraft(KEY, { amount: '' }, { schemaVersion: 2 }),
    );
    expect(result.current[0]).toEqual({ amount: '' });
    expect(result.current[4]).toBe(false);
  });

  // 6. Envelopes older than the TTL are discarded. We forge a timestamp
  //    8 days in the past (default TTL is 7 days).
  it('discards envelopes past the TTL', () => {
    const eightDaysAgo = Date.now() - 8 * 24 * 60 * 60 * 1000;
    window.localStorage.setItem(
      fullKey,
      JSON.stringify({ v: 1, d: { amount: 'stale' }, t: eightDaysAgo }),
    );

    const { result } = renderHook(() => useFormDraft(KEY, { amount: '' }));
    expect(result.current[0]).toEqual({ amount: '' });
    expect(result.current[4]).toBe(false);
    // And the expired slot was cleaned up.
    expect(window.localStorage.getItem(fullKey)).toBeNull();
  });

  // 7. restoredFromDraft is false when no draft existed at mount, and
  //    stays false after the first persist (only an actual cross-mount
  //    recovery flips it to true — Tier 1.4 contract).
  it('restoredFromDraft stays false for a fresh session', () => {
    const { result } = renderHook(() => useFormDraft(KEY, { amount: '' }));

    expect(result.current[4]).toBe(false);

    act(() => {
      result.current[1]({ amount: '99' });
      vi.advanceTimersByTime(DEFAULT_DEBOUNCE_MS);
    });

    // Still false — the user typed it themselves this session, no
    // recovery happened.
    expect(result.current[4]).toBe(false);
  });
});

describe('clearAllDrafts', () => {
  // Bonus: verifies the logout cleanup helper actually wipes everything
  // under the namespace (and only that namespace).
  it('removes only keys under DRAFT_NAMESPACE_PREFIX', () => {
    window.localStorage.setItem('user', JSON.stringify({ id: '42' }));
    window.localStorage.setItem('tenant_slug', 'acme');
    window.localStorage.setItem(buildFullKey('form-a', '42'), '{}');
    window.localStorage.setItem(buildFullKey('form-b', '42'), '{}');
    window.localStorage.setItem(buildFullKey('form-c', 'anon'), '{}');

    clearAllDrafts();

    expect(window.localStorage.getItem('user')).not.toBeNull();
    expect(window.localStorage.getItem('tenant_slug')).toBe('acme');
    expect(
      window.localStorage.getItem(buildFullKey('form-a', '42')),
    ).toBeNull();
    expect(
      window.localStorage.getItem(buildFullKey('form-b', '42')),
    ).toBeNull();
    expect(
      window.localStorage.getItem(buildFullKey('form-c', 'anon')),
    ).toBeNull();
  });
});
