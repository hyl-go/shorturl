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
