package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"shorturl/internal/logic"
	"shorturl/internal/svc"
)

// PerformanceHandler GET /admin/performance — 主机与 MySQL/Redis 性能快照（管理端）。
func PerformanceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewPerformanceLogic(r.Context(), svcCtx)
		resp, err := l.PerformanceSnapshot()
		if err != nil {
			logx.Errorw("performance snapshot failed", logx.LogField{Key: "err", Value: err.Error()})
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
