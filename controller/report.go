package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// parseCSVInts parses a comma-separated list of ints from a query string,
// returning nil if the input is empty. Invalid entries are silently dropped
// to keep the API forgiving (callers only see the filtered set).
func parseCSVInts(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := make([]int, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.Atoi(part)
		if err != nil || v == 0 {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// GetReportStats handles GET /api/report/stats.
// Returns aggregated token/quota/request counts for a filtered time range.
func GetReportStats(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channelIds := parseCSVInts(c.Query("channel_ids"))
	userIds := parseCSVInts(c.Query("user_ids"))
	group := c.Query("group")

	stat, err := model.GetReportStats(startTimestamp, endTimestamp, channelIds, userIds, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stat,
	})
}

// GetReportTopChannels handles GET /api/report/top/channels.
// Returns the top-N channels by total token consumption.
func GetReportTopChannels(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	rows, err := model.GetTopChannels(startTimestamp, endTimestamp, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}

// GetReportTopUsers handles GET /api/report/top/users.
// Returns the top-N users by total token consumption.
func GetReportTopUsers(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	rows, err := model.GetTopUsers(startTimestamp, endTimestamp, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}
