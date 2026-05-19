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
import HFShell from '../../../components/hifi/HFShell';
import {
  useTweaks,
  TweaksPanel,
  TweakSection,
  TweakRadio,
  TweakColor,
  TweakToggle,
} from '../../../components/hifi/TweaksPanel';

/*
 * HiFi 16 — Variants: Dashboard rendered 4 ways via Tweaks.
 * Ported from hifi/hf16-variants.jsx (2026-05-07 variants pack).
 * Tweaks panel state replaces the iframe postMessage protocol with local useState.
 */

const LAYOUTS = {
  grid: { cols: 'repeat(4, 1fr)', big: 'span 2', span: 'span 1' },
  stacked: { cols: '1fr', big: 'span 1', span: 'span 1' },
  sidebar: { cols: '320px 1fr', big: 'span 1', span: 'span 1' },
  feed: { cols: '1fr 1fr', big: 'span 2', span: 'span 1' },
};

const KPIS = [
  {
    l: 'qps',
    v: '398',
    d: '+12.4%',
    t: [4, 5, 7, 6, 8, 9, 11, 10, 12, 11, 13, 12, 14],
  },
  {
    l: 'ttft p50',
    v: '412ms',
    d: '−8ms',
    t: [9, 10, 9, 8, 9, 8, 8, 7, 8, 7, 8, 8, 7],
  },
  {
    l: 'error rate',
    v: '0.81%',
    d: '+0.22pp',
    t: [2, 3, 2, 3, 4, 5, 6, 8, 9, 11, 12, 10, 8],
  },
  {
    l: 'spend / hr',
    v: '$1,051',
    d: '+$48',
    t: [6, 6, 7, 8, 8, 9, 10, 10, 11, 10, 11, 12, 11],
  },
];

const SIGNALS = [
  ['err', 'openai/backup down · 12m'],
  ['warn', 'vertex/asia errors ↑12%'],
  ['warn', 'acme near monthly cap'],
];

const HFVariants = () => {
  const [t, setTweak] = useTweaks({
    layout: 'grid',
    density: 'comfortable',
    accent: '#ff5d1f',
    showSparks: true,
  });

  const L = LAYOUTS[t.layout] || LAYOUTS.grid;
  const pad = t.density === 'compact' ? 12 : t.density === 'roomy' ? 24 : 16;

  return (
    <div
      style={{
        '--hf-accent': t.accent,
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <HFShell
        active='dashboard'
        crumbs={['variants', 'dashboard · ' + t.layout]}
        actions={
          <>
            <button type='button' className='btn'>
              last 24h ▾
            </button>
            <span className='lbl'>+ tweaks panel ↗</span>
          </>
        }
      >
        <div className='hf-page-head'>
          <div>
            <div className='lbl' style={{ marginBottom: 6 }}>
              variant · {t.layout} · {t.density}
            </div>
            <h1>Today, so far</h1>
            <div className='sub'>
              use the Tweaks panel (bottom-right) to swap layout, density,
              accent
            </div>
          </div>
        </div>

        <div
          style={{
            padding: pad + 8,
            display: 'grid',
            gridTemplateColumns: L.cols,
            gap: pad,
          }}
        >
          {KPIS.map((k, i) => (
            <div key={i} className='panel' style={{ padding: pad + 4 }}>
              <div className='lbl'>{k.l}</div>
              <div
                className='display'
                style={{
                  fontSize: t.density === 'compact' ? 24 : 30,
                  marginTop: 4,
                }}
              >
                {k.v}
              </div>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'flex-end',
                  marginTop: 6,
                }}
              >
                <span className='mono muted' style={{ fontSize: 10 }}>
                  {k.d} · 1h
                </span>
                {t.showSparks && (
                  <span className='spark'>
                    {k.t.map((v, j) => (
                      <i
                        key={j}
                        style={{
                          height: v * 1.2 + 'px',
                          background: 'var(--hf-accent)',
                        }}
                      />
                    ))}
                  </span>
                )}
              </div>
            </div>
          ))}

          <div
            className='panel'
            style={{ gridColumn: L.big, padding: pad + 4 }}
          >
            <div className='lbl'>model spend · last 24h</div>
            <div
              style={{
                display: 'flex',
                alignItems: 'flex-end',
                gap: 6,
                height: 140,
                marginTop: 12,
              }}
            >
              {[
                28, 34, 41, 38, 52, 68, 72, 84, 92, 86, 78, 82, 90, 88, 76, 68,
                58, 52, 46, 42, 38, 34, 32, 30,
              ].map((v, i) => (
                <div
                  key={i}
                  style={{
                    flex: 1,
                    height: v + '%',
                    background:
                      i === 9 ? 'var(--hf-accent)' : 'var(--hf-ink-2)',
                    opacity: i === 9 ? 1 : 0.35,
                  }}
                />
              ))}
            </div>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                marginTop: 6,
              }}
            >
              <span className='faint mono' style={{ fontSize: 9 }}>
                00:00
              </span>
              <span className='faint mono' style={{ fontSize: 9 }}>
                now
              </span>
            </div>
          </div>

          <div
            className='panel'
            style={{ gridColumn: L.span, padding: pad + 4 }}
          >
            <div className='lbl'>active signals · 3</div>
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: 8,
                marginTop: 10,
              }}
            >
              {SIGNALS.map((a, i) => (
                <div
                  key={i}
                  style={{
                    padding: '6px 10px',
                    borderLeft: '2px solid var(--hf-' + a[0] + ')',
                    background: 'var(--hf-paper)',
                    fontSize: 11,
                  }}
                >
                  {a[1]}
                </div>
              ))}
            </div>
          </div>
        </div>
      </HFShell>

      <TweaksPanel title='Tweaks'>
        <TweakSection title='layout'>
          <TweakRadio
            label='layout'
            value={t.layout}
            onChange={(v) => setTweak('layout', v)}
            options={[
              ['grid', 'grid'],
              ['stacked', 'stack'],
              ['sidebar', 'side'],
              ['feed', 'feed'],
            ]}
          />
        </TweakSection>
        <TweakSection title='density'>
          <TweakRadio
            label='density'
            value={t.density}
            onChange={(v) => setTweak('density', v)}
            options={[
              ['compact', 'compact'],
              ['comfortable', 'comfy'],
              ['roomy', 'roomy'],
            ]}
          />
        </TweakSection>
        <TweakSection title='accent color'>
          <TweakColor
            label='accent'
            value={t.accent}
            onChange={(v) => setTweak('accent', v)}
            options={['#ff5d1f', '#2d4a8a', '#1f7a4f', '#b3392b', '#5a3aa8']}
          />
        </TweakSection>
        <TweakSection title='sparklines'>
          <TweakToggle
            label='show sparks'
            value={t.showSparks}
            onChange={(v) => setTweak('showSparks', v)}
          />
        </TweakSection>
      </TweaksPanel>
    </div>
  );
};

export default HFVariants;
