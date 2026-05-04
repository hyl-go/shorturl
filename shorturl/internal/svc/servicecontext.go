package svc

import (
	"context"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/bloom"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"shorturl/internal/ai"
	"shorturl/internal/config"
	"shorturl/internal/crawler"
	"shorturl/internal/worker"
	"shorturl/model"
	"shorturl/pkg/ssrf"
	"shorturl/sequence"
)

type ServiceContext struct {
	Config            config.Config
	ShortUrlMapModel  model.ShortUrlMapModel
	Sequence          sequence.Sequence
	ShortUrlBlackList map[string]struct{}
	Filter            *bloom.Filter
	Redis             *redis.Redis
	DbConn            sqlx.SqlConn
	AIFactory         *ai.Factory
	Fetcher           *crawler.Fetcher
	// UserURLProbe 对用户长链做可达性探测（SSRF 防护 Client）
	UserURLProbe *http.Client
	LogWorker    *worker.LogWorker
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.ShortUrlDB.DSN)
	// 把配置文件中的黑名单放到map中
	m := make(map[string]struct{}, len(c.ShortUrlBlackList))
	for _, v := range c.ShortUrlBlackList {
		m[v] = struct{}{}
	}
	store := redis.New(c.CacheRedis[0].Host, func(r *redis.Redis) {
		r.Type = redis.NodeType
		r.Pass = c.CacheRedis[0].Pass
	})
	filter := bloom.New(store, "bloom_filter", 20*(1<<20))
	shortUrlMapModel := model.NewShortUrlMapModel(conn, c.CacheRedis)
	aiFactory := ai.NewFactory(c.AI)
	ssrfPol := ssrf.Policy{
		OnlyStdPorts:        c.SSRF.OnlyStdPorts,
		AllowPrivateTargets: c.SSRF.AllowPrivateTargets,
	}
	probeTO := c.SSRF.ProbeTimeout
	if probeTO <= 0 {
		probeTO = 5 * time.Second
	}
	fetchTO := c.SSRF.FetchTimeout
	if fetchTO <= 0 {
		fetchTO = 15 * time.Second
	}
	maxRedir := c.SSRF.MaxRedirects
	if maxRedir <= 0 {
		maxRedir = 5
	}
	maxBody := c.SSRF.MaxFetchBodyBytes
	if maxBody <= 0 {
		maxBody = 2 * 1024 * 1024
	}
	userURLProbe := ssrf.NewUserURLHTTPClient(ssrf.ClientOptions{
		Policy:       ssrfPol,
		Timeout:      probeTO,
		MaxRedirects: maxRedir,
	})
	fetcher := crawler.NewFetcher(crawler.Options{
		HTTPClient: ssrf.NewUserURLHTTPClient(ssrf.ClientOptions{
			Policy:       ssrfPol,
			Timeout:      fetchTO,
			MaxRedirects: maxRedir,
		}),
		MaxBodyBytes: maxBody,
	})

	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     c.Asynq.RedisAddr,
		Password: c.Asynq.RedisPass,
		DB:       c.Asynq.RedisDB,
	})
	logWorker := &worker.LogWorker{
		Client: asynqClient,
		DB:     conn,
	}

	// 启动时预热布隆过滤器，将数据库中所有已有短链接加载进去
	if surls, err := shortUrlMapModel.FindAllSurl(context.Background()); err != nil {
		logx.Errorf("bloom filter warmup failed: %v", err)
	} else {
		for _, surl := range surls {
			if err := filter.Add([]byte(surl)); err != nil {
				logx.Errorf("bloom filter add [%s] failed: %v", surl, err)
			}
		}
		logx.Infof("bloom filter warmup done, loaded %d surls", len(surls))
	}

	return &ServiceContext{
		Config:            c,
		ShortUrlMapModel:  shortUrlMapModel,
		Sequence:          sequence.NewMysql(c.Sequence.DSN),
		Filter:            filter,
		Redis:             store,
		ShortUrlBlackList: m,
		DbConn:            conn,
		AIFactory:         aiFactory,
		Fetcher:           fetcher,
		UserURLProbe:      userURLProbe,
		LogWorker:         logWorker,
	}
}
