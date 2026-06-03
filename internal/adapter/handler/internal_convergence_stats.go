package handler

import (
	"net/http"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/gin-gonic/gin"
)

// convergenceStats holds unified-currency linkage counts across all tenants.
type convergenceStats struct {
	UsersTotal      int64 `json:"users_total"`
	UsersLinked     int64 `json:"users_linked"`
	TokensTotal     int64 `json:"tokens_total"`
	TokensLinked    int64 `json:"tokens_linked"`
	TokensUnlimited int64 `json:"tokens_unlimited"`
}

// InternalConvergenceStats returns read-only counts of newhub users and tokens
// linked to the unified-currency platform. All counts are global across tenants.
// GORM soft-delete is honoured on both tables (deleted_at IS NULL).
//
// GET /internal/admin/convergence-stats
func InternalConvergenceStats(c *gin.Context) {
	var stats convergenceStats

	// users_total: non-deleted users regardless of linkage status.
	if err := repo.DB.Model(&repo.User{}).Count(&stats.UsersTotal).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "count users failed"})
		return
	}

	// users_linked: non-deleted users with a platform account binding.
	if err := repo.DB.Model(&repo.User{}).
		Where("lurus_account_id IS NOT NULL").
		Count(&stats.UsersLinked).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "count linked users failed"})
		return
	}

	// tokens_total: non-deleted tokens (GORM soft-delete applied automatically).
	if err := repo.DB.Model(&repo.Token{}).Count(&stats.TokensTotal).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "count tokens failed"})
		return
	}

	// tokens_linked: non-deleted tokens carrying a platform account ID.
	if err := repo.DB.Model(&repo.Token{}).
		Where("identity_account_id > 0").
		Count(&stats.TokensLinked).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "count linked tokens failed"})
		return
	}

	// tokens_unlimited: non-deleted tokens with wallet-gated spend (no local cap).
	if err := repo.DB.Model(&repo.Token{}).
		Where("unlimited_quota = ?", true).
		Count(&stats.TokensUnlimited).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "count unlimited tokens failed"})
		return
	}

	c.JSON(http.StatusOK, stats)
}
