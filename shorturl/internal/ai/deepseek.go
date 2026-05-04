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
	if statsData == "" {
		statsData = "（无摘要输入）"
	}
	if d.apiKey == "" || d.baseURL == "" || d.model == "" {
		return FallbackStatsReport(statsData), nil
	}
	prompt := fmt.Sprintf(`你是数据分析师。根据下方「短链访问统计摘要」，用中文输出**仅一段 JSON**（不要 Markdown、不要代码围栏），格式：
{"summary":"1～3句总体结论","trends":["趋势要点1","趋势要点2"],"anomalies":["异常或风险点，若无则填「无明显异常」"],"suggestions":["可执行建议1","建议2"]}
统计摘要：
%s`, statsData)

	reqBody := map[string]any{
		"model": d.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.35,
	}
	raw, err := d.callChatCompletions(ctx, reqBody)
	if err != nil {
		return FallbackStatsReport(statsData), nil
	}
	var r ReportResult
	if err := json.Unmarshal([]byte(extractJSON(raw)), &r); err != nil {
		return FallbackStatsReport(statsData), nil
	}
	NormalizeReportResult(&r)
	if strings.TrimSpace(r.Summary) == "" {
		return FallbackStatsReport(statsData), nil
	}
	return &r, nil
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
