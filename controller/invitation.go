package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/lurus-api/model"
	"github.com/gin-gonic/gin"
)

// CreateInviteCodeRequest represents the request body for creating invitation codes
type CreateInviteCodeRequest struct {
	Count     int  `json:"count" binding:"required,min=1,max=100"` // Number of codes to create (1-100)
	ExpiresIn *int `json:"expires_in"`                              // Expiration time in seconds, nil means never expires
}

// AdminCreateInviteCodes creates one or more invitation codes
// POST /api/admin/invitation-codes
func AdminCreateInviteCodes(c *gin.Context) {
	var req CreateInviteCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request parameters: " + err.Error(),
		})
		return
	}

	createdBy := c.GetInt("id")
	if createdBy == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User not authenticated",
		})
		return
	}

	codes, codeStrings, err := model.CreateInvitationCodes(createdBy, req.Count, req.ExpiresIn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create invitation codes: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Invitation codes created successfully",
		"data": gin.H{
			"codes":        codeStrings,
			"count":        len(codes),
			"expires_in":   req.ExpiresIn,
			"code_details": codes,
		},
	})
}

// AdminListInviteCodes lists all invitation codes with pagination
// GET /api/admin/invitation-codes?p=1&page_size=10
func AdminListInviteCodes(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	startIdx := (page - 1) * pageSize

	codes, total, err := model.GetAllInvitationCodes(startIdx, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get invitation codes: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    codes,
		"total":   total,
		"page":    page,
	})
}

// AdminSearchInviteCodes searches invitation codes by code prefix
// GET /api/admin/invitation-codes/search?keyword=xxx&p=1&page_size=10
func AdminSearchInviteCodes(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		AdminListInviteCodes(c)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	startIdx := (page - 1) * pageSize

	codes, total, err := model.SearchInvitationCodes(keyword, startIdx, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to search invitation codes: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    codes,
		"total":   total,
		"page":    page,
	})
}

// AdminDeleteInviteCode deletes an unused invitation code
// DELETE /api/admin/invitation-codes/:id
func AdminDeleteInviteCode(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid code ID",
		})
		return
	}

	err = model.DeleteInvitationCode(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Invitation code deleted successfully",
	})
}

// AdminGetInviteCodeStats returns statistics about invitation codes
// GET /api/admin/invitation-codes/stats
func AdminGetInviteCodeStats(c *gin.Context) {
	total, used, expired, active, err := model.GetInvitationCodeStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get statistics: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total":   total,
			"used":    used,
			"expired": expired,
			"active":  active,
		},
	})
}

// AdminCleanupExpiredInviteCodes deletes all expired unused invitation codes
// DELETE /api/admin/invitation-codes/cleanup
func AdminCleanupExpiredInviteCodes(c *gin.Context) {
	deleted, err := model.DeleteExpiredInvitationCodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to cleanup expired codes: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Expired invitation codes cleaned up",
		"data": gin.H{
			"deleted": deleted,
		},
	})
}

// ValidateInviteCode validates an invitation code (public endpoint)
// GET /api/invitation/validate?code=xxx
func ValidateInviteCode(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"valid":   false,
			"message": "Code is required",
		})
		return
	}

	valid := model.ValidateInvitationCode(code)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"valid":   valid,
	})
}
