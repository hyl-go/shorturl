package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"shorturl/internal/config"
	"shorturl/internal/types"

	"github.com/zeromicro/go-zero/core/breaker"
)

type DeepSeekProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewDeepSeekProvider(cfg config.AIProviderConfig) *DeepSeekProvider {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &DeepSeekProvider{
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		model:   cfg.Model,
		client:  &http.Client{Timeout: timeout},
	}
}

func (d *DeepSeekProvider) Name() string {
	return "deepseek"
}

func (d *DeepSeekProvider) AnalyzeURL(ctx context.Context, url, title, desc string) (*AnalyzeResult, error) {
	if d.apiKey == "" || d.baseURL == "" || d.model == "" {
		return d.ruleBasedAnalyze(url, title, desc), nil
	}
	prompt := fmt.Sprintf(`请分析以下网页链接，返回 JSON：
{
  "suggestions": ["建议1","建议2","建议3"],
  "category": "分类",
  "safety_level": "safe|suspicious|dangerous",
  "safety_reason": "原因"
}
URL: %s
标题: %s
描述: %s
仅返回 JSON，不要附加说明。`, url, title, desc)

	reqBody := map[string]any{
		"model": d.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.3,
	}
	raw, err := d.callChatCompletions(ctx, reqBody)
	if err != nil {
		if errors.Is(err, breaker.ErrServiceUnavailable) {
			return d.ruleBasedAnalyze(url, title, desc), nil
		}
		return nil, err
	}

	var result AnalyzeResult
	if err := json.Unmarshal([]byte(extractJSON(raw)), &result); err != nil {
		return nil, fmt.Errorf("decode analyze result failed: %w", err)
	}
	result.PageTitle = title
	result.PageDesc = desc
	return &result, nil
}

func (d *DeepSeekProvider) GenerateReport(ctx context.Context, statsData string) (*ReportResult, error) {
	statsData = strings.TrimSpace(statsData)
	if statsData == "" {
		statsData = "（无摘要输入）"
	}
	if d.apiKey == "" || d.baseURL == "" || d.model == "" {
		return FallbackStatsReport(statsData), nil
	}

	var prompt string
	if structuredStatsPayload(statsData) {
		prompt = buildStructuredAnalysisPrompt(statsData)
	} else {
		prompt = fmt.Sprintf(`你是资深数据分析师。根据下方「短链访问统计摘要」，输出严格 JSON（不要 markdown 围栏外文字、不要代码块标记、仅一行根 JSON 对象）：
{
  "title":"短链访问分析报告",
  "summary":"1-3句总体结论，必须可执行",
  "trends":["趋势要点1","趋势要点2"],
  "anomalies":["异常或风险点；若无填 无明显异常"],
  "suggestions":["可执行建议1","可执行建议2"],
  "markdown":"完整 Markdown 报告正文，包含 #标题、##概述、##趋势洞察、##异常与风险、##建议动作 五段"
}
硬性约束：
1) JSON 必须包含且仅使用以下键：title, summary, trends, anomalies, suggestions, markdown；
2) trends/anomalies/suggestions 均为字符串数组，数组长度 1～8；
3) markdown 中的量化表述须能在下方摘要中找到依据；
4) 禁止编造访问次数、国家或日期。
统计摘要：
%s`, statsData)
	}

	reqBody := map[string]any{
		"model": d.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.25,
	}
	raw, err := d.callChatCompletions(ctx, reqBody)
	if err != nil {
		return FallbackStatsReport(statsData), nil
	}
	chunk := extractJSON(raw)
	if !validateReportJSONKeys(chunk) {
		return FallbackStatsReport(statsData), nil
	}
	var r ReportResult
	if err := json.Unmarshal([]byte(chunk), &r); err != nil {
		return FallbackStatsReport(statsData), nil
	}
	NormalizeReportResult(&r)
	if strings.TrimSpace(r.Summary) == "" {
		return FallbackStatsReport(statsData), nil
	}
	return &r, nil
}

