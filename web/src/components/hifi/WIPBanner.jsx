import React from 'react';

/**
 * WIPBanner — explicit "not yet wired" marker for v2 hi-fi pages whose
 * content is still design-mock content. Surfaces honesty per §4.1 ⑥:
 * "markers present" vs "behavior verified" — this is the marker.
 *
 * Usage:
 *   <WIPBanner
 *     reason="No backend endpoint for X yet"
 *     todo="path/to/issue or planning artifact"
 *   />
 */
const WIPBanner = ({ reason, todo }) => (
  <div
    role='status'
    aria-label='work in progress'
    style={{
      margin: '14px 24px 0',
      padding: '10px 14px',
      border: '1px dashed var(--hf-warn, #c08a3e)',
      borderRadius: 2,
      background: 'rgba(192,138,62,0.08)',
      color: 'var(--hf-ink-2)',
      fontFamily: 'var(--hf-mono)',
      fontSize: 11,
      lineHeight: 1.55,
    }}
  >
    <div style={{ fontWeight: 600, marginBottom: 2, letterSpacing: '0.04em' }}>
      ⚠ WORK IN PROGRESS — content is design-mock only
    </div>
    {reason && (
      <div style={{ opacity: 0.85 }}>
        Reason: {reason}
      </div>
    )}
    {todo && (
      <div style={{ opacity: 0.7 }}>
        TODO: {todo}
      </div>
    )}
  </div>
);

export default WIPBanner;
