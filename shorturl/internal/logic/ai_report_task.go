package logic

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/logx"

	"shorturl/internal/aireport"
	"shorturl/internal/ai"
	"shorturl/internal/svc"
	"shorturl/internal/types"
)

// AIReportTaskRunner 消费 Asynq 报告任务；与 HTTP 请求解耦，支持重试与超时。
type AIReportTaskRunner struct {
	svcCtx *svc.ServiceContext
}

func NewAIReportTaskRunner(svcCtx *svc.ServiceContext) *AIReportTaskRunner {
	return &AIReportTaskRunner{svcCtx: svcCtx}
}

// HandleTask 实现 asynq.HandlerFunc。
func (r *AIReportTaskRunner) HandleTask(ctx context.Context, t *asynq.Task) error {
	var p aireport.TaskPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	store := r.svcCtx.AIReportStore
	if store == nil {
		logx.Errorf("[aireport] store nil, skip job=%s", p.JobID)
		return nil
	}
	if err := store.SetRunning(ctx, p.JobID); err != nil {
		logx.Errorf("[aireport] set running job=%s: %v", p.JobID, err)
		return err
	}

	var stats types.StatsResponse
	if err := json.Unmarshal([]byte(p.StatsJSON), &stats); err != nil {
		logx.Errorf("[aireport] stats json job=%s: %v", p.JobID, err)
		fb := fallbackAIReportJSON(&p, "统计快照解析失败，已使用占位报告")
		_ = store.SetFailed(ctx, p.JobID, err.Error(), fb)
		return nil
	}

	structured := aireport.FormatStructuredPromptJSON(p.ShortURL, p.StartDate, p.EndDate, &stats)
	rep := r.svcCtx.AIFactory.GenerateReportWithFallback(ctx, r.svcCtx.Config.AI.Provider, structured)
	ar := reportResultToAIReport(rep)
	b, err := json.Marshal(ar)
	if err != nil {
		fb := fallbackAIReportJSON(&p, "报告序列化失败")
		_ = store.SetFailed(ctx, p.JobID, err.Error(), fb)
		return nil
	}
	if err := store.SetCompleted(ctx, p.JobID, string(b)); err != nil {
		logx.Errorf("[aireport] set completed job=%s: %v", p.JobID, err)
		return err
	}
	return nil
}

func reportResultToAIReport(rep *ai.ReportResult) types.AIReport {
	if rep == nil {
		return types.AIReport{}
	}
	return types.AIReport{
		Title:       rep.Title,
		Summary:     rep.Summary,
		Trends:      rep.Trends,
		Anomalies:   rep.Anomalies,
		Suggestions: rep.Suggestions,
		Markdown:    rep.Markdown,
	}
}

func fallbackAIReportJSON(p *aireport.TaskPayload, reason string) string {
	hint := p.ShortURL + " | " + p.StartDate + "~" + p.EndDate + " | " + reason
	fr := ai.FallbackStatsReport(hint)
	ar := reportResultToAIReport(fr)
	b, _ := json.Marshal(ar)
	return string(b)
}
