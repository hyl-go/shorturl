package worker

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/logx"

	"shorturl/internal/aireport"
)

func NewAsynqServer(redisOpt asynq.RedisClientOpt) *asynq.Server {
	return asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 12,
		Queues: map[string]int{
			aireport.QueueName: 6,
			QueueStats:         3,
			"default":          1,
		},
	})
}

// BuildMux aiReport 可为 nil（测试）；生产由 main 注入 AI 报告任务处理函数。
func BuildMux(logWorker *LogWorker, statsWorker *StatsWorker, gcWorker *LinkGCWorker, aiReport func(context.Context, *asynq.Task) error) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeAccessLog, func(ctx context.Context, task *asynq.Task) error {
		return logWorker.HandleAccessLog(ctx, task)
	})
	mux.HandleFunc(TypeStatsAggregateHour, func(ctx context.Context, task *asynq.Task) error {
		return statsWorker.AggregateHour(ctx, task)
	})
	if gcWorker != nil {
		mux.HandleFunc(TypeLinkGCPurge, func(ctx context.Context, task *asynq.Task) error {
			return gcWorker.PurgeOldTombstones(ctx, task)
		})
	}
	if aiReport != nil {
		mux.HandleFunc(aireport.TaskType, aiReport)
	}
	return mux
}

func RunServer(srv *asynq.Server, mux *asynq.ServeMux) {
	go func() {
		if err := srv.Run(mux); err != nil {
			logx.Errorf("asynq server stopped: %v", err)
		}
	}()
}
