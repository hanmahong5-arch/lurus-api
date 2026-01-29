package search

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
	"github.com/meilisearch/meilisearch-go"
)

// UserDocument represents a user document in Meilisearch
// 表示 Meilisearch 中的用户文档
type UserDocument struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        int    `json:"role"`
	Status      int    `json:"status"`
	Quota       int    `json:"quota"`
	UsedQuota   int    `json:"used_quota"`
	Group       string `json:"group"`
	CreatedTime int64  `json:"created_time"`
}

// User represents the user model (to avoid circular import)
// 表示用户模型(避免循环导入)
type User struct {
	Id          int
	Username    string
	Email       string
	DisplayName string
	Role        int
	Status      int
	Quota       int
	UsedQuota   int
	Group       string
}

// ConvertUserToDocument converts a User to UserDocument
// 将 User 转换为 UserDocument
func ConvertUserToDocument(user *User) *UserDocument {
	return &UserDocument{
		ID:          user.Id,
		Username:    user.Username,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Status:      user.Status,
		Quota:       user.Quota,
		UsedQuota:   user.UsedQuota,
		Group:       user.Group,
		CreatedTime: 0, // Not available in User model
	}
}

// IndexUser indexes a single user
// 索引单个用户
func IndexUser(user *User) error {
	if !IsEnabled() {
		return nil
	}

	doc := ConvertUserToDocument(user)
	indexName := GetIndexName(IndexUsers)
	index := Client.Index(indexName)

	_, err := index.AddDocuments([]UserDocument{*doc}, nil)
	if err != nil {
		if Debug {
			common.SysLog(fmt.Sprintf("Failed to index user %d: %v", user.Id, err))
		}
		return fmt.Errorf("failed to index user: %w", err)
	}

	return nil
}

// SearchUsers searches users with keyword
// 使用关键词搜索用户
func SearchUsers(keyword string, group string, status int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	if !IsEnabled() {
		return nil, 0, fmt.Errorf("Meilisearch is not enabled")
	}

	indexName := GetIndexName(IndexUsers)
	index := Client.Index(indexName)

	// Build filter
	// 构建过滤器
	var filters []string
	if group != "" {
		filters = append(filters, fmt.Sprintf("group = \"%s\"", escapeFilterString(group)))
	}
	if status > 0 {
		filters = append(filters, fmt.Sprintf("status = %d", status))
	}
	filterStr := strings.Join(filters, " AND ")

	// Calculate offset
	// 计算偏移量
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	// Build search request
	// 构建搜索请求
	searchReq := &meilisearch.SearchRequest{
		Limit:  int64(pageSize),
		Offset: int64(offset),
	}

	if filterStr != "" {
		searchReq.Filter = filterStr
	}

	// Execute search
	// 执行搜索
	searchResp, err := index.Search(keyword, searchReq)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search users: %w", err)
	}

	// Convert results
	// 转换结果
	results := make([]map[string]interface{}, 0, len(searchResp.Hits))
	for _, hit := range searchResp.Hits {
		var hitMap map[string]interface{}
		if err := hit.DecodeInto(&hitMap); err == nil {
			results = append(results, hitMap)
		}
	}

	total := searchResp.EstimatedTotalHits
	if searchResp.TotalHits > 0 {
		total = searchResp.TotalHits
	}

	return results, total, nil
}

// DeleteUserByID deletes a user from search index
// 从搜索索引删除用户
func DeleteUserByID(userID int) error {
	if !IsEnabled() {
		return nil
	}

	indexName := GetIndexName(IndexUsers)
	index := Client.Index(indexName)

	_, err := index.DeleteDocument(fmt.Sprintf("%d", userID), nil)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}
