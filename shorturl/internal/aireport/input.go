package aireport

import (
	"encoding/json"
	"fmt"
	"strings"

	"shorturl/internal/types"
)

// StructuredInput 与模型提示词对齐的结构化输入（序列化为 JSON 传入 GenerateReport）。
type StructuredInput struct {
	ShortURL  string              `json:"shortURL"`
	StartDate string              `json:"startDate"`
	EndDate   string              `json:"endDate"`
	Metrics   types.StatsResponse `json:"metrics"`
}

// FormatStructuredPromptJSON 将 Stats + 区间封装为模型可读的结构化 JSON 字符串。
func FormatStructuredPromptJSON(shortURL, start, end string, m *types.StatsResponse) string {
	if m == nil {
		return ""
	}
	in := StructuredInput{
		ShortURL:  strings.TrimSpace(shortURL),
		StartDate: strings.TrimSpace(start),
		EndDate:   strings.TrimSpace(end),
		Metrics:   *m,
	}
	b, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":"marshal_metrics_failed","shortURL":%q}`, shortURL)
	}
	return string(b)
}
