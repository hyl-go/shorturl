package surlfilter

import (
	"context"

	"github.com/zeromicro/go-zero/core/bloom"
)

type legacyBloom struct {
	f *bloom.Filter
}

func (l *legacyBloom) Add(ctx context.Context, item []byte) error {
	return l.f.AddCtx(ctx, item)
}

func (l *legacyBloom) Remove(ctx context.Context, item []byte) error {
	_ = item
	_ = ctx
	// 标准布隆不支持删除；降级路径上为 no-op
	return nil
}

func (l *legacyBloom) Exists(ctx context.Context, item []byte) (bool, error) {
	return l.f.ExistsCtx(ctx, item)
}

func (l *legacyBloom) Warmup(ctx context.Context, items []string) error {
	for _, s := range items {
		if s == "" {
			continue
		}
		if err := l.f.AddCtx(ctx, []byte(s)); err != nil {
			return err
		}
	}
	return nil
}
