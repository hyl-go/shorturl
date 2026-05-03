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
