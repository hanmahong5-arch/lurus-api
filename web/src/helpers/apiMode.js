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
const TENANT_SLUG_KEY = 'tenant_slug';
const DEFAULT_TENANT = 'lurus';

export function isV2Mode() {
  return !!localStorage.getItem(TENANT_SLUG_KEY);
}

export function getTenantSlug() {
  return localStorage.getItem(TENANT_SLUG_KEY) || '';
}

export function setTenantSlug(slug) {
  if (slug) {
    localStorage.setItem(TENANT_SLUG_KEY, slug);
  }
}

export function clearTenantSlug() {
  localStorage.removeItem(TENANT_SLUG_KEY);
}

// Build V2 API path: /api/v2/{slug}{path}
export function v2Url(path) {
  const slug = getTenantSlug() || DEFAULT_TENANT;
  return `/api/v2/${slug}${path}`;
}
