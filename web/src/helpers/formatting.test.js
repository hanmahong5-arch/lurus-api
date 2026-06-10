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
import { describe, it, expect, vi } from 'vitest';

// Echo keys so relative-time bucketing is asserted deterministically,
// independent of i18n init timing / detected language.
vi.mock('../i18n/i18n', () => ({ default: { t: (k) => k } }));

import {
  QUOTA_PER_USD,
  quotaToUSD,
  formatUSD,
  formatCNY,
  formatTime,
  formatRelativeTime,
} from './formatting';

describe('quota helpers', () => {
  it('uses the canonical 500_000 units-per-USD constant', () => {
    expect(QUOTA_PER_USD).toBe(500_000);
  });

  it('quotaToUSD converts and fixes decimals', () => {
    expect(quotaToUSD(500_000)).toBe('1.00');
    expect(quotaToUSD(0)).toBe('0.00');
    expect(quotaToUSD(null)).toBe('0.00');
    expect(quotaToUSD(750_000, 4)).toBe('1.5000');
  });

  it('formatUSD prefixes $ and em-dashes zero', () => {
    expect(formatUSD(500_000)).toBe('$1.0000');
    expect(formatUSD(0)).toBe('—');
  });
});

describe('formatCNY', () => {
  it('formats numbers with grouping and 2 decimals', () => {
    expect(formatCNY(1234.5)).toBe('¥1,234.50');
  });
  it('em-dashes non-numeric input', () => {
    expect(formatCNY('x')).toBe('—');
    expect(formatCNY(undefined)).toBe('—');
  });
});

describe('formatTime', () => {
  it('em-dashes empty/zero timestamps', () => {
    expect(formatTime(0)).toBe('—');
    expect(formatTime(null)).toBe('—');
  });
});

describe('formatRelativeTime', () => {
  // i18n uninitialized → t returns the key; assert the correct bucket key.
  const now = 1_000_000;
  it('picks the right relative bucket', () => {
    expect(formatRelativeTime(now - 30, now)).toBe(
      'console.common.time_just_now',
    );
    expect(formatRelativeTime(now - 120, now)).toBe(
      'console.common.time_minutes_ago',
    );
    expect(formatRelativeTime(now - 7200, now)).toBe(
      'console.common.time_hours_ago',
    );
    expect(formatRelativeTime(now - 200_000, now)).toBe(
      'console.common.time_days_ago',
    );
  });
  it('em-dashes a missing timestamp', () => {
    expect(formatRelativeTime(0, now)).toBe('—');
  });
});
