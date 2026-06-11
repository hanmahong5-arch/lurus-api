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
import { useTranslation } from 'react-i18next';

/**
 * NotAvailable — honest "no backend" cell. Disambiguates the silent em-dash
 * that reads as "no data yet" from the truthful "this metric is not wired:
 * there is no endpoint behind it". Per CLAUDE.md §4.1 ⑥: a visible marker
 * with a stated reason, never a fake value.
 *
 * Use for columns/fields whose value cannot be produced because the backend
 * aggregation/storage does not exist (channel QPS/P50/cost-1h, log TTFT, …) —
 * NOT for genuinely-empty data (use a plain empty-state for "no rows yet").
 *
 * Usage:
 *   <NotAvailable reason="no metrics backend: channel QPS is not aggregated" />
 *   <NotAvailable reason="…" label="—" />   // when the column is numeric
 */
const NotAvailable = ({ reason, label }) => {
  const { t } = useTranslation();
  return (
    <span
      className='faint mono'
      data-testid='na-cell'
      data-na-reason={reason}
      title={reason}
      style={{
        fontSize: 11,
        cursor: 'help',
        borderBottom: '1px dotted var(--hf-rule)',
      }}
    >
      {label ?? t('console.not_available.label', 'n/a')}
    </span>
  );
};

export default NotAvailable;
