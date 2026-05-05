package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"shorturl/pkg/base62"

	"github.com/hibiken/asynq"
	"shorturl/internal/config"
	"shorturl/internal/handler"
	"shorturl/internal/logic"
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

	if mw := handler.RateLimitMiddleware(c); mw != nil {
		server.Use(mw)
	}
	if mw := handler.AdminAuthMiddleware(c); mw != nil {
		server.Use(mw)
	}

	ctx := svc.NewServiceContext(c)
	redisOpt := asynq.RedisClientOpt{
		Addr:     c.Asynq.RedisAddr,
		Password: c.Asynq.RedisPass,
		DB:       c.Asynq.RedisDB,
	}
	statsWorker := worker.NewStatsWorker(ctx.DbConn)
	var gcWorker *worker.LinkGCWorker
	if c.GC.Enabled {
		gcWorker = worker.NewLinkGCWorker(ctx.DbConn, ctx.Redis, ctx.Filter)
	}
	workerServer := worker.NewAsynqServer(redisOpt)
	reportRunner := logic.NewAIReportTaskRunner(ctx)
	worker.RunServer(workerServer, worker.BuildMux(ctx.LogWorker, statsWorker, gcWorker, reportRunner.HandleTask))

	scheduler := asynq.NewScheduler(redisOpt, nil)
	if _, err := scheduler.Register(
		"0 * * * *",
		asynq.NewTask(worker.TypeStatsAggregateHour, nil),
		asynq.Queue(worker.QueueStats),
		asynq.MaxRetry(5),
	); err != nil {
		log.Fatalf("scheduler register failed: %v", err)
	}
	if c.GC.Enabled && gcWorker != nil {
		retention := c.GC.RetentionDays
		if retention <= 0 {
			retention = 90
		}
		if _, err := scheduler.Register("0 4 * * *", asynq.NewTask(worker.TypeLinkGCPurge, []byte(strconv.Itoa(retention)))); err != nil {
			log.Fatalf("scheduler register link gc failed: %v", err)
		}
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
