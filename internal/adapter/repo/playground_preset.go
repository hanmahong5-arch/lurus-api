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
package repo

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// PlaygroundPreset stores a named snapshot of the Playground form so users
// can reload frequently-used prompt + model combinations. Scoped by
// (TenantID, UserID) — each user owns their own set of presets.
type PlaygroundPreset struct {
	Id       uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID string `gorm:"index:idx_pg_preset_tenant_user;not null" json:"tenant_id"`
	UserID   int    `gorm:"index:idx_pg_preset_tenant_user;not null" json:"user_id"`
	// Name is unique per (tenant, user) — enforced at the application layer
	// (409 Conflict) rather than a DB unique constraint to keep SQLite/PG
	// compat simple and give a friendlier error message.
	Name   string `gorm:"not null" json:"name"`
	Prompt string `gorm:"type:text" json:"prompt"`
	// Models is a JSON-encoded []string — e.g. '["gpt-4o","claude-3.5-sonnet"]'
	Models string `gorm:"type:text" json:"models"`
	// Params is a JSON-encoded object — e.g. '{"temperature":0.7,"top_p":1.0,"max_tokens":1024}'
	Params    string    `gorm:"type:text" json:"params"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// CreatePlaygroundPreset inserts a new preset. Returns the created row.
func CreatePlaygroundPreset(p *PlaygroundPreset) error {
	return DB.Create(p).Error
}

// ListPlaygroundPresets returns all presets owned by (tenantID, userID),
// ordered newest-first (most recently created is most relevant).
func ListPlaygroundPresets(tenantID string, userID int) ([]*PlaygroundPreset, error) {
	var rows []*PlaygroundPreset
	err := DB.
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("id DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetPlaygroundPreset fetches a single preset by primary key. Returns nil if
// not found (callers distinguish "not found" vs "error" via the bool).
func GetPlaygroundPreset(id uint) (*PlaygroundPreset, error) {
	var row PlaygroundPreset
	err := DB.First(&row, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// DeletePlaygroundPreset removes a preset by primary key. The caller is
// responsible for verifying ownership before calling this.
func DeletePlaygroundPreset(id uint) error {
	return DB.Delete(&PlaygroundPreset{}, id).Error
}

// PlaygroundPresetNameExists reports whether a preset with the given name
// already exists for the (tenantID, userID) pair.
func PlaygroundPresetNameExists(tenantID string, userID int, name string) (bool, error) {
	var count int64
	err := DB.Model(&PlaygroundPreset{}).
		Where("tenant_id = ? AND user_id = ? AND name = ?", tenantID, userID, name).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
