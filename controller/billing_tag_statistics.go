package controller

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"one-api/common"
	"one-api/model"
	"time"

	"github.com/gin-gonic/gin"
)

type BillingTagStatsRequest struct {
	StartTime string `form:"start_time" binding:"required"`
	EndTime   string `form:"end_time" binding:"required"`
}

// GetBillingTagStatistics 获取按计费标签分组的统计数据
func GetBillingTagStatistics(c *gin.Context) {
	var req BillingTagStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("invalid parameters: %v", err))
		return
	}

	// Validate date format
	if _, err := time.Parse("2006-01-02", req.StartTime); err != nil {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("invalid start_time format, expected YYYY-MM-DD"))
		return
	}
	if _, err := time.Parse("2006-01-02", req.EndTime); err != nil {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("invalid end_time format, expected YYYY-MM-DD"))
		return
	}

	// Get statistics
	statistics, err := model.GetBillingTagStatisticsByPeriod(req.StartTime, req.EndTime)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// Get model usage data
	modelUsage, err := model.GetModelUsageByBillingTag(req.StartTime, req.EndTime)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "",
		"data":        statistics,
		"model_usage": modelUsage,
	})
}

// ExportBillingTagStatisticsCSV 导出按计费标签分组的统计数据为CSV
func ExportBillingTagStatisticsCSV(c *gin.Context) {
	var req BillingTagStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("invalid parameters: %v", err))
		return
	}

	// Validate date format
	if _, err := time.Parse("2006-01-02", req.StartTime); err != nil {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("invalid start_time format, expected YYYY-MM-DD"))
		return
	}
	if _, err := time.Parse("2006-01-02", req.EndTime); err != nil {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("invalid end_time format, expected YYYY-MM-DD"))
		return
	}

	// Get statistics
	statistics, err := model.GetBillingTagStatisticsByPeriod(req.StartTime, req.EndTime)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// Set headers for CSV download
	filename := fmt.Sprintf("billing_tag_stats_%s_%s.csv", req.StartTime, req.EndTime)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Write UTF-8 BOM for Excel compatibility
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	// Create CSV writer
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Write CSV header
	header := []string{
		"计费标签",
		"请求次数",
		"额度消耗",
		"输入Tokens",
		"输出Tokens",
		"请求时长(ms)",
	}
	if err := writer.Write(header); err != nil {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("failed to write CSV header: %v", err))
		return
	}

	// Write data rows
	for _, stat := range statistics {
		row := []string{
			stat.BillingTag,
			fmt.Sprintf("%d", stat.RequestCount),
			fmt.Sprintf("%d", stat.Quota),
			fmt.Sprintf("%d", stat.PromptTokens),
			fmt.Sprintf("%d", stat.CompletionTokens),
			fmt.Sprintf("%d", stat.RequestTime),
		}
		if err := writer.Write(row); err != nil {
			common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("failed to write CSV row: %v", err))
			return
		}
	}
}

