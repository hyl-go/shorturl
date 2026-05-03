package main

import (
	"flag"
	"fmt"
	"log"
	"shorturl/pkg/base62"

	"github.com/hibiken/asynq"
	"shorturl/internal/config"
	"shorturl/internal/handler"
	"shorturl/internal/svc"
	"shorturl/internal/worker"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/shorturl-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	logx.Infof("load conf: Name=%s Host=%s Port=%d", c.Name, c.Host, c.Port)
	base62.MustInt(c.BaseString)
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	redisOpt := asynq.RedisClientOpt{
		Addr:     c.Asynq.RedisAddr,
		Password: c.Asynq.RedisPass,
		DB:       c.Asynq.RedisDB,
	}
	statsWorker := worker.NewStatsWorker(ctx.DbConn)
	workerServer := worker.NewAsynqServer(redisOpt)
	worker.RunServer(workerServer, worker.BuildMux(ctx.LogWorker, statsWorker))

	scheduler := asynq.NewScheduler(redisOpt, nil)
	if _, err := scheduler.Register("0 * * * *", asynq.NewTask(worker.TypeStatsAggregateHour, nil)); err != nil {
		log.Fatalf("scheduler register failed: %v", err)
	}
	go func() {
		if err := scheduler.Run(); err != nil {
			logx.Errorf("asynq scheduler stopped: %v", err)
		}
	}()

	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
