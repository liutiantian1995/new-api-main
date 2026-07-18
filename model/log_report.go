package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ReportStat carries aggregated token/quota stats for a filtered time range.
// Unlike Stat (which is used for the dashboard "rpm/tpm" overlay), ReportStat
// is the response shape for the admin reports endpoint.
type ReportStat struct {
	Quota            int `json:"quota"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	CachedTokens     int `json:"cached_tokens"`
	TotalTokens      int `json:"total_tokens"`
	RequestCount     int `json:"request_count"`
}

// TopChannelRow is one row in the channel top-N report.
type TopChannelRow struct {
	ChannelID        int    `json:"channel_id" gorm:"column:channel_id"`
	ChannelName      string `json:"channel_name" gorm:"column:channel_name"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"column:prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens" gorm:"column:completion_tokens"`
	CachedTokens     int    `json:"cached_tokens" gorm:"column:cached_tokens"`
	TotalTokens      int    `json:"total_tokens" gorm:"column:total_tokens"`
	Quota            int    `json:"quota" gorm:"column:quota"`
	RequestCount     int    `json:"request_count" gorm:"column:request_count"`
}

// TopUserRow is one row in the user top-N report.
type TopUserRow struct {
	UserID           int    `json:"user_id" gorm:"column:user_id"`
	Username         string `json:"username" gorm:"column:username"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"column:prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens" gorm:"column:completion_tokens"`
	CachedTokens     int    `json:"cached_tokens" gorm:"column:cached_tokens"`
	TotalTokens      int    `json:"total_tokens" gorm:"column:total_tokens"`
	Quota            int    `json:"quota" gorm:"column:quota"`
	RequestCount     int    `json:"request_count" gorm:"column:request_count"`
}

// cacheScanRow is the subset of log columns needed for cache-token aggregation.
type cacheScanRow struct {
	Other string `gorm:"column:other"`
}

// GetReportStats returns aggregated stats for a filtered time range.
// channelIds and userIds are optional (nil/empty = no filter).
// CachedTokens is computed in Go (cross-dialect JSON extraction is brittle).
func GetReportStats(startTimestamp, endTimestamp int64, channelIds []int, userIds []int, group string) (ReportStat, error) {
	var stat ReportStat
	tx := LOG_DB.Table("logs").
		Select("COALESCE(SUM(quota), 0) AS quota, "+
			"COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, "+
			"COALESCE(SUM(completion_tokens), 0) AS completion_tokens, "+
			"COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) AS total_tokens, "+
			"COUNT(*) AS request_count").
		Where("type = ?", LogTypeConsume)

	tx = applyReportFilters(tx, startTimestamp, endTimestamp, channelIds, userIds, group)

	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query report stats: " + err.Error())
		return stat, err
	}
	stat.CachedTokens = sumCachedTokensForFilter(startTimestamp, endTimestamp, channelIds, userIds, group)
	return stat, nil
}

// GetTopChannels returns the top-N channels by total token consumption.
// channelName is fetched via LEFT JOIN channels on channel_id = id.
func GetTopChannels(startTimestamp, endTimestamp int64, limit int) ([]TopChannelRow, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	var rows []TopChannelRow
	err := LOG_DB.Table("logs").
		Select("channel_id, "+
			"COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, "+
			"COALESCE(SUM(completion_tokens), 0) AS completion_tokens, "+
			"COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) AS total_tokens, "+
			"COALESCE(SUM(quota), 0) AS quota, "+
			"COUNT(*) AS request_count").
		Where("type = ?", LogTypeConsume).
		Where("channel_id != 0").
		Where("created_at >= ?", startTimestamp).
		Where("created_at <= ?", endTimestamp).
		Group("channel_id").
		Order("total_tokens DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		common.SysError("failed to query top channels: " + err.Error())
		return nil, err
	}
	// Resolve channel names via a single batched query against the main DB.
	channelIds := make([]int, 0, len(rows))
	for _, r := range rows {
		channelIds = append(channelIds, r.ChannelID)
	}
	nameByID, _ := batchGetChannelNames(channelIds)
	for i := range rows {
		if name, ok := nameByID[rows[i].ChannelID]; ok {
			rows[i].ChannelName = name
		}
	}
	// Cache tokens: scan matching rows per channel.
	populateTopCachedTokensChannel(&rows, startTimestamp, endTimestamp)
	return rows, nil
}

// GetTopUsers returns the top-N users by total token consumption.
func GetTopUsers(startTimestamp, endTimestamp int64, limit int) ([]TopUserRow, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	var rows []TopUserRow
	err := LOG_DB.Table("logs").
		Select("user_id, "+
			"MAX(username) AS username, "+
			"COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, "+
			"COALESCE(SUM(completion_tokens), 0) AS completion_tokens, "+
			"COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) AS total_tokens, "+
			"COALESCE(SUM(quota), 0) AS quota, "+
			"COUNT(*) AS request_count").
		Where("type = ?", LogTypeConsume).
		Where("user_id != 0").
		Where("created_at >= ?", startTimestamp).
		Where("created_at <= ?", endTimestamp).
		Group("user_id").
		Order("total_tokens DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		common.SysError("failed to query top users: " + err.Error())
		return nil, err
	}
	populateTopCachedTokensUser(&rows, startTimestamp, endTimestamp)
	return rows, nil
}

// applyReportFilters appends the shared filter set (time, channel, user, group)
// to a logs-table query.
func applyReportFilters(tx *gorm.DB, startTimestamp, endTimestamp int64, channelIds []int, userIds []int, group string) *gorm.DB {
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if len(channelIds) > 0 {
		tx = tx.Where("channel_id IN ?", channelIds)
	}
	if len(userIds) > 0 {
		tx = tx.Where("user_id IN ?", userIds)
	}
	if group != "" {
		tx = tx.Where(logGroupCol + " = ?", group)
	}
	return tx
}

// sumCachedTokensForFilter scans matching log rows' `other` JSON and returns
// the sum of cache tokens detected (OpenAI prompt_tokens_details.cached_tokens
// or Claude cache_read_input_tokens + cache_creation_input_tokens).
// Single-pass scan, dialect-agnostic.
func sumCachedTokensForFilter(startTimestamp, endTimestamp int64, channelIds []int, userIds []int, group string) int {
	return scanCachedTokens(startTimestamp, endTimestamp, channelIds, userIds, group, nil)
}

// scanCachedTokens runs the cache scan and optionally accumulates per-key totals.
// When accumulator is non-nil, it is keyed by the ID dimension chosen by the
// caller (channel_id when called from the channel top-N path; user_id from
// the user top-N path). Otherwise, only the grand total is returned.
//
// All filter arguments MUST be honored — both the global filter set
// (channelIds/userIds/group) AND the per-dimension ID restriction
// (channelIds/userIds non-empty) so we do not scan the entire time range.
func scanCachedTokens(startTimestamp, endTimestamp int64, channelIds []int, userIds []int, group string, accumulator map[int]int) int {
	type row struct {
		Other     string `gorm:"column:other"`
		ChannelID int    `gorm:"column:channel_id"`
		UserID    int    `gorm:"column:user_id"`
	}
	var rows []row
	tx := LOG_DB.Table("logs").
		Select("other, channel_id, user_id").
		Where("type = ?", LogTypeConsume).
		Where("other != ''")
	tx = applyReportFilters(tx, startTimestamp, endTimestamp, channelIds, userIds, group)
	if err := tx.Scan(&rows).Error; err != nil {
		common.SysLog("failed to scan logs for cache tokens: " + err.Error())
		return 0
	}
	total := 0
	for _, r := range rows {
		c := extractCacheTokens(r.Other)
		if c == 0 {
			continue
		}
		total += c
		if accumulator != nil {
			// Caller chooses which dimension to accumulate by passing
			// either channelIds or userIds above. We mirror that here
			// by checking which ID field was requested.
			if len(channelIds) > 0 && r.ChannelID != 0 {
				accumulator[r.ChannelID] += c
			} else if len(userIds) > 0 && r.UserID != 0 {
				accumulator[r.UserID] += c
			}
		}
	}
	return total
}

// populateTopCachedTokensChannel fills CachedTokens per row.
// IMPORTANT: scans only logs whose channel_id matches one of the rows
// already in `rows`, never the entire time range. This bounds the scan
// to at most len(rows) channel buckets.
func populateTopCachedTokensChannel(rows *[]TopChannelRow, startTimestamp, endTimestamp int64) {
	if len(*rows) == 0 {
		return
	}
	channelIds := make([]int, 0, len(*rows))
	for _, r := range *rows {
		channelIds = append(channelIds, r.ChannelID)
	}
	perChannel := make(map[int]int)
	scanCachedTokens(startTimestamp, endTimestamp, channelIds, nil, "", perChannel)
	for i := range *rows {
		(*rows)[i].CachedTokens = perChannel[(*rows)[i].ChannelID]
	}
}

// populateTopCachedTokensUser fills CachedTokens per row.
// IMPORTANT: scans only logs whose user_id matches one of the rows
// already in `rows`, never the entire time range. This bounds the scan
// to at most len(rows) user buckets.
func populateTopCachedTokensUser(rows *[]TopUserRow, startTimestamp, endTimestamp int64) {
	if len(*rows) == 0 {
		return
	}
	userIds := make([]int, 0, len(*rows))
	for _, r := range *rows {
		userIds = append(userIds, r.UserID)
	}
	perUser := make(map[int]int)
	scanCachedTokens(startTimestamp, endTimestamp, nil, userIds, "", perUser)
	for i := range *rows {
		(*rows)[i].CachedTokens = perUser[(*rows)[i].UserID]
	}
}

// extractCacheTokens parses one log's `other` JSON field and returns the
// detected cache tokens (sum across OpenAI-format and Claude-format fields).
// Returns 0 on parse failure or missing fields.
func extractCacheTokens(otherJSON string) int {
	if otherJSON == "" {
		return 0
	}
	var raw map[string]any
	if err := common.Unmarshal([]byte(otherJSON), &raw); err != nil {
		return 0
	}
	usage, ok := raw["usage"].(map[string]any)
	if !ok {
		return 0
	}
	sum := 0
	// OpenAI: usage.prompt_tokens_details.cached_tokens
	if details, ok := usage["prompt_tokens_details"].(map[string]any); ok {
		if v, ok := details["cached_tokens"].(float64); ok {
			sum += int(v)
		}
	}
	// Claude: usage.cache_read_input_tokens + cache_creation_input_tokens
	if v, ok := usage["cache_read_input_tokens"].(float64); ok {
		sum += int(v)
	}
	if v, ok := usage["cache_creation_input_tokens"].(float64); ok {
		sum += int(v)
	}
	return sum
}

// batchGetChannelNames fetches names for a list of channel IDs in one query.
// Returns a map keyed by channel ID. Missing channels are omitted.
func batchGetChannelNames(ids []int) (map[int]string, error) {
	if len(ids) == 0 {
		return map[int]string{}, nil
	}
	var channels []Channel
	err := DB.Select("id, name").Where("id IN ?", ids).Find(&channels).Error
	if err != nil {
		return map[int]string{}, err
	}
	out := make(map[int]string, len(channels))
	for _, c := range channels {
		out[c.Id] = c.Name
	}
	return out, nil
}
