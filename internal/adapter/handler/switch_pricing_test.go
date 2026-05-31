package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// The public Switch pricing endpoint reuses the pricing-test DB fixtures
// (setupPricingRouter sets repo.DB / LOG_DB / tenant), but registers its own
// no-tenant route on a fresh engine so the static "/switch/pricing" path does
// not collide with the ":tenant_slug/pricing" wildcard.

func switchPricingRouter() *gin.Engine {
	r := gin.New()
	r.GET("/api/v2/switch/pricing", GetSwitchPricing)
	return r
}

func TestGetSwitchPricing_HappyPathNoTenant(t *testing.T) {
	setupPricingRouter(t) // sets the repo globals + cleanup
	r := switchPricingRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v2/switch/pricing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	resp := parsePricing(t, w) // shared helper from v2_pricing_test.go
	if resp["success"] != true {
		t.Errorf("success = %v, want true", resp["success"])
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data missing/wrong type, body: %s", w.Body.String())
	}
	for _, key := range []string{"pricing", "vendors", "group_ratio", "quota_per_unit"} {
		if _, present := data[key]; !present {
			t.Errorf("data.%s missing from response", key)
		}
	}
}

func TestGetSwitchPricing_QuotaPerUnitIsStandardScale(t *testing.T) {
	setupPricingRouter(t)
	r := switchPricingRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v2/switch/pricing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp := parsePricing(t, w)
	data := resp["data"].(map[string]interface{})
	// newapi standard: 500000 quota = $1. The client divides quota totals by
	// this to get USD, so a drift here would silently mis-bill the dashboard.
	if qpu, _ := data["quota_per_unit"].(float64); qpu != 500000 {
		t.Errorf("quota_per_unit = %v, want 500000", data["quota_per_unit"])
	}
}

func TestGetSwitchPricing_StripsAdminOnlyFields(t *testing.T) {
	setupPricingRouter(t)
	r := switchPricingRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v2/switch/pricing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	for _, forbidden := range []string{"usable_group", "auto_groups"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("response leaked admin-only field %q", forbidden)
		}
	}
}
