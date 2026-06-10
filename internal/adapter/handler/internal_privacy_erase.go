package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app/governance"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Audit actions for the PIPL §47 erasure flow.
const (
	AuditActionErasureRequested = "privacy.erasure.requested"
)

// InternalPrivacyErase is the PIPL §47 account-erasure intake endpoint.
// lurus-platform calls it after the account-deletion cooling-off period
// expires (newhub has no NATS consumer — the trigger is internal HTTP,
// same pattern as SEAM S1 fund).
//
// Route:  POST /internal/v1/privacy/erase
// Scope:  user:delete (platform's internal key already carries it)
//
// Request body (JSON):
//
//	{
//	  "event_id":   "<platform erasure event UUID>",  // idempotency key
//	  "account_id": 100,                               // platform account integer id
//	  "request_id": "...",                             // optional, tracing
//	  "reason":     "user_requested"                   // optional, stored verbatim
//	}
//
// Behaviour:
//   - unknown account (no newhub user bound) → durable no_data row + 200
//   - replayed event_id → 200 with replayed:true and the original row's status
//   - first call for a bound user → intent row + synchronous containment
//     (disable user + disable all tokens) → 202; the batched background
//     executor (internal/lifecycle/privacy_erasure.go) runs the cascade
//
// Honest scope boundary (also in contracts.md): PG backups/WAL, Meilisearch
// snapshots, already-shipped stdout logs and upstream LLM vendor logs are NOT
// covered; logs are pseudonymized (billing fields retained), not deleted.
func InternalPrivacyErase(c *gin.Context) {
	var req struct {
		EventID   string `json:"event_id"`
		AccountID int64  `json:"account_id"`
		RequestID string `json:"request_id"`
		Reason    string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "invalid request body: " + err.Error(),
			"error_code": "INVALID_REQUEST",
		})
		return
	}
	if req.EventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "event_id is required: pass the platform erasure event UUID for idempotency",
			"error_code": "MISSING_EVENT_ID",
		})
		return
	}
	if req.AccountID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    fmt.Sprintf("account_id must be a positive integer, got %d", req.AccountID),
			"error_code": "INVALID_ACCOUNT_ID",
		})
		return
	}

	// Resolve the newhub user. No binding → durable no_data evidence row so
	// the platform gets a queryable answer, not a transient 404.
	user, lookupErr := repo.GetUserByLurusAccountID(req.AccountID)
	if lookupErr != nil {
		if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			row, createErr := repo.CreateErasureRequestIdempotent(
				c.Request.Context(),
				req.EventID, req.AccountID, req.RequestID, req.Reason,
				0, "", repo.ErasureStatusNoData,
			)
			replayed := errors.Is(createErr, repo.ErrErasureEventExists)
			if createErr != nil && !replayed {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success":    false,
					"message":    "failed to record no_data erasure: " + createErr.Error(),
					"error_code": "ERASURE_RECORD_FAILED",
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data":    erasureResponse(row, replayed),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"message":    "user lookup failed: " + lookupErr.Error(),
			"error_code": "USER_LOOKUP_FAILED",
		})
		return
	}

	row, createErr := repo.CreateErasureRequestIdempotent(
		c.Request.Context(),
		req.EventID, req.AccountID, req.RequestID, req.Reason,
		user.Id, user.TenantId, repo.ErasureStatusPending,
	)
	if replayed := errors.Is(createErr, repo.ErrErasureEventExists); replayed {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    erasureResponse(row, true),
		})
		return
	} else if createErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"message":    "failed to record erasure intent: " + createErr.Error(),
			"error_code": "ERASURE_RECORD_FAILED",
		})
		return
	}

	// Synchronous containment: stop new spend/auth immediately; the data
	// cascade itself runs in the background executor. Failures here are
	// logged but do not fail the intake — the executor's token hard-delete
	// supersedes the disable anyway.
	if err := repo.DisableUserById(user.Id); err != nil {
		common.SysError(fmt.Sprintf(
			`{"event":"privacy_erase_disable_failed","who":"account:%d/user:%d","what":"disable user at erasure intake","result":"failed: %s"}`,
			req.AccountID, user.Id, err.Error()))
	}
	if n, err := repo.DisableTokensByUserID(user.Id); err != nil {
		common.SysError(fmt.Sprintf(
			`{"event":"privacy_erase_disable_failed","who":"account:%d/user:%d","what":"disable tokens at erasure intake","result":"failed: %s"}`,
			req.AccountID, user.Id, err.Error()))
	} else {
		common.SysLog(fmt.Sprintf(
			`{"event":"privacy_erase_accepted","who":"account:%d/user:%d","what":"erasure intent %s recorded","result":"user disabled, %d tokens disabled, cascade queued"}`,
			req.AccountID, user.Id, req.EventID, n))
	}

	auditEvent := governance.NewAuditEvent(c, "system", 0,
		AuditActionErasureRequested, "user", user.Id,
		fmt.Sprintf(`{"event_id":%q,"account_id":%d}`, req.EventID, req.AccountID))
	governance.RecordAuditEvent(auditEvent)

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"data":    erasureResponse(row, false),
	})
}

// InternalGetPrivacyErasure lets the platform poll erasure progress and pull
// compliance evidence.
//
// Route:  GET /internal/v1/privacy/erase/:event_id
// Scope:  user:read
func InternalGetPrivacyErasure(c *gin.Context) {
	eventID := c.Param("event_id")
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "event_id path parameter is required",
			"error_code": "MISSING_EVENT_ID",
		})
		return
	}

	row, err := repo.GetErasureRequestByEventID(eventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success":    false,
				"message":    "no erasure request recorded for event_id '" + eventID + "'",
				"error_code": "ERASURE_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"message":    "erasure lookup failed: " + err.Error(),
			"error_code": "ERASURE_LOOKUP_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    erasureResponse(row, false),
	})
}

// erasureResponse shapes the wire payload for both endpoints (flat per the
// API-contract-minimalism rule). user_id is intentionally omitted — the
// platform keys on event_id/account_id and the internal id is pseudonymous
// ledger detail.
func erasureResponse(row *entity.PrivacyErasureRequest, replayed bool) gin.H {
	resp := gin.H{
		"event_id":      row.EventID,
		"account_id":    row.AccountID,
		"status":        row.Status,
		"step":          row.Step,
		"logs_scrubbed": row.LogsScrubbed,
		"replayed":      replayed,
		"created_at":    row.CreatedAt,
	}
	if row.CompletedAt != nil {
		resp["completed_at"] = row.CompletedAt
	}
	if row.LastError != "" {
		resp["last_error"] = row.LastError
	}
	return resp
}
