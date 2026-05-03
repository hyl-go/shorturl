package worker

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/logx"
)

func NewAsynqServer(redisOpt asynq.RedisClientOpt) *asynq.Server {
	return asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			"default": 1,
		},
	})
}

func BuildMux(logWorker *LogWorker, statsWorker *StatsWorker) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeAccessLog, func(ctx context.Context, task *asynq.Task) error {
		return logWorker.HandleAccessLog(ctx, task)
	})
	mux.HandleFunc(TypeStatsAggregateHour, func(ctx context.Context, task *asynq.Task) error {
		return statsWorker.AggregateHour(ctx, task)
	})
	return mux
}

func RunServer(srv *asynq.Server, mux *asynq.ServeMux) {
	go func() {
		if err := srv.Run(mux); err != nil {
			logx.Errorf("asynq server stopped: %v", err)
		}
	}()
}
