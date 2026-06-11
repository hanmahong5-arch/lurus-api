// Package savings implements the Phase 1 cost-aware-routing savings analyzer:
// a pure re-pricing engine that estimates how much routing real historical
// spend to cheaper quality-equivalent models WOULD have saved. It performs no
// I/O (no DB, no gin) so the honesty core is fully unit-testable.
//
// Method (aggregate strategy — cheap, no per-row scan): for each token-priced
// model, back the realized blended rate out of real money and substitute the
// curated alternative. Group ratio is a common factor that cancels, so no
// per-row group lookup is needed.
//
// Caveat (also surfaced in the API/UI): cache tokens are folded into prompt by
// the caller, which UNDER-states savings — a deliberately conservative bias.
// Quality tiers are heuristic, not eval-backed.
package savings

import (
	"sort"

	"github.com/LurusTech/lurus-hub/internal/pkg/setting/ratio_setting"
)

// ModelSpend is a per-model spend aggregate over real consume-log rows.
type ModelSpend struct {
	Model            string
	TotalQuota       int64
	PromptTokens     int64
	CompletionTokens int64
	Count            int64
}

// Opportunity is a single model→substitute re-pricing result.
type Opportunity struct {
	Model        string  `json:"model"`
	AltModel     string  `json:"alt_model"`
	CurrentQuota int64   `json:"current_quota"`
	SavingsQuota int64   `json:"savings_quota"`
	SavingsPct   float64 `json:"savings_pct"`
}

// Scenario is the aggregate result for one substitution strategy.
type Scenario struct {
	TotalSavingsQuota int64         `json:"total_savings_quota"`
	SavingsPct        float64       `json:"savings_pct"`
	TopOpportunities  []Opportunity `json:"top_opportunities"`
}

// Excluded records a model whose spend was not re-priced, with an honest reason.
type Excluded struct {
	Model  string `json:"model"`
	Quota  int64  `json:"quota"`
	Reason string `json:"reason"`
}

// Result is the full analyzer output for a window of spend.
type Result struct {
	TotalSpendQuota int64      `json:"total_spend_quota"`
	Conservative    Scenario   `json:"conservative"`
	Aggressive      Scenario   `json:"aggressive"`
	Excluded        []Excluded `json:"excluded"`
}

const topOpportunityLimit = 10

// Exclusion reasons (stable identifiers surfaced in the API).
const (
	reasonPerCallPriced = "per_call_priced"
	reasonUnmapped      = "unmapped"
	reasonReasoning     = "reasoning_model"
	reasonFreeOrZero    = "free_or_zero"
)

// repriceModel returns the estimated quota saved by serving the same token mix
// on an alternative model, given config ratios. Group ratio is a common factor
// that cancels in the ratio of effective rates, so it never appears here:
//
//	effRatio = (prompt + completion*completionRatio) * modelRatio
//	savings  = totalQuota * (1 - effRatioAlt/effRatioOrig), floored at 0.
//
// totalQuota is the realized (group-scaled) spend; multiplying by the
// dimensionless rate ratio re-prices it without needing the group ratio.
func repriceModel(s ModelSpend, modelRatio, completionRatio, altModelRatio, altCompletionRatio float64) int64 {
	unitsOrig := float64(s.PromptTokens) + float64(s.CompletionTokens)*completionRatio
	unitsAlt := float64(s.PromptTokens) + float64(s.CompletionTokens)*altCompletionRatio
	effOrig := unitsOrig * modelRatio
	effAlt := unitsAlt * altModelRatio
	if effOrig <= 0 {
		return 0
	}
	saved := float64(s.TotalQuota) * (1 - effAlt/effOrig)
	if saved <= 0 {
		return 0
	}
	return int64(saved)
}

