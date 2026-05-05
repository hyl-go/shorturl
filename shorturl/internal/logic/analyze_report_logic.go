package logic

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"

	"shorturl/internal/svc"
	"shorturl/internal/types"
)

type AnalyzeReportStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyzeReportStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyzeReportStatusLogic {
	return &AnalyzeReportStatusLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *AnalyzeReportStatusLogic) Status(req *types.AnalyzeReportStatusRequest) (*types.AnalyzeReportStatusResponse, error) {
	st := l.svcCtx.AIReportStore
	if st == nil {
		return nil, errors.New("报告任务存储未初始化")
	}
	rec, err := st.Get(l.ctx, req.JobId)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errors.New("任务不存在或已过期")
		}
		return nil, err
	}
	out := &types.AnalyzeReportStatusResponse{
		Status:         string(rec.Status),
		MarkdownEdited: rec.MarkdownEdited,
	}
	if rec.Error != "" {
		out.Error = rec.Error
	}
	if rec.AIReportJSON != "" {
		var ar types.AIReport
		if err := json.Unmarshal([]byte(rec.AIReportJSON), &ar); err == nil {
			out.AIReport = &ar
		}
	}
	return out, nil
}

type AnalyzeReportUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyzeReportUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyzeReportUpdateLogic {
	return &AnalyzeReportUpdateLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *AnalyzeReportUpdateLogic) Update(req *types.AnalyzeReportUpdateRequest) (*types.AnalyzeReportUpdateResponse, error) {
	st := l.svcCtx.AIReportStore
	if st == nil {
		return nil, errors.New("报告任务存储未初始化")
	}
	if _, err := st.Get(l.ctx, req.JobId); err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errors.New("任务不存在或已过期")
		}
		return nil, err
	}
	if err := st.SetMarkdownEdited(l.ctx, req.JobId, req.Markdown); err != nil {
		return nil, err
	}
	return &types.AnalyzeReportUpdateResponse{Ok: true}, nil
}
