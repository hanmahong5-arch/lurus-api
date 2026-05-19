package handler

import (
	"net/http"
	"strconv"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"

	"github.com/gin-gonic/gin"
)

// modelV2View is the whitelist projection of repo.Model for the v2 list API.
// Internal enrichment fields (BoundChannels, MatchedModels, MatchedCount) are
// deliberately excluded — the v2 catalog view does not need per-channel data.
type modelV2View struct {
	ID           int      `json:"id"`
	ModelName    string   `json:"model_name"`
	Vendor       string   `json:"vendor"`
	NameRule     int      `json:"name_rule"`
	Status       int      `json:"status"`
	QuotaTypes   []int    `json:"quota_types"`
	EnableGroups []string `json:"enable_groups"`
	Endpoints    string   `json:"endpoints"`
	CreatedTime  int64    `json:"created_time"`
}

// ListModelsV2 handles GET /api/v2/:tenant_slug/models
// Returns a paginated, filtered model catalog without internal enrichment fields.
func ListModelsV2(c *gin.Context) {
	slug := c.Param("tenant_slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "tenant slug required",
			"error_code": "INVALID_TENANT_SLUG",
		})
		return
	}

	_, err := repo.GetTenantBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "Tenant not found",
			"error_code": "TENANT_NOT_FOUND",
		})
		return
	}

	limitRaw := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitRaw)
	if err != nil || limit < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "limit must be an integer between 1 and 100",
			"error_code": "INVALID_PAGINATION",
		})
		return
	}
	if limit > 100 {
		limit = 100
	}

	offsetRaw := c.DefaultQuery("offset", "0")
	offset, err := strconv.Atoi(offsetRaw)
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "offset must be a non-negative integer",
			"error_code": "INVALID_PAGINATION",
		})
		return
	}

	keyword := c.Query("keyword")
	vendor := c.Query("vendor")

	models, total, err := repo.SearchModels(keyword, vendor, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to query models: " + err.Error(),
		})
		return
	}

	// Build whitelist-projected response items — no enrichModels call.
	items := make([]modelV2View, 0, len(models))
	for _, m := range models {
		if m == nil {
			continue
		}
		items = append(items, modelV2View{
			ID:           m.Id,
			ModelName:    m.ModelName,
			Vendor:       resolveVendorName(m.VendorID),
			NameRule:     m.NameRule,
			Status:       m.Status,
			QuotaTypes:   m.QuotaTypes,
			EnableGroups: m.EnableGroups,
			Endpoints:    m.Endpoints,
			CreatedTime:  m.CreatedTime,
		})
	}

	vendorCounts := buildVendorCounts()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items":         items,
			"total":         total,
			"limit":         limit,
			"offset":        offset,
			"vendor_counts": vendorCounts,
		},
	})
}

// resolveVendorName looks up the vendor name for a given vendor ID.
// Returns an empty string when the ID is zero or lookup fails — callers treat
// empty string as "unknown vendor" without panicking.
func resolveVendorName(vendorID int) string {
	if vendorID == 0 {
		return ""
	}
	v, err := repo.GetVendorByID(vendorID)
	if err != nil || v == nil {
		return ""
	}
	return v.Name
}

// buildVendorCounts returns a map of vendor name → model count for the full
// unfiltered catalog. Used by the frontend to render the vendor filter pills
// with counts. Errors are non-fatal; an empty map is returned instead.
func buildVendorCounts() map[string]int64 {
	type row struct {
		Name  string
		Count int64
	}
	var rows []row
	repo.DB.Table("models").
		Select("vendors.name as name, count(models.id) as count").
		Joins("LEFT JOIN vendors ON vendors.id = models.vendor_id AND vendors.deleted_at IS NULL").
		Where("models.deleted_at IS NULL").
		Group("vendors.name").
		Scan(&rows)
	result := make(map[string]int64, len(rows))
	for _, r := range rows {
		result[r.Name] = r.Count
	}
	return result
}
