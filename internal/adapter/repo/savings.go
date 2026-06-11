package repo

import (
	"fmt"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/ratio_setting"
)

// ModelSpendRow is a per-model spend aggregate over consume logs. It carries
// the prompt/completion token split the savings re-pricing engine needs.
type ModelSpendRow struct {
	ModelName        string `json:"model_name"`
	TotalQuota       int64  `json:"total_quota"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	Count            int64  `json:"count"`
}

// GetSpendByModel aggregates token-priced consume spend grouped by model over
// rows since startTime. Clones the governance.go query shape (type=consume,
// quota>0). Order by spend so callers can cheaply take the top-N.
func GetSpendByModel(startTime int64) ([]ModelSpendRow, error) {
	var rows []ModelSpendRow
	err := LOG_DB.Model(&entity.Log{}).
		Select(`model_name,
			COALESCE(SUM(quota), 0) as total_quota,
			COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as completion_tokens,
			COUNT(*) as count`).
		Where("type = ? AND created_at >= ? AND quota > 0", LogTypeConsume, startTime).
		Group("model_name").
		Order("total_quota DESC").
		Find(&rows).Error
	return rows, err
}

// ProductSpendRow is a per-source-product spend aggregate (Workstream 0).
type ProductSpendRow struct {
	Product    string `json:"product"`
	TotalQuota int64  `json:"total_quota"`
	Count      int64  `json:"count"`
}

// GetSpendByProduct aggregates consume spend grouped by the source_product tag
// carried in the log's Other JSON. Rows written before attribution shipped have
// no tag and fold into DefaultSourceProduct, so the dimension is never empty.
func GetSpendByProduct(startTime int64) ([]ProductSpendRow, error) {
	// The default literal is injected as a trusted compile-time constant.
	sel := fmt.Sprintf(`COALESCE(%s, '%s') as product,
		COALESCE(SUM(quota), 0) as total_quota,
		COUNT(*) as count`, jsonSourceProductExpr(), ratio_setting.DefaultSourceProduct)
	var rows []ProductSpendRow
	err := LOG_DB.Model(&entity.Log{}).
		Select(sel).
		Where("type = ? AND created_at >= ? AND quota > 0", LogTypeConsume, startTime).
		Group("product").
		Order("total_quota DESC").
		Find(&rows).Error
	return rows, err
}

// jsonSourceProductExpr returns a dialect-specific SQL fragment that extracts
// Other.source_product as text. The Other column is TEXT that holds either a
// JSON object (relay rows) or an empty string (some error/legacy rows), so the
// extract must be guarded against non-JSON input or it errors mid-query.
func jsonSourceProductExpr() string {
	switch LOG_DB.Name() {
	case "postgres":
		// newhub always persists Other as a valid JSON object or "" — NULLIF
		// guards the empty case so the ::jsonb cast never sees invalid input.
		return `NULLIF(other, '')::jsonb ->> 'source_product'`
	case "mysql":
		return `CASE WHEN JSON_VALID(other) THEN JSON_UNQUOTE(JSON_EXTRACT(other, '$.source_product')) END`
	default: // sqlite (incl. test DB) — JSON1 is built into modern SQLite
		return `CASE WHEN json_valid(other) THEN json_extract(other, '$.source_product') END`
	}
}