// Compute re-prices each token-priced model's real spend against its curated
// conservative (same-family) and aggressive (same-tier cross-vendor) substitute
// using LIVE pricing. It never mutates input and performs no I/O.
func Compute(spends []ModelSpend) Result {
	res := Result{Excluded: []Excluded{}}
	var consOpps, aggrOpps []Opportunity

	for _, s := range spends {
		res.TotalSpendQuota += s.TotalQuota

		// Per-call priced (image/suno/mj): GetModelPrice returns a real price,
		// so the spend is not token re-priceable.
		if _, perCall := ratio_setting.GetModelPrice(s.Model, false); perCall {
			res.Excluded = append(res.Excluded, Excluded{s.Model, s.TotalQuota, reasonPerCallPriced})
			continue
		}

		eq, ok := ratio_setting.GetModelEquivalence(s.Model)
		if !ok {
			res.Excluded = append(res.Excluded, Excluded{s.Model, s.TotalQuota, reasonUnmapped})
			continue
		}
		if eq.Reasoning {
			res.Excluded = append(res.Excluded, Excluded{s.Model, s.TotalQuota, reasonReasoning})
			continue
		}

		modelRatio, ratioOK, _ := ratio_setting.GetModelRatio(s.Model)
		if !ratioOK || modelRatio <= 0 {
			res.Excluded = append(res.Excluded, Excluded{s.Model, s.TotalQuota, reasonFreeOrZero})
			continue
		}
		completionRatio := ratio_setting.GetCompletionRatio(s.Model)

		if op, ok := opportunity(s, eq.ConservativeSub, modelRatio, completionRatio); ok {
			consOpps = append(consOpps, op)
			res.Conservative.TotalSavingsQuota += op.SavingsQuota
		}
		if op, ok := opportunity(s, eq.AggressiveSub, modelRatio, completionRatio); ok {
			aggrOpps = append(aggrOpps, op)
			res.Aggressive.TotalSavingsQuota += op.SavingsQuota
		}
	}

	res.Conservative.TopOpportunities = topN(consOpps)
	res.Aggressive.TopOpportunities = topN(aggrOpps)
	if res.TotalSpendQuota > 0 {
		res.Conservative.SavingsPct = float64(res.Conservative.TotalSavingsQuota) / float64(res.TotalSpendQuota)
		res.Aggressive.SavingsPct = float64(res.Aggressive.TotalSavingsQuota) / float64(res.TotalSpendQuota)
	}
	return res
}

// opportunity re-prices one model against one substitute using live ratios.
// Returns (zero, false) when the sub is absent/self/free or not cheaper.
func opportunity(s ModelSpend, sub string, modelRatio, completionRatio float64) (Opportunity, bool) {
	if sub == "" || sub == s.Model {
		return Opportunity{}, false
	}
	altRatio, ok, _ := ratio_setting.GetModelRatio(sub)
	if !ok || altRatio <= 0 {
		return Opportunity{}, false
	}
	altCompletion := ratio_setting.GetCompletionRatio(sub)
	saved := repriceModel(s, modelRatio, completionRatio, altRatio, altCompletion)
	if saved <= 0 {
		return Opportunity{}, false
	}
	pct := 0.0
	if s.TotalQuota > 0 {
		pct = float64(saved) / float64(s.TotalQuota)
	}
	return Opportunity{
		Model:        s.Model,
		AltModel:     sub,
		CurrentQuota: s.TotalQuota,
		SavingsQuota: saved,
		SavingsPct:   pct,
	}, true
}

// topN sorts opportunities by savings desc (model name as a stable tie-break)
// and returns at most topOpportunityLimit. Always returns a non-nil slice.
func topN(opps []Opportunity) []Opportunity {
	sort.Slice(opps, func(i, j int) bool {
		if opps[i].SavingsQuota != opps[j].SavingsQuota {
			return opps[i].SavingsQuota > opps[j].SavingsQuota
		}
		return opps[i].Model < opps[j].Model
	})
	if len(opps) > topOpportunityLimit {
		opps = opps[:topOpportunityLimit]
	}
	if opps == nil {
		return []Opportunity{}
	}
	return opps
}
