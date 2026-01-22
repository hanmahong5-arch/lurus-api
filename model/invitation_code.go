package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/QuantumNous/lurus-api/common"
	"gorm.io/gorm"
)

// InvitationCode represents a one-time use invitation code for registration
type InvitationCode struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	Code      string `json:"code" gorm:"uniqueIndex;size:32"`
	CreatedBy int    `json:"created_by" gorm:"index"` // User ID of the creator (Root/Admin)
	UsedBy    *int   `json:"used_by"`                 // User ID who used this code, nil if unused
	UsedAt    *int64 `json:"used_at"`                 // Timestamp when used
	ExpiresAt *int64 `json:"expires_at"`              // Expiration timestamp, nil means never expires
	CreatedAt int64  `json:"created_at"`
}

func (InvitationCode) TableName() string {
	return "invitation_codes"
}

// IsValid checks if the invitation code is still valid (not used and not expired)
func (c *InvitationCode) IsValid() bool {
	if c == nil {
		return false
	}
	// Already used
	if c.UsedBy != nil {
		return false
	}
	// Check expiration
	if c.ExpiresAt != nil && *c.ExpiresAt < time.Now().Unix() {
		return false
	}
	return true
}

// GenerateInviteCode generates a random 16-character invitation code
func GenerateInviteCode() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateInvitationCode creates a new invitation code
func CreateInvitationCode(createdBy int, expiresIn *int) (*InvitationCode, string, error) {
	code, err := GenerateInviteCode()
	if err != nil {
		return nil, "", err
	}

	inviteCode := &InvitationCode{
		Code:      code,
		CreatedBy: createdBy,
		CreatedAt: time.Now().Unix(),
	}

	// Set expiration if provided
	if expiresIn != nil && *expiresIn > 0 {
		expiresAt := time.Now().Unix() + int64(*expiresIn)
		inviteCode.ExpiresAt = &expiresAt
	}

	if err := DB.Create(inviteCode).Error; err != nil {
		return nil, "", err
	}

	return inviteCode, code, nil
}

// CreateInvitationCodes creates multiple invitation codes
func CreateInvitationCodes(createdBy int, count int, expiresIn *int) ([]*InvitationCode, []string, error) {
	if count <= 0 || count > 100 {
		return nil, nil, errors.New("count must be between 1 and 100")
	}

	codes := make([]*InvitationCode, 0, count)
	codeStrings := make([]string, 0, count)

	for i := 0; i < count; i++ {
		code, codeStr, err := CreateInvitationCode(createdBy, expiresIn)
		if err != nil {
			return nil, nil, err
		}
		codes = append(codes, code)
		codeStrings = append(codeStrings, codeStr)
	}

	return codes, codeStrings, nil
}

// GetInvitationCodeByCode retrieves an invitation code by its code string
func GetInvitationCodeByCode(code string) (*InvitationCode, error) {
	if code == "" {
		return nil, errors.New("code is required")
	}

	var inviteCode InvitationCode
	err := DB.Where("code = ?", code).First(&inviteCode).Error
	if err != nil {
		return nil, err
	}
	return &inviteCode, nil
}

// GetInvitationCodeById retrieves an invitation code by its ID
func GetInvitationCodeById(id int) (*InvitationCode, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}

	var inviteCode InvitationCode
	err := DB.First(&inviteCode, id).Error
	if err != nil {
		return nil, err
	}
	return &inviteCode, nil
}

// UseInvitationCode marks an invitation code as used by a user
func UseInvitationCode(code string, userId int) error {
	if code == "" {
		return errors.New("code is required")
	}
	if userId == 0 {
		return errors.New("invalid user id")
	}

	common.RandomSleep()
	return DB.Transaction(func(tx *gorm.DB) error {
		var inviteCode InvitationCode
		// Lock the row for update
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("code = ?", code).First(&inviteCode).Error; err != nil {
			return errors.New("invalid invitation code")
		}

		if !inviteCode.IsValid() {
			if inviteCode.UsedBy != nil {
				return errors.New("invitation code already used")
			}
			return errors.New("invitation code expired")
		}

		now := time.Now().Unix()
		inviteCode.UsedBy = &userId
		inviteCode.UsedAt = &now

		return tx.Save(&inviteCode).Error
	})
}

// ValidateInvitationCode checks if a code is valid without using it
func ValidateInvitationCode(code string) bool {
	inviteCode, err := GetInvitationCodeByCode(code)
	if err != nil {
		return false
	}
	return inviteCode.IsValid()
}

// GetAllInvitationCodes retrieves all invitation codes with pagination
func GetAllInvitationCodes(startIdx int, num int) ([]*InvitationCode, int64, error) {
	var codes []*InvitationCode
	var total int64

	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&InvitationCode{}).Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err := tx.Order("id desc").Limit(num).Offset(startIdx).Find(&codes).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return codes, total, nil
}

// GetInvitationCodesByCreator retrieves codes created by a specific user
func GetInvitationCodesByCreator(creatorId int, startIdx int, num int) ([]*InvitationCode, int64, error) {
	var codes []*InvitationCode
	var total int64

	query := DB.Model(&InvitationCode{}).Where("created_by = ?", creatorId)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("id desc").Limit(num).Offset(startIdx).Find(&codes).Error; err != nil {
		return nil, 0, err
	}

	return codes, total, nil
}

// SearchInvitationCodes searches invitation codes by code prefix
func SearchInvitationCodes(keyword string, startIdx int, num int) ([]*InvitationCode, int64, error) {
	var codes []*InvitationCode
	var total int64

	query := DB.Model(&InvitationCode{}).Where("code LIKE ?", keyword+"%")

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("id desc").Limit(num).Offset(startIdx).Find(&codes).Error; err != nil {
		return nil, 0, err
	}

	return codes, total, nil
}

// DeleteInvitationCode deletes an unused invitation code
func DeleteInvitationCode(id int) error {
	if id == 0 {
		return errors.New("id is required")
	}

	inviteCode, err := GetInvitationCodeById(id)
	if err != nil {
		return err
	}

	// Only allow deletion of unused codes
	if inviteCode.UsedBy != nil {
		return errors.New("cannot delete used invitation code")
	}

	return DB.Delete(inviteCode).Error
}

// DeleteExpiredInvitationCodes deletes all expired unused invitation codes
func DeleteExpiredInvitationCodes() (int64, error) {
	now := time.Now().Unix()
	result := DB.Where("used_by IS NULL AND expires_at IS NOT NULL AND expires_at < ?", now).Delete(&InvitationCode{})
	return result.RowsAffected, result.Error
}

// GetInvitationCodeStats returns statistics about invitation codes
func GetInvitationCodeStats() (total int64, used int64, expired int64, active int64, err error) {
	now := time.Now().Unix()

	if err = DB.Model(&InvitationCode{}).Count(&total).Error; err != nil {
		return
	}

	if err = DB.Model(&InvitationCode{}).Where("used_by IS NOT NULL").Count(&used).Error; err != nil {
		return
	}

	if err = DB.Model(&InvitationCode{}).Where("used_by IS NULL AND expires_at IS NOT NULL AND expires_at < ?", now).Count(&expired).Error; err != nil {
		return
	}

	active = total - used - expired
	return
}
