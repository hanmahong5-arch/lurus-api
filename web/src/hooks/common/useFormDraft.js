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

import { useCallback, useEffect, useRef, useState } from 'react';

// Tier 1.4 (2026-05-19) — form-draft persistence to localStorage.
//
// Long admin / billing forms (CreditPoolDrawer, tenant create, channel
// edit) lost everything on accidental tab close or token expiry mid-fill.
// This hook adds debounced auto-save + per-user namespacing so a refresh
// recovers the in-progress state.

export const DRAFT_NAMESPACE_PREFIX = 'lurus-hub:draft:';
export const DEFAULT_DEBOUNCE_MS = 300;
export const DEFAULT_TTL_MS = 7 * 24 * 60 * 60 * 1000; // 7 days
export const DEFAULT_SCHEMA_VERSION = 1;

// readCurrentUserId pulls the live user id from localStorage.user (the
// same key the auth helpers use). The namespacing prefix uses this so
// switching accounts on the same browser never crosses drafts. Returns
// 'anon' for unauthenticated sessions.
function readCurrentUserId() {
  try {
    const raw = window.localStorage.getItem('user');
    if (!raw) return 'anon';
    const parsed = JSON.parse(raw);
    if (parsed && parsed.id != null) return String(parsed.id);
    return 'anon';
  } catch (_) {
    return 'anon';
  }
}

// buildFullKey is exported for test / debug — the storage layout is
// `lurus-hub:draft:{userId}:{storageKey}`. Stored values are JSON-encoded
// envelopes: `{ v: schemaVersion, d: draftValue, t: writeEpochMs }`.
export function buildFullKey(storageKey, userId) {
  const u = userId || readCurrentUserId();
  return `${DRAFT_NAMESPACE_PREFIX}${u}:${storageKey}`;
}

// clearAllDrafts removes every namespaced draft from localStorage. Wire
// this into the logout handler so a different account on the same
// browser never sees the previous user's drafts (defence in depth — the
// per-user namespacing already isolates by id, but logout-cleanup makes
// it impossible to recover even if the userId resolution drifts).
export function clearAllDrafts() {
  try {
    const keys = [];
    for (let i = 0; i < window.localStorage.length; i++) {
      const k = window.localStorage.key(i);
      if (k && k.startsWith(DRAFT_NAMESPACE_PREFIX)) keys.push(k);
    }
    keys.forEach((k) => window.localStorage.removeItem(k));
  } catch (_) {
    // ignore — private-mode / disabled storage
  }
}

