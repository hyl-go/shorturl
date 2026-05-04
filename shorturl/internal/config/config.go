package config

import (
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf

	ShortUrlDB struct {
		DSN string
	}

	Sequence struct {
		DSN string
	}

	BaseString string

	ShortUrlBlackList []string

	ShortDomain string

	CacheRedis cache.CacheConf

	AI AIConfig

	Asynq AsynqConfig

	// Admin.ApiToken 非空时，/stats、/analyze、/links* 需 Header X-Admin-Token；演示环境可留空
	Admin struct {
		ApiToken string `json:",optional"`
	}

	// GC：物理清理长期软删墓碑，释放 lurl/surl 唯一槽位（布隆键无法移除，仅接受假阳性）
	GC struct {
		Enabled       bool `json:",optional"`
		RetentionDays int  `json:",optional"`
	}

	// RateLimit：按客户端 IP 的令牌桶限流（轻量路径与重路径分开计数）。Enabled=false 时不挂载。
	RateLimit struct {
		Enabled bool `json:",optional"`
		// 多数接口：每秒令牌数 / 突发
		PerIPQPS        int `json:",optional"`
		PerIPBurst      int `json:",optional"`
		HeavyPerIPQPS   int `json:",optional"` // POST /convert、/analyze
		HeavyPerIPBurst int `json:",optional"`
		// MaxTrackedIPs 进程内按 IP 限流条目上限，超出时淘汰一批键，防止 map 无限增长
		MaxTrackedIPs int `json:",optional"`
	}

	// SSRF：对用户 URL 的出站探测与抓取（connect、Fetcher）统一策略。
	SSRF struct {
		ProbeTimeout        time.Duration `json:",optional"`
		FetchTimeout        time.Duration `json:",optional"`
		MaxRedirects        int           `json:",optional"`
		OnlyStdPorts        bool          `json:",optional"` // true 时仅允许 80/443
		AllowPrivateTargets bool          `json:",optional"` // true 允许 localhost/私网（仅开发）；生产务必 false
		MaxFetchBodyBytes   int64         `json:",optional"`
	}
}

type AIConfig struct {
	Provider string

	DeepSeek AIProviderConfig

	Fallback struct {
		Enabled   bool
		Providers []string
	}
}

type AIProviderConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

type AsynqConfig struct {
	RedisAddr string
	RedisPass string
	RedisDB   int
	Queue     string
}
