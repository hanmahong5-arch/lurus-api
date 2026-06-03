package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/logger"
	"github.com/LurusTech/lurus-hub/internal/pkg/metrics"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"

	"github.com/gin-gonic/gin"
)

// ReturnPreConsumedQuota refunds local quota and releases platform pre-auth
// when a relay request fails after pre-consumption. Must be safe to call
// multiple times (idempotent on relayInfo state).
func ReturnPreConsumedQuota(c *gin.Context, relayInfo *relaycommon.RelayInfo) {
	// Refund local quota
	if relayInfo.FinalPreConsumedQuota != 0 {
		logger.LogInfo(c, fmt.Sprintf("refunding pre-consumed quota %s for user %d",
			logger.FormatQuota(relayInfo.FinalPreConsumedQuota), relayInfo.UserId))
		err := PostConsumeQuota(relayInfo, -relayInfo.FinalPreConsumedQuota, 0, false)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to refund local quota: userId=%d, amount=%d, err=%s",
				relayInfo.UserId, relayInfo.FinalPreConsumedQuota, err.Error()))
		}
	}

	// Release platform wallet freeze — every pre-auth MUST be either settled or released.
	releasePlatformPreAuth(relayInfo)
}

// releasePlatformPreAuth releases a platform pre-auth with retry-to-outbox fallback.
// Safe to call when PlatformPreAuthID == 0 (no-op).
func releasePlatformPreAuth(relayInfo *relaycommon.RelayInfo) {
	preAuthID := relayInfo.PlatformPreAuthID
	if preAuthID <= 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := common.ReleaseWithBreaker(ctx, preAuthID); err != nil {
		common.SysLog(fmt.Sprintf("release pre-auth %d failed, enqueuing outbox: %s", preAuthID, err.Error()))
		if enqErr := EnqueueRelease(relayInfo.IdentityAccountID, preAuthID); enqErr != nil {
			// Both release and outbox failed — platform TTL (300s) is the safety net.
			// Log at highest severity so ops can investigate.
			common.SysError(fmt.Sprintf("CRITICAL: pre-auth %d stuck frozen — both release and outbox failed. "+
				"Platform TTL will auto-expire in ≤300s. release_err=%s, outbox_err=%s",
				preAuthID, err.Error(), enqErr.Error()))
		}
	}
	// Keep PlatformPreAuthID for observability in logs/metrics (don't clear to 0).
}

// PreConsumeQuota validates the user can afford the request and pre-deducts quota.
//
// When unified billing is enabled (BILLING_UNIFIED_ENABLED=true) and the token
// is linked to a platform account (IdentityAccountID > 0), this also freezes
// the estimated cost in the platform wallet via PreAuthorize. On any failure
// after a successful pre-auth, the caller MUST call ReturnPreConsumedQuota to
// release the frozen wallet balance.
func PreConsumeQuota(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	// Guard: don't re-enter pre-auth on relay retry (preAuthID already set from first attempt)
	if relayInfo.PlatformPreAuthID > 0 {
		// Already pre-authorized — skip platform call, continue to local quota check
		logger.LogInfo(c, fmt.Sprintf("skipping re-entry PreAuthorize, existing preAuthID=%d", relayInfo.PlatformPreAuthID))
	} else if common.BillingUnifiedEnabled() && relayInfo.IdentityAccountID > 0 && preConsumedQuota > 0 {
		if apiErr := platformPreAuthorize(c, preConsumedQuota, relayInfo); apiErr != nil {
			return apiErr
		}
	}

	// Local quota validation (always runs — backward compat + defense in depth)
	userQuota, err := repo.GetUserQuota(relayInfo.UserId, false)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}

	if userQuota <= 0 || userQuota-preConsumedQuota < 0 {
		// Local quota insufficient — must release platform pre-auth if one was created.
		releasePlatformPreAuth(relayInfo)
		relayInfo.PlatformPreAuthID = 0
		return types.NewErrorWithStatusCode(
			fmt.Errorf("insufficient quota: available %s, required %s",
				logger.FormatQuota(userQuota), logger.FormatQuota(preConsumedQuota)),
			types.ErrorCodeInsufficientUserQuota, http.StatusPaymentRequired,
			types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog(),
			types.ErrOptionWithTopupURL())
	}

	// Tenant monthly quota enforcement (runs after user-level check).
	if apiErr := enforceTenantQuota(c.GetString("tenant_id"), preConsumedQuota); apiErr != nil {
		releasePlatformPreAuth(relayInfo)
		relayInfo.PlatformPreAuthID = 0
		return apiErr
	}

	// Trust optimization: skip local pre-deduction when balance is high enough
	trustQuota := common.GetTrustQuota()
	relayInfo.UserQuota = userQuota

	if userQuota > trustQuota {
		if relayInfo.TokenUnlimited || c.GetInt("token_quota") > trustQuota {
			preConsumedQuota = 0
		}
	}

	if preConsumedQuota > 0 {
		if err := PreConsumeTokenQuota(relayInfo, preConsumedQuota); err != nil {
			releasePlatformPreAuth(relayInfo)
			relayInfo.PlatformPreAuthID = 0
			return types.NewErrorWithStatusCode(err, types.ErrorCodePreConsumeTokenQuotaFailed,
				http.StatusPaymentRequired, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		if err := repo.DecreaseUserQuota(relayInfo.UserId, preConsumedQuota); err != nil {
			releasePlatformPreAuth(relayInfo)
			relayInfo.PlatformPreAuthID = 0
			return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
		}
	}

	relayInfo.FinalPreConsumedQuota = preConsumedQuota
	return nil
}

// platformPreAuthorize calls the platform to freeze wallet balance.
// High-balance users can skip this call entirely (cache-based trust).
func platformPreAuthorize(c *gin.Context, estimatedQuota int, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	estimatedLB := float64(estimatedQuota) / common.QuotaPerUnit
	accountID := relayInfo.IdentityAccountID

	// Fast path: skip pre-auth for users with high cached balance.
	// They'll still be charged via settle; this just avoids the synchronous call.
	if common.ShouldSkipPreAuth(accountID, estimatedLB) {
		logger.LogInfo(c, fmt.Sprintf("skipping pre-auth for high-balance account %d (estimated %.4f LB)",
			accountID, estimatedLB))
		return nil
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	preAuthStart := time.Now()
	result, err := common.PreAuthorizeWithBreaker(ctx, accountID, estimatedLB,
		"lurus-api", "", fmt.Sprintf("relay userId=%d model=%s", relayInfo.UserId, relayInfo.OriginModelName), 300)
	metrics.BillingPreAuthDuration.Observe(time.Since(preAuthStart).Seconds())

	if err != nil {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("insufficient balance or billing service unavailable"),
			types.ErrorCodeInsufficientUserQuota, http.StatusPaymentRequired,
			types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog(),
			types.ErrOptionWithTopupURL())
	}

	relayInfo.PlatformPreAuthID = result.PreAuthID
	logger.LogInfo(c, fmt.Sprintf("platform pre-auth created: id=%d amount=%.4f LB account=%d",
		result.PreAuthID, estimatedLB, accountID))
	return nil
}
