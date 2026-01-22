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
import AilurusDashboard from '../../components/dashboard/AilurusDashboard';

/**
 * Dashboard Page - Main user dashboard with Ailurus styling
 *
 * Features:
 * - Statistics cards with animated counters
 * - Charts panel with multiple visualization types
 * - API info, announcements, FAQ, and uptime panels
 * - Glassmorphic design with luminous effects
 */
const DashboardPage = () => (
  <div className="mt-[60px] px-4 py-2">
    <AilurusDashboard />
  </div>
);

export default DashboardPage;
