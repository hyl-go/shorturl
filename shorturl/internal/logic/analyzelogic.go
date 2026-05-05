package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"shorturl/internal/aireport"
	"shorturl/internal/svc"
	"shorturl/internal/types"
)

type AnalyzeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyzeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyzeLogic {
	return &AnalyzeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyzeLogic) Analyze(req *types.AnalyzeRequest) (*types.AnalyzeResponse, error) {
	statsLogic := NewStatsLogic(l.ctx, l.svcCtx)
	stats, err := statsLogic.Stats(&types.StatsRequest{
		ShortURL:  req.ShortURL,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
	})
	if err != nil {
		return nil, err
	}

	jobID := uuid.New().String()
	statsBytes, err := json.Marshal(stats)
	if err != nil {
		return nil, err
	}
	payload := aireport.TaskPayload{
		JobID:     jobID,
		ShortURL:  req.ShortURL,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		StatsJSON: string(statsBytes),
	}
	taskBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if l.svcCtx.AIReportStore == nil || l.svcCtx.AsynqClient == nil {
		return nil, fmt.Errorf("异步报告队列未初始化")
	}
	if err := l.svcCtx.AIReportStore.CreatePending(l.ctx, jobID, req.ShortURL, req.StartDate, req.EndDate); err != nil {
		return nil, err
	}
	task := asynq.NewTask(aireport.TaskType, taskBody)
	_, err = l.svcCtx.AsynqClient.Enqueue(task,
		asynq.Queue(aireport.QueueName),
		asynq.MaxRetry(4),
		asynq.Timeout(12*time.Minute),
	)
	if err != nil {
		_ = l.svcCtx.AIReportStore.Delete(l.ctx, jobID)
		return nil, fmt.Errorf("报告任务入队失败: %w", err)
	}

	return &types.AnalyzeResponse{
		Statistics: *stats,
		AIReport:   nil,
		ReportJob: types.ReportJobInfo{
			JobId:  jobID,
			Status: string(aireport.StatusPending),
		},
	}, nil
}
