package model

import (
	"one-api/common"
)

// BillingTagStatistic 按计费标签分组的统计数据
type BillingTagStatistic struct {
	BillingTag       string `gorm:"column:billing_tag" json:"billing_tag"`
	RequestCount     int64  `gorm:"column:request_count" json:"request_count"`
	Quota            int64  `gorm:"column:quota" json:"quota"`
	PromptTokens     int64  `gorm:"column:prompt_tokens" json:"prompt_tokens"`
	CompletionTokens int64  `gorm:"column:completion_tokens" json:"completion_tokens"`
	RequestTime      int64  `gorm:"column:request_time" json:"request_time"`
}

// ModelUsageByBillingTag 按计费标签和模型分组的使用统计
type ModelUsageByBillingTag struct {
	BillingTag   string `gorm:"column:billing_tag" json:"billing_tag"`
	ModelName    string `gorm:"column:model_name" json:"model_name"`
	RequestCount int64  `gorm:"column:request_count" json:"request_count"`
}

// GetBillingTagStatisticsByPeriod 获取按计费标签分组的统计数据
// 逻辑：先通过token_name关联token表获取billing_tag，如果billing_tag为空，则使用用户的group作为聚合值
func GetBillingTagStatisticsByPeriod(startTime, endTime string) ([]*BillingTagStatistic, error) {
	var statistics []*BillingTagStatistic

	// 根据数据库类型选择JSON提取语法
	var jsonExtract string
	if common.UsingPostgreSQL {
		// PostgreSQL 使用 ->> 操作符提取JSON字段
		jsonExtract = `tokens.setting->>'billing_tag'`
	} else if common.UsingSQLite {
		// SQLite 使用 json_extract 函数
		jsonExtract = `json_extract(tokens.setting, '$.billing_tag')`
	} else {
		// MySQL 使用 JSON_UNQUOTE(JSON_EXTRACT()) 或 ->> 操作符
		jsonExtract = `JSON_UNQUOTE(JSON_EXTRACT(tokens.setting, '$.billing_tag'))`
	}

	// SQL查询逻辑：
	// 1. 从logs表查询消费记录
	// 2. 通过token_name和user_id关联tokens表获取billing_tag
	// 3. 通过user_id关联users表获取group
	// 4. 使用COALESCE：优先使用billing_tag，如果为空或null则使用用户的group
	// 5. 按照最终的标签进行聚合
	query := `
		SELECT 
			COALESCE(NULLIF(` + jsonExtract + `, ''), users.` + getGroupColumn() + `) as billing_tag,
			COUNT(*) as request_count,
			SUM(logs.quota) as quota,
			SUM(logs.prompt_tokens) as prompt_tokens,
			SUM(logs.completion_tokens) as completion_tokens,
			SUM(logs.request_time) as request_time
		FROM logs
		LEFT JOIN tokens ON logs.token_name = tokens.name AND logs.user_id = tokens.user_id AND tokens.deleted_at IS NULL
		INNER JOIN users ON logs.user_id = users.id
		WHERE logs.type = 2
		AND DATE(FROM_UNIXTIME(logs.created_at)) BETWEEN ? AND ?
		GROUP BY billing_tag
		ORDER BY quota DESC
	`

	// 根据数据库类型调整日期转换语法
	if common.UsingPostgreSQL {
		query = `
			SELECT 
				COALESCE(NULLIF(` + jsonExtract + `, ''), users."group") as billing_tag,
				COUNT(*) as request_count,
				SUM(logs.quota) as quota,
				SUM(logs.prompt_tokens) as prompt_tokens,
				SUM(logs.completion_tokens) as completion_tokens,
				SUM(logs.request_time) as request_time
			FROM logs
			LEFT JOIN tokens ON logs.token_name = tokens.name AND logs.user_id = tokens.user_id AND tokens.deleted_at IS NULL
			INNER JOIN users ON logs.user_id = users.id
			WHERE logs.type = 2
			AND TO_TIMESTAMP(logs.created_at)::DATE BETWEEN ?::DATE AND ?::DATE
			GROUP BY billing_tag
			ORDER BY quota DESC
		`
	} else if common.UsingSQLite {
		query = `
			SELECT 
				COALESCE(NULLIF(` + jsonExtract + `, ''), users."group") as billing_tag,
				COUNT(*) as request_count,
				SUM(logs.quota) as quota,
				SUM(logs.prompt_tokens) as prompt_tokens,
				SUM(logs.completion_tokens) as completion_tokens,
				SUM(logs.request_time) as request_time
			FROM logs
			LEFT JOIN tokens ON logs.token_name = tokens.name AND logs.user_id = tokens.user_id AND tokens.deleted_at IS NULL
			INNER JOIN users ON logs.user_id = users.id
			WHERE logs.type = 2
			AND DATE(logs.created_at, 'unixepoch') BETWEEN ? AND ?
			GROUP BY billing_tag
			ORDER BY quota DESC
		`
	}

	err := DB.Raw(query, startTime, endTime).Scan(&statistics).Error
	if err != nil {
		return nil, err
	}

	return statistics, nil
}

