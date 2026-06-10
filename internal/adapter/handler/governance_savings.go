package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app/savings"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
)

const (
	savingsDefaultHours = 168 // 7d — the analyzer's natural window
	savingsMaxHours     = 720 // 30d — matches the governance cap
	// savingsCaveat is rendered verbatim in the UI so the heuristic nature of
	// the number is never hidden.
	savingsCaveat = "aggregate estimate, conservative on cache; tiers heuristic, not eval-backed"
)

// parseSavingsWindowStart returns the start timestamp and the clamped hours.
func parseSavingsWindowStart(c *gin.Context) (int64, int) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", strconv.Itoa(savingsDefaultHours)))
	if hours <= 0 || hours > savingsMaxHours {
		hours = savingsDefaultHours
	}
	return time.Now().Add(-time.Duration(hours) * time.Hour).Unix(), hours
}

// GetGovernanceSavings re-prices real historical spend against curated model
// substitutes to estimate cost-aware-routing savings. Read-only; touches no hot
// path. Every total is SUM(quota) of real rows — never a constant.
// GET /api/v2/admin/governance/savings?hours=168
func GetGovernanceSavings(c *gin.Context) {
	start, hours := parseSavingsWindowStart(c)

	modelRows, err := repo.GetSpendByModel(start)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "query failed"})
		return
	}
	productRows, err := repo.GetSpendByProduct(start)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "query failed"})
		return
	}

	spends := make([]savings.ModelSpend, 0, len(modelRows))
	for _, r := range modelRows {
		spends = append(spends, savings.ModelSpend{
			Model:            r.ModelName,
			TotalQuota:       r.TotalQuota,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
			Count:            r.Count,
		})
	}
	result := savings.Compute(spends)

	if modelRows == nil {
		modelRows = []repo.ModelSpendRow{}
	}
	if productRows == nil {
		productRows = []repo.ProductSpendRow{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"window_hours":      hours,
			"currency_unit":     "quota",
			"quota_per_usd":     common.QuotaPerUnit,
			"heuristic":         true,
			"caveat":            savingsCaveat,
			"total_spend_quota": result.TotalSpendQuota,
			"spend_by_model":    modelRows,
			"spend_by_product":  productRows,
			"scenarios": gin.H{
				"conservative": result.Conservative,
				"aggressive":   result.Aggressive,
			},
			"excluded": result.Excluded,
		},
	})
}
