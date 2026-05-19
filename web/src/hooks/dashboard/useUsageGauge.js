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

import { useMemo } from 'react';

/**
 * Alert levels based on usage percentage.
 * @param {number} percent - Usage percentage (0-100)
 * @returns {'green'|'yellow'|'red'|'critical'}
 */
function getLevel(percent) {
  if (percent >= 95) return 'critical';
  if (percent >= 80) return 'red';
  if (percent >= 50) return 'yellow';
  return 'green';
}

/**
 * @param {object} userState - User context state
 * @param {Array} trendData - Balance trend data array (recent 7+ points)
 * @returns {object} Usage gauge metrics
 */
export function useUsageGauge(userState, trendData) {
  return useMemo(() => {
    const quota = userState?.user?.quota ?? 0;
    const usedQuota = userState?.user?.used_quota ?? 0;
    const totalQuota = quota + usedQuota;

    // Prevent division by zero
    const usagePercent = totalQuota > 0 ? (usedQuota / totalQuota) * 100 : 0;

    // Calculate daily consumption rate from trend data (last 7 points average)
    let dailyRate = 0;
    const balanceTrend = trendData?.balance;
    if (balanceTrend && balanceTrend.length >= 2) {
      const recentPoints = balanceTrend.slice(-7);
      if (recentPoints.length >= 2) {
        const totalDrop =
          recentPoints[0] - recentPoints[recentPoints.length - 1];
        dailyRate = Math.max(0, totalDrop / (recentPoints.length - 1));
      }
    }

    // Projected exhaustion
    let daysRemaining = Infinity;
    let exhaustionDate = null;
    if (dailyRate > 0 && quota > 0) {
      daysRemaining = Math.ceil(quota / dailyRate);
      const date = new Date();
      date.setDate(date.getDate() + daysRemaining);
      exhaustionDate = date.toISOString().slice(0, 10);
    }

    const level = getLevel(usagePercent);

    return {
      quota,
      usedQuota,
      totalQuota,
      usagePercent: Math.min(100, Math.round(usagePercent * 10) / 10),
      dailyRate: Math.round(dailyRate * 100) / 100,
      daysRemaining: isFinite(daysRemaining) ? daysRemaining : null,
      exhaustionDate,
      level,
    };
  }, [userState?.user?.quota, userState?.user?.used_quota, trendData?.balance]);
}
