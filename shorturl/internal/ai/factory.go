package ai

import (
	"context"
	"fmt"
	"strings"

	"shorturl/internal/config"

	"github.com/zeromicro/go-zero/core/logx"
)

type Factory struct {
	providers map[string]AIProvider
	fallback  []string
}

func NewFactory(cfg config.AIConfig) *Factory {
	f := &Factory{
		providers: make(map[string]AIProvider),
	}
	if cfg.Fallback.Enabled {
		f.fallback = cfg.Fallback.Providers
	}
	f.providers["deepseek"] = NewDeepSeekProvider(cfg.DeepSeek)
	return f
}

func (f *Factory) GetProvider(name string) (AIProvider, error) {
	p, ok := f.providers[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("unknown ai provider: %s", name)
	}
	return p, nil
}

func (f *Factory) AnalyzeWithFallback(ctx context.Context, primary, url, title, desc string) (*AnalyzeResult, error) {
	names := []string{strings.ToLower(primary)}
	names = append(names, f.fallback...)

	var lastErr error
	for _, name := range names {
		p, err := f.GetProvider(name)
		if err != nil {
			lastErr = err
			continue
		}
		result, err := p.AnalyzeURL(ctx, url, title, desc)
		if err == nil {
			return result, nil
		}
		lastErr = err
		logx.Errorf("[ai] provider=%s analyze failed: %v", name, err)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no provider configured")
	}
	return nil, lastErr
}

// GenerateReportWithFallback 与 AnalyzeWithFallback 一致：按主 Provider + Fallback 列表依次尝试；
// 任一返回非空 summary 即采纳；全部失败则返回 FallbackStatsReport（不返回 error）。
func (f *Factory) GenerateReportWithFallback(ctx context.Context, primary string, statsData string) *ReportResult {
	order := []string{strings.ToLower(strings.TrimSpace(primary))}
	order = append(order, f.fallback...)
	seen := make(map[string]struct{})
	var names []string
	for _, n := range order {
		n = strings.ToLower(strings.TrimSpace(n))
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		names = append(names, n)
	}
	for _, name := range names {
		p, err := f.GetProvider(name)
		if err != nil {
			logx.Errorf("[ai] report skip provider=%s: %v", name, err)
			continue
		}
		report, err := p.GenerateReport(ctx, statsData)
		if err != nil {
			logx.Errorf("[ai] provider=%s GenerateReport failed: %v", name, err)
			continue
		}
		if report != nil && strings.TrimSpace(report.Summary) != "" {
			NormalizeReportResult(report)
			return report
		}
	}
	return FallbackStatsReport(statsData)
}