// useFormDraft returns [draft, setDraft, clear, isDirty, restoredFromDraft].
//
// Behaviour contract:
//
//   - Mount: read storage; if the envelope's schema version matches AND
//     the value is < 7d old, restore it and set restoredFromDraft=true.
//     Stale or version-mismatched envelopes are discarded silently.
//   - setDraft(next): updates React state synchronously, persists to
//     storage after the debounce window. Identical to useState semantics
//     for the caller's purposes; the only difference is the side effect.
//   - clear(): removes the storage entry, resets draft to initialValue,
//     and flips restoredFromDraft to false. Call this from your submit-
//     success handler.
//   - restoredFromDraft: stays true until clear() is called OR the
//     component unmounts. The caller is expected to render a "Restored
//     from saved draft — [Discard]" banner so users aren't surprised by
//     prefilled fields.
//
// Options (all optional):
//   - debounceMs: write batching window (default 300ms)
//   - schemaVersion: bump when the draft shape changes incompatibly
//   - ttlMs: how long a draft stays valid (default 7 days)
//   - userId: override; useful for tests and SSR
//
// What this hook intentionally does NOT do (Tier 2):
//   - Cross-tab synchronisation via the storage event
//   - IndexedDB fallback (localStorage 5MB is enough for v1)
//   - Form-field-level conflict resolution (caller decides via the
//     Discard banner)
export function useFormDraft(storageKey, initialValue, options = {}) {
  const {
    debounceMs = DEFAULT_DEBOUNCE_MS,
    schemaVersion = DEFAULT_SCHEMA_VERSION,
    ttlMs = DEFAULT_TTL_MS,
    userId,
  } = options;

  // The full key is captured once at mount. Changing userId mid-life
  // would create a new draft slot — explicitly out of scope for v1; an
  // account switch should trigger logout → clearAllDrafts → reload.
  const fullKeyRef = useRef(buildFullKey(storageKey, userId));

  // Initial state — pull from storage if possible.
  const [draft, setDraftState] = useState(() => {
    try {
      const raw = window.localStorage.getItem(fullKeyRef.current);
      if (!raw) return initialValue;
      const envelope = JSON.parse(raw);
      if (!envelope || typeof envelope !== 'object') return initialValue;
      if (envelope.v !== schemaVersion) return initialValue;
      if (typeof envelope.t !== 'number' || Date.now() - envelope.t > ttlMs) {
        // expired — clean up so the slot doesn't accumulate stale state.
        window.localStorage.removeItem(fullKeyRef.current);
        return initialValue;
      }
      return envelope.d;
    } catch (_) {
      return initialValue;
    }
  });

  const [restoredFromDraft, setRestoredFromDraft] = useState(() => {
    try {
      const raw = window.localStorage.getItem(fullKeyRef.current);
      if (!raw) return false;
      const envelope = JSON.parse(raw);
      if (!envelope || envelope.v !== schemaVersion) return false;
      if (typeof envelope.t !== 'number' || Date.now() - envelope.t > ttlMs) {
        return false;
      }
      return true;
    } catch (_) {
      return false;
    }
  });

  // Track the initial value via ref so isDirty stays correct across
  // setDraft / clear cycles. We compare by JSON serialisation — good
  // enough for the form shapes we persist (plain objects of primitives).
  const initialRef = useRef(initialValue);
  const isDirty = JSON.stringify(draft) !== JSON.stringify(initialRef.current);

  // Debounce machinery — a single trailing timer per setDraft burst so
  // a fast-typing user doesn't pin localStorage on every keystroke.
  const debounceTimerRef = useRef(null);
  const writePending = useRef(false);

  const persist = useCallback(
    (value) => {
      try {
        const envelope = {
          v: schemaVersion,
          d: value,
          t: Date.now(),
        };
        window.localStorage.setItem(
          fullKeyRef.current,
          JSON.stringify(envelope),
        );
      } catch (_) {
        // ignore — private-mode / quota exceeded
      }
    },
    [schemaVersion],
  );

  const setDraft = useCallback(
    (nextOrUpdater) => {
      setDraftState((prev) => {
        const next =
          typeof nextOrUpdater === 'function'
            ? nextOrUpdater(prev)
            : nextOrUpdater;

        if (debounceTimerRef.current) {
          clearTimeout(debounceTimerRef.current);
        }
        writePending.current = true;
        debounceTimerRef.current = setTimeout(() => {
          persist(next);
          writePending.current = false;
          debounceTimerRef.current = null;
        }, debounceMs);

        return next;
      });
    },
    [debounceMs, persist],
  );

  const clear = useCallback(() => {
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
      debounceTimerRef.current = null;
      writePending.current = false;
    }
    try {
      window.localStorage.removeItem(fullKeyRef.current);
    } catch (_) {
      // ignore — private-mode
    }
    setDraftState(initialRef.current);
    setRestoredFromDraft(false);
  }, []);

  // Flush any pending write on unmount — otherwise a quick edit + nav
  // away loses the last keystroke.
  useEffect(() => {
    return () => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
        if (writePending.current) {
          // Best-effort sync persist using whatever the state currently
          // holds. We capture it via the React state by reading from a
          // ref we keep in sync below.
          persist(latestRef.current);
        }
        debounceTimerRef.current = null;
        writePending.current = false;
      }
    };
  }, [persist]);

  // Keep a ref of the latest draft for the unmount flush above.
  const latestRef = useRef(draft);
  latestRef.current = draft;

  return [draft, setDraft, clear, isDirty, restoredFromDraft];
}

export default useFormDraft;
