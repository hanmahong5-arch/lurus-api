package search

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/lurus-api/common"
	"github.com/meilisearch/meilisearch-go"
)

// ChannelDocument represents a channel document in Meilisearch
// 表示 Meilisearch 中的通道文档
type ChannelDocument struct {
	ID       int     `json:"id"`
	Type     int     `json:"type"`
	Name     string  `json:"name"`
	BaseURL  string  `json:"base_url"`
	Models   string  `json:"models"`
	Group    string  `json:"group"`
	Tag      string  `json:"tag"`
	Status   int     `json:"status"`
	Priority int64   `json:"priority"`
	Balance  float64 `json:"balance"`
	TestTime int64   `json:"test_time"`
}

// Channel represents the channel model (to avoid circular import)
// 表示通道模型(避免循环导入)
type Channel struct {
	Id       int
	Type     int
	Name     string
	BaseURL  *string
	Models   string
	Group    string
	Tag      *string
	Status   int
	Priority *int64
	Balance  float64
	TestTime int64
}

// ConvertChannelToDocument converts a Channel to ChannelDocument
// 将 Channel 转换为 ChannelDocument
func ConvertChannelToDocument(channel *Channel) *ChannelDocument {
	doc := &ChannelDocument{
		ID:       channel.Id,
		Type:     channel.Type,
		Name:     channel.Name,
		Models:   channel.Models,
		Group:    channel.Group,
		Status:   channel.Status,
		Balance:  channel.Balance,
		TestTime: channel.TestTime,
	}

	if channel.BaseURL != nil {
		doc.BaseURL = *channel.BaseURL
	}
	if channel.Tag != nil {
		doc.Tag = *channel.Tag
	}
	if channel.Priority != nil {
		doc.Priority = *channel.Priority
	}

	return doc
}

// IndexChannel indexes a single channel
// 索引单个通道
func IndexChannel(channel *Channel) error {
	if !IsEnabled() {
		return nil
	}

	doc := ConvertChannelToDocument(channel)
	indexName := GetIndexName(IndexChannels)
	index := Client.Index(indexName)

	_, err := index.AddDocuments([]ChannelDocument{*doc}, nil)
	if err != nil {
		if Debug {
			common.SysLog(fmt.Sprintf("Failed to index channel %d: %v", channel.Id, err))
		}
		return fmt.Errorf("failed to index channel: %w", err)
	}

	return nil
}

// SearchChannels searches channels with keyword
// 使用关键词搜索通道
func SearchChannels(keyword string, group string, status int, page, pageSize int) ([]map[string]interface{}, int64, error) {
	if !IsEnabled() {
		return nil, 0, fmt.Errorf("Meilisearch is not enabled")
	}

	indexName := GetIndexName(IndexChannels)
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
		return nil, 0, fmt.Errorf("failed to search channels: %w", err)
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

// DeleteChannelByID deletes a channel from search index
// 从搜索索引删除通道
func DeleteChannelByID(channelID int) error {
	if !IsEnabled() {
		return nil
	}

	indexName := GetIndexName(IndexChannels)
	index := Client.Index(indexName)

	_, err := index.DeleteDocument(fmt.Sprintf("%d", channelID), nil)
	if err != nil {
		return fmt.Errorf("failed to delete channel: %w", err)
	}

	return nil
}
