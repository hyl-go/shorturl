package handler

import (
	"github.com/go-playground/validator/v10"
	"github.com/zeromicro/go-zero/core/logx"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"shorturl/internal/logic"
	"shorturl/internal/svc"
	"shorturl/internal/types"
	"shorturl/pkg/ip"
)

func ShowHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ShowRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if err := validator.New().StructCtx(r.Context(), &req); err != nil {
			logx.Errorw("validator check failed", logx.LogField{Key: "err", Value: err.Error()})
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.IP = ip.FromRequest(r)
		req.Country = firstNonEmpty(
			r.Header.Get("CF-IPCountry"),
			r.Header.Get("X-Country"),
			r.Header.Get("X-Geo-Country"),
		)
		req.Region = firstNonEmpty(
			r.Header.Get("X-Region"),
			r.Header.Get("X-Geo-Region"),
			r.Header.Get("X-City"),
			r.Header.Get("X-Geo-City"),
		)
		req.UserAgent = r.UserAgent()
		req.Referer = r.Referer()
		l := logic.NewShowLogic(r.Context(), svcCtx)
		resp, err := l.Show(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		http.Redirect(w, r, resp.LongURL, http.StatusFound)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
