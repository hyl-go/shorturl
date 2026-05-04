package ai

import "strings"

// NormalizeReportResult 将 AI JSON 中可能为 null 的数组字段规范为非 nil，便于前端序列化。
func NormalizeReportResult(r *ReportResult) {
	if r.Trends == nil {
		r.Trends = []string{}
	}
	if r.Anomalies == nil {
		r.Anomalies = []string{}
	}
	if r.Suggestions == nil {
		r.Suggestions = []string{}
	}
}

// FallbackStatsReport 在大模型不可用或解析失败时使用的结构化占位报告。
func FallbackStatsReport(statsData string) *ReportResult {
	if strings.TrimSpace(statsData) == "" {
		statsData = "（无摘要输入）"
	}
	return &ReportResult{
		Summary:     "已根据当前 PV/UV 与端侧占比生成简要结论（大模型未配置或调用失败时显示此条）。",
		Trends:      []string{statsData},
		Anomalies:   []string{"无明显异常（请以图表与明细为准）"},
		Suggestions: []string{"拉长统计区间对比趋势", "关注移动端占比与地域分布变化"},
	}
}
