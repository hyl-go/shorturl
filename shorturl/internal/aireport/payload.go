package aireport

// TaskPayload 入队时携带；Worker 内据 StatsJSON 再次解析，与 HTTP 层统计结果一致。
type TaskPayload struct {
	JobID     string `json:"jobId"`
	ShortURL  string `json:"shortURL"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	// StatsJSON 为 types.StatsResponse 的 JSON，供 Prompt 使用结构化指标
	StatsJSON string `json:"statsJSON"`
}