func structuredStatsPayload(s string) bool {
	var v struct {
		Metrics types.StatsResponse `json:"metrics"`
	}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return false
	}
	return len(v.Metrics.ChartData) > 0 || v.Metrics.TotalPV > 0 || v.Metrics.TotalUV > 0 ||
		len(v.Metrics.GeoStats) > 0 || len(v.Metrics.DeviceStats.Breakdown) > 0 ||
		len(v.Metrics.GeoByCountry) > 0 || len(v.Metrics.GeoByRegion) > 0
}

func buildStructuredAnalysisPrompt(structuredJSON string) string {
	return fmt.Sprintf(`你是资深增长与数据分析师。输入为「短链访问统计」的结构化 JSON（含 metrics：总 PV/UV、按日序列 chartData、设备 breakdown、地域 geoStats 与聚合维度）。
你的输出必须是**单个 JSON 对象**（禁止 Markdown 代码围栏、禁止 JSON 外交互说明），键名固定如下：
{
  "title": "简洁中文标题",
  "summary": "2～4句执行摘要，必须引用 metrics 中的数字或趋势",
  "trends": ["基于 chartData 或总量变化的要点", "..."],
  "anomalies": ["基于数据异常或波动；若无则写 无明显异常"],
  "suggestions": ["可落地的运营/技术建议", "..."],
  "markdown": "完整 Markdown 报告，至少包含 # 标题、## 数据概览、## 趋势与解读、## 风险与异常、## 建议动作；表格或列表中出现的数字须来自输入 JSON"
}
规则：
1) 所有键必须出现；数组每项为非空字符串；
2) 不得编造 metrics 中不存在的日期、国家、次数；
3) 若样本量极少（如总 PV≤5），在 summary 与 anomalies 中明确提示「样本不足，结论仅供参考」；
4) markdown 与 summary/trends 等字段语义一致。

输入数据：
%s`, structuredJSON)
}

func validateReportJSONKeys(jsonStr string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return false
	}
	for _, k := range []string{"title", "summary", "trends", "anomalies", "suggestions", "markdown"} {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

func (d *DeepSeekProvider) callChatCompletions(ctx context.Context, body map[string]any) (string, error) {
	var out string
	err := breaker.GetBreaker("deepseek-http").DoCtx(ctx, func() error {
		var e error
		out, e = d.doChatCompletionsOnce(ctx, body)
		return e
	})
	if err != nil {
		return "", err
	}
	return out, nil
}

func (d *DeepSeekProvider) doChatCompletionsOnce(ctx context.Context, body map[string]any) (string, error) {
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bs, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("deepseek status=%d body=%s", resp.StatusCode, string(bs))
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(bs, &raw); err != nil {
		return "", err
	}
	if len(raw.Choices) == 0 {
		return "", fmt.Errorf("empty ai response")
	}
	return raw.Choices[0].Message.Content, nil
}

func (d *DeepSeekProvider) ruleBasedAnalyze(url, title, desc string) *AnalyzeResult {
	category := "其他"
	lower := strings.ToLower(url + " " + title + " " + desc)
	if strings.Contains(lower, "tech") || strings.Contains(lower, "go") || strings.Contains(lower, "github") {
		category = "技术"
	}
	level := "safe"
	reason := "未检测到高风险关键词"
	if strings.Contains(lower, "free-money") || strings.Contains(lower, "lottery") || strings.Contains(lower, "porn") {
		level = "dangerous"
		reason = "疑似诈骗或违规内容关键词"
	}
	return &AnalyzeResult{
		Suggestions:  []string{"quick-link", "smart-short", "easy-jump"},
		Category:     category,
		SafetyLevel:  level,
		SafetyReason: reason,
		PageTitle:    title,
		PageDesc:     desc,
	}
}

func extractJSON(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end <= start {
		return raw
	}
	return raw[start : end+1]
}
