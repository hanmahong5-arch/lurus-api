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
import React, { useCallback, useRef, useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';

/*
 * TweaksPanel — adapted from design canvas tweaks-panel.jsx (2026-05-07).
 * Stripped of the parent-iframe postMessage protocol (only meaningful in
 * Claude Design preview); kept the visual controls + useTweaks state hook.
 *
 * Usage:
 *   const [t, setTweak] = useTweaks({ density: 'comfortable', accent: '#ff5d1f' });
 *   ...
 *   <TweaksPanel title="Tweaks">
 *     <TweakSection title="density">
 *       <TweakRadio value={t.density} onChange={(v) => setTweak('density', v)}
 *         options={[['compact', 'compact'], ['comfortable', 'comfy'], ['roomy', 'roomy']]} />
 *     </TweakSection>
 *     <TweakSection title="accent">
 *       <TweakColor value={t.accent} onChange={(v) => setTweak('accent', v)}
 *         options={['#ff5d1f', '#2d4a8a', '#1f7a4f']} />
 *     </TweakSection>
 *   </TweaksPanel>
 */

const TWEAKS_STYLE = `
  .twk-panel{position:fixed;right:16px;bottom:16px;z-index:2147483646;width:280px;
    max-height:calc(100vh - 32px);display:flex;flex-direction:column;
    background:rgba(250,249,247,.78);color:#29261b;
    -webkit-backdrop-filter:blur(24px) saturate(160%);backdrop-filter:blur(24px) saturate(160%);
    border:.5px solid rgba(255,255,255,.6);border-radius:14px;
    box-shadow:0 1px 0 rgba(255,255,255,.5) inset,0 12px 40px rgba(0,0,0,.18);
    font:11.5px/1.4 ui-sans-serif,system-ui,-apple-system,sans-serif;overflow:hidden}
  [data-theme='dark'] .twk-panel{background:rgba(20,18,15,.85);color:#f0ede4;border-color:rgba(255,255,255,.08)}
  .twk-hd{display:flex;align-items:center;justify-content:space-between;
    padding:10px 8px 10px 14px;cursor:move;user-select:none}
  .twk-hd b{font-size:12px;font-weight:600;letter-spacing:.01em}
  .twk-x{appearance:none;border:0;background:transparent;color:rgba(41,38,27,.55);
    width:22px;height:22px;border-radius:6px;cursor:pointer;font-size:13px;line-height:1}
  [data-theme='dark'] .twk-x{color:rgba(240,237,228,.6)}
  .twk-x:hover{background:rgba(0,0,0,.06)}
  .twk-body{padding:2px 14px 14px;display:flex;flex-direction:column;gap:10px;
    overflow-y:auto;overflow-x:hidden;min-height:0}
  .twk-row{display:flex;flex-direction:column;gap:5px}
  .twk-row-h{flex-direction:row;align-items:center;justify-content:space-between;gap:10px}
  .twk-lbl{display:flex;justify-content:space-between;align-items:baseline;
    color:rgba(41,38,27,.72)}
  [data-theme='dark'] .twk-lbl{color:rgba(240,237,228,.7)}
  .twk-lbl>span:first-child{font-weight:500}
  .twk-val{color:rgba(41,38,27,.5);font-variant-numeric:tabular-nums}
  .twk-sect{font-size:10px;font-weight:600;letter-spacing:.06em;text-transform:uppercase;
    color:rgba(41,38,27,.45);padding:10px 0 0}
  [data-theme='dark'] .twk-sect{color:rgba(240,237,228,.45)}
  .twk-sect:first-child{padding-top:0}
  .twk-seg{position:relative;display:flex;padding:2px;border-radius:8px;
    background:rgba(0,0,0,.06);user-select:none}
  [data-theme='dark'] .twk-seg{background:rgba(255,255,255,.06)}
  .twk-seg-thumb{position:absolute;top:2px;bottom:2px;border-radius:6px;
    background:rgba(255,255,255,.9);box-shadow:0 1px 2px rgba(0,0,0,.12);
    transition:left .15s cubic-bezier(.3,.7,.4,1),width .15s}
  [data-theme='dark'] .twk-seg-thumb{background:rgba(255,255,255,.18)}
  .twk-seg button{appearance:none;position:relative;z-index:1;flex:1;border:0;
    background:transparent;color:inherit;font:inherit;font-weight:500;min-height:22px;
    border-radius:6px;cursor:pointer;padding:4px 6px;line-height:1.2}
  .twk-toggle{position:relative;width:32px;height:18px;border:0;border-radius:999px;
    background:rgba(0,0,0,.15);transition:background .15s;cursor:pointer;padding:0}
  .twk-toggle[data-on="1"]{background:#34c759}
  .twk-toggle i{position:absolute;top:2px;left:2px;width:14px;height:14px;border-radius:50%;
    background:#fff;box-shadow:0 1px 2px rgba(0,0,0,.25);transition:transform .15s}
  .twk-toggle[data-on="1"] i{transform:translateX(14px)}
  .twk-chips{display:flex;gap:6px}
  .twk-chip{position:relative;appearance:none;flex:1;min-width:0;height:46px;
    padding:0;border:0;border-radius:6px;overflow:hidden;cursor:pointer;
    box-shadow:0 0 0 .5px rgba(0,0,0,.12),0 1px 2px rgba(0,0,0,.06);
    transition:transform .12s cubic-bezier(.3,.7,.4,1),box-shadow .12s}
  .twk-chip:hover{transform:translateY(-1px)}
  .twk-chip[data-on="1"]{box-shadow:0 0 0 1.5px rgba(0,0,0,.85),0 2px 6px rgba(0,0,0,.15)}
  .twk-chip svg{position:absolute;top:6px;left:6px;width:13px;height:13px;
    filter:drop-shadow(0 1px 1px rgba(0,0,0,.3))}
`;

let styleInjected = false;
const injectStyle = () => {
  if (styleInjected || typeof document === 'undefined') return;
  const el = document.createElement('style');
  el.setAttribute('data-tweaks-style', '1');
  el.textContent = TWEAKS_STYLE;
  document.head.appendChild(el);
  styleInjected = true;
};

export const useTweaks = (defaults) => {
  const [values, setValues] = useState(defaults);
  const setTweak = useCallback((keyOrEdits, val) => {
    const edits =
      typeof keyOrEdits === 'object' && keyOrEdits !== null
        ? keyOrEdits
        : { [keyOrEdits]: val };
    setValues((prev) => ({ ...prev, ...edits }));
  }, []);
  return [values, setTweak];
};

export const TweaksPanel = ({ title, children, defaultOpen = true }) => {
  const { t: tr } = useTranslation();
  const [open, setOpen] = useState(defaultOpen);
  const dragRef = useRef(null);
  const offsetRef = useRef({ x: 16, y: 16 });

  useEffect(() => {
    injectStyle();
  }, []);

  const shownTitle = title ?? tr('console.component.tweaks.title', 'Tweaks');

  if (!open) {
    return (
      <button
        type='button'
        onClick={() => setOpen(true)}
        style={{
          position: 'fixed',
          right: 16,
          bottom: 16,
          zIndex: 2147483646,
          height: 32,
          padding: '0 14px',
          border: 0,
          borderRadius: 16,
          background: 'rgba(0,0,0,0.85)',
          color: '#fff',
          fontFamily: 'ui-sans-serif,system-ui,sans-serif',
          fontSize: 11.5,
          fontWeight: 500,
          cursor: 'pointer',
          boxShadow: '0 4px 16px rgba(0,0,0,0.2)',
        }}
      >
        ⚙ {shownTitle}
      </button>
    );
  }

  const onDragStart = (e) => {
    const panel = dragRef.current;
    if (!panel) return;
    const r = panel.getBoundingClientRect();
    const sx = e.clientX;
    const sy = e.clientY;
    const startRight = window.innerWidth - r.right;
    const startBottom = window.innerHeight - r.bottom;
    const move = (ev) => {
      offsetRef.current = {
        x: Math.max(8, startRight - (ev.clientX - sx)),
        y: Math.max(8, startBottom - (ev.clientY - sy)),
      };
      panel.style.right = offsetRef.current.x + 'px';
      panel.style.bottom = offsetRef.current.y + 'px';
    };
    const up = () => {
      window.removeEventListener('mousemove', move);
      window.removeEventListener('mouseup', up);
    };
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', up);
  };

  return (
    <div
      ref={dragRef}
      className='twk-panel'
      style={{ right: offsetRef.current.x, bottom: offsetRef.current.y }}
    >
      <div className='twk-hd' onMouseDown={onDragStart}>
        <b>{shownTitle}</b>
        <button
          className='twk-x'
          aria-label={tr('console.component.tweaks.close_aria', 'Close tweaks')}
          onMouseDown={(e) => e.stopPropagation()}
          onClick={() => setOpen(false)}
        >
          ✕
        </button>
      </div>
      <div className='twk-body'>{children}</div>
    </div>
  );
};

export const TweakSection = ({ title, children }) => (
  <>
    <div className='twk-sect'>{title}</div>
    {children}
  </>
);

export const TweakRow = ({ label, value, children, inline = false }) => (
  <div className={inline ? 'twk-row twk-row-h' : 'twk-row'}>
    <div className='twk-lbl'>
      <span>{label}</span>
      {value != null && <span className='twk-val'>{value}</span>}
    </div>
    {children}
  </div>
);

export const TweakRadio = ({ label, value, options, onChange }) => {
  const opts = options.map((o) =>
    Array.isArray(o)
      ? { value: o[0], label: o[1] }
      : typeof o === 'object'
        ? o
        : { value: o, label: o },
  );
  const idx = Math.max(
    0,
    opts.findIndex((o) => o.value === value),
  );
  const n = opts.length;
  return (
    <TweakRow label={label}>
      <div role='radiogroup' className='twk-seg'>
        <div
          className='twk-seg-thumb'
          style={{
            left: `calc(2px + ${idx} * (100% - 4px) / ${n})`,
            width: `calc((100% - 4px) / ${n})`,
          }}
        />
        {opts.map((o) => (
          <button
            key={o.value}
            type='button'
            role='radio'
            aria-checked={o.value === value}
            onClick={() => onChange(o.value)}
          >
            {o.label}
          </button>
        ))}
      </div>
    </TweakRow>
  );
};

export const TweakSelect = ({ label, value, options, onChange }) => {
  const opts = options.map((o) =>
    Array.isArray(o)
      ? { value: o[0], label: o[1] }
      : typeof o === 'object'
        ? o
        : { value: o, label: o },
  );
  return (
    <TweakRow label={label}>
      <select
        style={{
          appearance: 'none',
          width: '100%',
          height: 26,
          padding: '0 22px 0 8px',
          border: '.5px solid rgba(0,0,0,.1)',
          borderRadius: 7,
          background: 'rgba(255,255,255,.6)',
          color: 'inherit',
          font: 'inherit',
          outline: 'none',
        }}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        {opts.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </TweakRow>
  );
};

export const TweakToggle = ({ label, value, onChange }) => (
  <div className='twk-row twk-row-h'>
    <div className='twk-lbl'>
      <span>{label}</span>
    </div>
    <button
      type='button'
      className='twk-toggle'
      data-on={value ? '1' : '0'}
      role='switch'
      aria-checked={!!value}
      onClick={() => onChange(!value)}
    >
      <i />
    </button>
  </div>
);

const isLight = (hex) => {
  const h = String(hex).replace('#', '');
  const x = h.length === 3 ? h.replace(/./g, (c) => c + c) : h.padEnd(6, '0');
  const n = parseInt(x.slice(0, 6), 16);
  if (Number.isNaN(n)) return true;
  const r = (n >> 16) & 255;
  const g = (n >> 8) & 255;
  const b = n & 255;
  return r * 299 + g * 587 + b * 114 > 148000;
};

const Check = ({ light }) => (
  <svg viewBox='0 0 14 14' aria-hidden='true'>
    <path
      d='M3 7.2 5.8 10 11 4.2'
      fill='none'
      strokeWidth='2.2'
      strokeLinecap='round'
      strokeLinejoin='round'
      stroke={light ? 'rgba(0,0,0,.78)' : '#fff'}
    />
  </svg>
);

export const TweakColor = ({ label, value, options, onChange }) => {
  const key = (o) => String(JSON.stringify(o)).toLowerCase();
  const cur = key(value);
  return (
    <TweakRow label={label}>
      <div className='twk-chips' role='radiogroup'>
        {options.map((o, i) => {
          const colors = Array.isArray(o) ? o : [o];
          const [hero] = colors;
          const on = key(o) === cur;
          return (
            <button
              key={i}
              type='button'
              className='twk-chip'
              role='radio'
              aria-checked={on}
              data-on={on ? '1' : '0'}
              aria-label={colors.join(', ')}
              title={colors.join(' · ')}
              style={{ background: hero }}
              onClick={() => onChange(o)}
            >
              {on && <Check light={isLight(hero)} />}
            </button>
          );
        })}
      </div>
    </TweakRow>
  );
};