// GetModelUsageByBillingTag 获取按计费标签和模型分组的调用次数
func GetModelUsageByBillingTag(startTime, endTime string) ([]*ModelUsageByBillingTag, error) {
	var usage []*ModelUsageByBillingTag

	// 根据数据库类型选择JSON提取语法
	var jsonExtract string
	if common.UsingPostgreSQL {
		jsonExtract = `tokens.setting->>'billing_tag'`
	} else if common.UsingSQLite {
		jsonExtract = `json_extract(tokens.setting, '$.billing_tag')`
	} else {
		jsonExtract = `JSON_UNQUOTE(JSON_EXTRACT(tokens.setting, '$.billing_tag'))`
	}

	query := `
		SELECT 
			COALESCE(NULLIF(` + jsonExtract + `, ''), users.` + getGroupColumn() + `) as billing_tag,
			logs.model_name,
			COUNT(*) as request_count
		FROM logs
		LEFT JOIN tokens ON logs.token_name = tokens.name AND logs.user_id = tokens.user_id AND tokens.deleted_at IS NULL
		INNER JOIN users ON logs.user_id = users.id
		WHERE logs.type = 2
		AND DATE(FROM_UNIXTIME(logs.created_at)) BETWEEN ? AND ?
		GROUP BY billing_tag, logs.model_name
		ORDER BY billing_tag, request_count DESC
	`

	if common.UsingPostgreSQL {
		query = `
			SELECT 
				COALESCE(NULLIF(` + jsonExtract + `, ''), users."group") as billing_tag,
				logs.model_name,
				COUNT(*) as request_count
			FROM logs
			LEFT JOIN tokens ON logs.token_name = tokens.name AND logs.user_id = tokens.user_id AND tokens.deleted_at IS NULL
			INNER JOIN users ON logs.user_id = users.id
			WHERE logs.type = 2
			AND TO_TIMESTAMP(logs.created_at)::DATE BETWEEN ?::DATE AND ?::DATE
			GROUP BY billing_tag, logs.model_name
			ORDER BY billing_tag, request_count DESC
		`
	} else if common.UsingSQLite {
		query = `
			SELECT 
				COALESCE(NULLIF(` + jsonExtract + `, ''), users."group") as billing_tag,
				logs.model_name,
				COUNT(*) as request_count
			FROM logs
			LEFT JOIN tokens ON logs.token_name = tokens.name AND logs.user_id = tokens.user_id AND tokens.deleted_at IS NULL
			INNER JOIN users ON logs.user_id = users.id
			WHERE logs.type = 2
			AND DATE(logs.created_at, 'unixepoch') BETWEEN ? AND ?
			GROUP BY billing_tag, logs.model_name
			ORDER BY billing_tag, request_count DESC
		`
	}

	err := DB.Raw(query, startTime, endTime).Scan(&usage).Error
	if err != nil {
		return nil, err
	}

	return usage, nil
}

// getGroupColumn 返回group列名，处理不同数据库的保留字问题
func getGroupColumn() string {
	if common.UsingPostgreSQL || common.UsingSQLite {
		return `"group"`
	}
	return "`group`"
}

