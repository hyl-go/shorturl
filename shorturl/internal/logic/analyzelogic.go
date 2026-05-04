package logic

import (
	"context"
	"fmt"

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

	dataSummary := fmt.Sprintf("短链=%s, 日期=%s~%s, PV=%d, UV=%d, 移动端占比=%.2f%%",
		req.ShortURL, req.StartDate, req.EndDate, stats.TotalPV, stats.TotalUV, stats.DeviceStats.MobileRate)
	if len(stats.ChartData) > 0 {
		dataSummary += fmt.Sprintf(", 按日样本天数=%d", len(stats.ChartData))
	}
	if len(stats.GeoStats) > 0 {
		g := stats.GeoStats[0]
		dataSummary += fmt.Sprintf(", 地域Top1=%s/%s 次数=%d", g.Country, g.City, g.Count)
	}

	report := l.svcCtx.AIFactory.GenerateReportWithFallback(l.ctx, l.svcCtx.Config.AI.Provider, dataSummary)

	return &types.AnalyzeResponse{
		Statistics: *stats,
		AIReport: types.AIReport{
			Summary:     report.Summary,
			Trends:      report.Trends,
			Anomalies:   report.Anomalies,
			Suggestions: report.Suggestions,
		},
	}, nil
}
