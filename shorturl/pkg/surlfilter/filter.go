// Package surlfilter 提供短链路径段的近似存在性过滤。
// 推荐使用 Redis Stack 的 Cuckoo Filter（CF.*），支持删除；legacy 为 go-zero 标准布隆，无法删除元素。
package surlfilter

import (
	"context"
	"strings"

	goredis "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/bloom"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// Filter 近似集合：Exists 假阴性不可接受场景须以 DB 为准；假阳性仅多查库。
type Filter interface {
	Add(ctx context.Context, item []byte) error
	Remove(ctx context.Context, item []byte) error
	Exists(ctx context.Context, item []byte) (bool, error)
	Warmup(ctx context.Context, items []string) error
}

// Options 构造参数（由 config.Config 映射而来，避免 pkg 依赖 internal）。
type Options struct {
	Backend         string // cuckoo | legacy；空默认 cuckoo
	CuckooKey       string
	CuckooCapacity  int64
	LegacyBloomKey  string
	LegacyBloomBits uint
	LegacyStore     *redis.Redis
	GoRedis         *goredis.Client
}

// New 按选项构造过滤器。cuckoo 初始化失败时自动降级为 legacy。
func New(opt Options) Filter {
	backend := strings.ToLower(strings.TrimSpace(opt.Backend))
	if backend == "" {
		backend = "cuckoo"
	}
	if backend == "legacy" {
		return newLegacy(opt.LegacyStore, opt.LegacyBloomKey, opt.LegacyBloomBits)
	}
	if opt.GoRedis == nil {
		logx.Error("surlfilter: cuckoo backend requires GoRedis client, falling back to legacy bloom")
		return newLegacy(opt.LegacyStore, opt.LegacyBloomKey, opt.LegacyBloomBits)
	}
	key := strings.TrimSpace(opt.CuckooKey)
	if key == "" {
		key = "shorturl:surl_cf"
	}
	capacity := opt.CuckooCapacity
	if capacity <= 0 {
		capacity = 25_000_000
	}
	cf, err := NewCuckoo(opt.GoRedis, key, capacity)
	if err != nil {
		logx.Errorw("surlfilter: cuckoo init failed, falling back to legacy bloom (Remove becomes no-op)",
			logx.LogField{Key: "err", Value: err.Error()})
		return newLegacy(opt.LegacyStore, opt.LegacyBloomKey, opt.LegacyBloomBits)
	}
	return cf
}

func newLegacy(store *redis.Redis, key string, bits uint) Filter {
	if store == nil {
		return nil
	}
	if key == "" {
		key = "bloom_filter"
	}
	if bits == 0 {
		bits = 20 * (1 << 20)
	}
	return &legacyBloom{f: bloom.New(store, key, bits)}
}
