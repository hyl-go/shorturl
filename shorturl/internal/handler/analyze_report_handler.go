package handler

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"shorturl/internal/logic"
	"shorturl/internal/svc"
	"shorturl/internal/types"
)

func AnalyzeReportStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AnalyzeReportStatusRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := validator.New().StructCtx(r.Context(), &req); err != nil {
			logx.Errorw("validator check failed", logx.LogField{Key: "err", Value: err.Error()})
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewAnalyzeReportStatusLogic(r.Context(), svcCtx)
		resp, err := l.Status(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyzeReportUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AnalyzeReportUpdateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := validator.New().StructCtx(r.Context(), &req); err != nil {
			logx.Errorw("validator check failed", logx.LogField{Key: "err", Value: err.Error()})
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewAnalyzeReportUpdateLogic(r.Context(), svcCtx)
		resp, err := l.Update(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
