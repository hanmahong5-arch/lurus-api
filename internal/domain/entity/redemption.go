package entity

import "gorm.io/gorm"

type Redemption struct {
	Id           int            `json:"id"`
	UserId       int            `json:"user_id"`
	TenantId     string         `json:"tenant_id" gorm:"type:varchar(36);index;index:idx_redemption_tenant_status,priority:1;default:'default'"`
	Key          string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int            `json:"status" gorm:"default:1;index:idx_redemption_tenant_status,priority:2"`
	Name         string         `json:"name" gorm:"index"`
	Quota        int            `json:"quota" gorm:"default:100"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
	Count        int            `json:"count" gorm:"-:all"`
	UsedUserId   int            `json:"used_user_id"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"`
}
