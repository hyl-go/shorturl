package svc

import (
	"context"

	"github.com/zeromicro/go-zero/core/bloom"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"shorturl/internal/config"
	"shorturl/model"
	"shorturl/sequence"
)

type ServiceContext struct {
	Config            config.Config
	ShortUrlMapModel  model.ShortUrlMapModel
	Sequence          sequence.Sequence
	ShortUrlBlackList map[string]struct{}
	Filter            *bloom.Filter
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
		ShortUrlBlackList: m,
	}
}
