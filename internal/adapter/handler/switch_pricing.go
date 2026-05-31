package handler

import (
	"net/http"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// GetSwitchPricing returns the public pricing catalogue consumed by the Switch
// desktop client's cost dashboard.
//
// Route: GET /api/v2/switch/pricing (no auth — public, cacheable).
//
// It mirrors GetPricingV2's field whitelist (no admin-only usable_group /
// auto_groups) but drops the per-tenant requirement (this is one public
// catalogue) and ADDS two fields so the client can derive USD-per-token
// faithfully instead of guessing:
//
//	completion_ratio — scales output rate off model_ratio
//	quota_per_unit   — the quota↔USD scale (newapi default 500000 quota = $1)
//
// so the client computes:
//
//	input_per_mtok  = model_ratio × (1e6 / quota_per_unit)
//	output_per_mtok = model_ratio × completion_ratio × (1e6 / quota_per_unit)
//
// Read-only; no DB migration.
func GetSwitchPricing(c *gin.Context) {
	rawPricing := repo.GetPricing()

	type pricingItem struct {
		ModelName              interface{} `json:"model_name"`
		Vendor                 interface{} `json:"vendor"`
		QuotaType              interface{} `json:"quota_type"`
		ModelRatio             interface{} `json:"model_ratio"`
		CompletionRatio        interface{} `json:"completion_ratio"`
		ModelPrice             interface{} `json:"model_price"`
		EnableGroups           interface{} `json:"enable_groups"`
		SupportedEndpointTypes interface{} `json:"supported_endpoint_types"`
	}

	// Build vendor id→name lookup once.
	vendors := repo.GetVendors()
	vendorByID := make(map[int]string, len(vendors))
	for _, v := range vendors {
		vendorByID[v.ID] = v.Name
	}

	pricing := make([]pricingItem, 0, len(rawPricing))
	for _, p := range rawPricing {
		pricing = append(pricing, pricingItem{
			ModelName:              p.ModelName,
			Vendor:                 vendorByID[p.VendorID],
			QuotaType:              p.QuotaType,
			ModelRatio:             p.ModelRatio,
			CompletionRatio:        p.CompletionRatio,
			ModelPrice:             p.ModelPrice,
			EnableGroups:           p.EnableGroup,
			SupportedEndpointTypes: p.SupportedEndpointTypes,
		})
	}

	// group_ratio scoped to all groups (public catalogue — no per-user
	// narrowing, matching GetPricingV2).
	groupRatio := make(map[string]float64)
	for k, v := range ratio_setting.GetGroupRatioCopy() {
		groupRatio[k] = v
	}

	vendorNames := make([]string, 0, len(vendors))
	for _, v := range vendors {
		vendorNames = append(vendorNames, v.Name)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"pricing":        pricing,
			"vendors":        vendorNames,
			"group_ratio":    groupRatio,
			"quota_per_unit": common.QuotaPerUnit,
		},
	})
}
