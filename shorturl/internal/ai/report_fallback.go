package ai

import "strings"

// BuildMarkdownReport 从结构化字段拼装 Markdown，供 LLM 未返回 markdown 或解析后为空时回填。
func BuildMarkdownReport(r *ReportResult) string {
	if r == nil {
		return ""
	}
	title := strings.TrimSpace(r.Title)
	if title == "" {
		title = "短链访问分析报告"
	}
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n\n")
	if s := strings.TrimSpace(r.Summary); s != "" {
		b.WriteString("## 摘要\n\n")
		b.WriteString(s)
		b.WriteString("\n\n")
	}
	appendBulletSection(&b, "## 趋势", r.Trends)
	appendBulletSection(&b, "## 异常", r.Anomalies)
	appendBulletSection(&b, "## 建议", r.Suggestions)
	return strings.TrimSpace(b.String())
}

func appendBulletSection(b *strings.Builder, heading string, items []string) {
	empty := true
	for _, it := range items {
		if strings.TrimSpace(it) != "" {
			empty = false
			break
		}
	}
	if empty {
		return
	}
	b.WriteString(heading)
	b.WriteString("\n\n")
	for _, it := range items {
		if t := strings.TrimSpace(it); t != "" {
			b.WriteString("- ")
			b.WriteString(t)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
}

// NormalizeReportResult 将 AI JSON 中可能为 null 的数组字段规范为非 nil，便于前端序列化。
func NormalizeReportResult(r *ReportResult) {
	if strings.TrimSpace(r.Title) == "" {
		r.Title = "短链访问分析报告"
	}
	if r.Trends == nil {
		r.Trends = []string{}
	}
	if r.Anomalies == nil {
		r.Anomalies = []string{}
	}
	if r.Suggestions == nil {
		r.Suggestions = []string{}
	}
	if strings.TrimSpace(r.Markdown) == "" {
		r.Markdown = BuildMarkdownReport(r)
	}
}

// FallbackStatsReport 在大模型不可用或解析失败时使用的结构化占位报告。
func FallbackStatsReport(statsData string) *ReportResult {
	if strings.TrimSpace(statsData) == "" {
		statsData = "（无摘要输入）"
	}
	return &ReportResult{
		Title:       "短链访问分析报告",
		Summary:     "已根据当前 PV/UV 与端侧占比生成简要结论（大模型未配置或调用失败时显示此条）。",
		Trends:      []string{statsData},
		Anomalies:   []string{"无明显异常（请以图表与明细为准）"},
		Suggestions: []string{"拉长统计区间对比趋势", "关注移动端占比与地域分布变化"},
		Markdown:    "",
	}
}
