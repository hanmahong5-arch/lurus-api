package entity

import (
	"strings"

	"gorm.io/gorm"
)

type Token struct {
	Id                 int            `json:"id"`
	UserId             int            `json:"user_id" gorm:"index;index:idx_tenant_user,priority:2"`
	TenantId           string         `json:"tenant_id" gorm:"type:varchar(36);index;index:idx_tenant_user,priority:1;default:'default'"` // Tenant isolation
	Key                string         `json:"key" gorm:"type:char(48);uniqueIndex"`
	Status             int            `json:"status" gorm:"default:1"`
	Name               string         `json:"name" gorm:"index"`
	CreatedTime        int64          `json:"created_time" gorm:"bigint"`
	AccessedTime       int64          `json:"accessed_time" gorm:"bigint"`
	ExpiredTime        int64          `json:"expired_time" gorm:"bigint;default:-1"` // -1 means never expired
	RemainQuota        int            `json:"remain_quota" gorm:"default:0"`
	UnlimitedQuota     bool           `json:"unlimited_quota"`
	ModelLimitsEnabled bool           `json:"model_limits_enabled"`
	ModelLimits        string         `json:"model_limits" gorm:"type:varchar(1024);default:''"`
	AllowIps           *string        `json:"allow_ips" gorm:"default:''"`
	UsedQuota          int            `json:"used_quota" gorm:"default:0"`
	Group              string         `json:"group" gorm:"default:''"`
	CrossGroupRetry    bool           `json:"cross_group_retry"`
	IdentityAccountID  int64          `json:"identity_account_id" gorm:"default:0;index:idx_identity_account,where:identity_account_id > 0"` // lurus-platform account ID
	// CreatorUserID is the users.id of the Reseller who issued this key via the
	// Provisioning API (POST /internal/v1/provisioning/tenants/:slug/keys).
	// 0 = legacy / non-provisioned key. See ADR 2026-05-18 §3.3.
	CreatorUserId int            `json:"creator_user_id" gorm:"default:0;index:idx_tokens_creator_user_id,where:creator_user_id > 0"`
	// LastUsedAt is updated on relay hit for Provisioning-issued keys. Unix
	// seconds. 0 = never used. See ADR 2026-05-18 §3.3.
	LastUsedAt    int64          `json:"last_used_at" gorm:"default:0"`
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

func (token *Token) Clean() {
	token.Key = ""
}

func (token *Token) GetIpLimits() []string {
	ipLimits := make([]string, 0)
	if token.AllowIps == nil {
		return ipLimits
	}
	cleanIps := strings.ReplaceAll(*token.AllowIps, " ", "")
	if cleanIps == "" {
		return ipLimits
	}
	ips := strings.Split(cleanIps, "\n")
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		ip = strings.ReplaceAll(ip, ",", "")
		if ip != "" {
			ipLimits = append(ipLimits, ip)
		}
	}
	return ipLimits
}

func (token *Token) IsModelLimitsEnabled() bool {
	return token.ModelLimitsEnabled
}

func (token *Token) GetModelLimits() []string {
	if token.ModelLimits == "" {
		return []string{}
	}
	return strings.Split(token.ModelLimits, ",")
}

func (token *Token) GetModelLimitsMap() map[string]bool {
	limits := token.GetModelLimits()
	limitsMap := make(map[string]bool)
	for _, limit := range limits {
		limitsMap[limit] = true
	}
	return limitsMap
}
