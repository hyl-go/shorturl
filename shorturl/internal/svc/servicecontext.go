package svc

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"shorturl/internal/aireport"
	"shorturl/internal/ai"
	"shorturl/internal/config"
	"shorturl/internal/crawler"
	"shorturl/internal/worker"
	"shorturl/model"
	"shorturl/pkg/geoip"
	"shorturl/pkg/ssrf"
	"shorturl/pkg/surlfilter"
	"shorturl/sequence"
)

type ServiceContext struct {
	Config            config.Config
	ShortUrlMapModel  model.ShortUrlMapModel
	Sequence          sequence.Sequence
	ShortUrlBlackList map[string]struct{}
	Filter            surlfilter.Filter
	Redis             *redis.Redis
	GoRedis           *goredis.Client
	DbConn            sqlx.SqlConn
	AIFactory         *ai.Factory
	Fetcher           *crawler.Fetcher
	// UserURLProbe 对用户长链做可达性探测（SSRF 防护 Client）
	UserURLProbe *http.Client
	LogWorker       *worker.LogWorker
	AsynqClient     *asynq.Client
	AIReportStore   *aireport.Store
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.ShortUrlDB.DSN)
	m := make(map[string]struct{}, len(c.ShortUrlBlackList))
	for _, v := range c.ShortUrlBlackList {
		m[v] = struct{}{}
	}
	store := redis.New(c.CacheRedis[0].Host, func(r *redis.Redis) {
		r.Type = redis.NodeType
		r.Pass = c.CacheRedis[0].Pass
	})

	n0 := c.CacheRedis[0]
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     n0.Host,
		Password: n0.Pass,
		DB:       0,
	})

	shortUrlMapModel := model.NewShortUrlMapModel(conn, c.CacheRedis)
	filter := surlfilter.New(surlfilter.Options{
		Backend:         c.ShortURLFilter.Backend,
		CuckooKey:       c.ShortURLFilter.CuckooKey,
		CuckooCapacity:  c.ShortURLFilter.CuckooCapacity,
		LegacyBloomKey:  c.ShortURLFilter.LegacyBloomKey,
		LegacyBloomBits: c.ShortURLFilter.LegacyBloomBits,
		LegacyStore:     store,
		GoRedis:         rdb,
	})

	seq := buildSequence(c, rdb, conn)

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
	var geoResolver *geoip.Resolver
	if c.GeoIP.Enabled {
		geoResolver = geoip.NewResolver(c.GeoIP.Endpoint, c.GeoIP.Timeout, c.GeoIP.CacheTTL)
	}

	logWorker := &worker.LogWorker{
		Client:      asynqClient,
		DB:          conn,
		GeoResolver: geoResolver,
	}
	reportStore := &aireport.Store{Rdb: rdb, TTL: 24 * time.Hour}

	if surls, err := shortUrlMapModel.FindAllSurl(context.Background()); err != nil {
		logx.Errorf("short url filter warmup list failed: %v", err)
	} else if filter != nil {
		if err := filter.Warmup(context.Background(), surls); err != nil {
			logx.Errorf("short url filter warmup failed: %v", err)
		} else {
			logx.Infof("short url filter warmup done, loaded %d surls", len(surls))
		}
	}

	return &ServiceContext{
		Config:            c,
		ShortUrlMapModel:  shortUrlMapModel,
		Sequence:          seq,
		Filter:            filter,
		Redis:             store,
		GoRedis:           rdb,
		ShortUrlBlackList: m,
		DbConn:            conn,
		AIFactory:         aiFactory,
		Fetcher:           fetcher,
		UserURLProbe:      userURLProbe,
		LogWorker:       logWorker,
		AsynqClient:     asynqClient,
		AIReportStore:   reportStore,
	}
}

func buildSequence(c config.Config, rdb *goredis.Client, conn sqlx.SqlConn) sequence.Sequence {
	prov := strings.ToLower(strings.TrimSpace(c.Sequence.Provider))
	if prov == "mysql" {
		return sequence.NewMysql(c.Sequence.DSN)
	}
	key := strings.TrimSpace(c.Sequence.RedisKey)
	if key == "" {
		key = "shorturl:sequence"
	}
	s, err := sequence.NewRedis(rdb, key, conn, c.Sequence.BootstrapFromMysql)
	if err != nil {
		logx.Must(err)
	}
	return s
}
