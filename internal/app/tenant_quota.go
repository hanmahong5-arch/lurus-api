package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"
)

// tenantQuotaEnforcementEnabled is the runtime kill-switch for tenant quota
// enforcement. Controlled by TENANT_QUOTA_ENFORCEMENT_ENABLED env var (default false).
var tenantQuotaEnforcementEnabled = parseTenantQuotaEnforcementEnabled()

// parseTenantQuotaEnforcementEnabled reads the env var so tests can re-invoke it.
func parseTenantQuotaEnforcementEnabled() bool {
	return common.GetEnvOrDefaultBool("TENANT_QUOTA_ENFORCEMENT_ENABLED", false)
}

// enforceTenantQuota checks whether adding preConsumedQuota to the current
// month's tenant usage would exceed Tenant.MaxQuota. Returns a typed API error
// on denial; nil means the request is allowed to proceed.
//
// Skip conditions (all no-ops):
//   - TENANT_QUOTA_ENFORCEMENT_ENABLED is false
//   - tenantID is empty
//   - tenant.MaxQuota == 0 (unlimited)
func enforceTenantQuota(tenantID string, preConsumedQuota int) *types.NewAPIError {
	if !tenantQuotaEnforcementEnabled {
		return nil
	}
	if tenantID == "" {
		return nil
	}

	tenant, err := repo.GetTenantByID(tenantID)
	if err != nil {
		// Fail open: if tenant lookup fails, don't block the request.
		common.SysError(fmt.Sprintf("enforceTenantQuota: tenant lookup failed tenant_id=%s err=%s", tenantID, err.Error()))
		return nil
	}
	if tenant.MaxQuota <= 0 {
		return nil
	}

	period := time.Now().UTC().Format("2006-01")
	used, err := repo.GetTenantMonthlyQuotaUsed(tenantID, period)
	if err != nil {
		// Fail open: log the error but don't block the request.
		common.SysError(fmt.Sprintf("enforceTenantQuota: monthly quota query failed tenant_id=%s period=%s err=%s",
			tenantID, period, err.Error()))
		return nil
	}

	if used+int64(preConsumedQuota) > tenant.MaxQuota {
		common.SysLogf(`{"event":"tenant_quota_denied","tenant_id":"%s","current_used":%d,"max_quota":%d,"requested":%d,"decision":"denied"}`,
			tenantID, used, tenant.MaxQuota, preConsumedQuota)
		return types.NewErrorWithStatusCode(
			fmt.Errorf("tenant monthly quota exceeded: used %d, max %d, requested %d",
				used, tenant.MaxQuota, preConsumedQuota),
			types.ErrorCodeTenantQuotaExceeded,
			http.StatusPaymentRequired,
			types.ErrOptionWithSkipRetry(),
			types.ErrOptionWithNoRecordErrorLog(),
		)
	}
	return nil
}
