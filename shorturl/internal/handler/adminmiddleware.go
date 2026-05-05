package handler

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"shorturl/internal/config"
)

// AdminAuthMiddleware 当 Config.Admin.ApiToken 非空时，对运营类路径校验请求头 X-Admin-Token。
// 返回 nil 表示不挂载中间件（演示默认）。
func AdminAuthMiddleware(c config.Config) func(http.HandlerFunc) http.HandlerFunc {
	token := strings.TrimSpace(c.Admin.ApiToken)
	if token == "" {
		return nil
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !adminProtectedPath(r.URL.Path) {
				next(w, r)
				return
			}
			got := strings.TrimSpace(r.Header.Get("X-Admin-Token"))
			if len(got) != len(token) || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("需要管理员凭证（Header: X-Admin-Token）"))
				return
			}
			next(w, r)
		}
	}
}

func adminProtectedPath(path string) bool {
	if strings.HasPrefix(path, "/admin/") {
		return true
	}
	if path == "/stats" || path == "/analyze" || strings.HasPrefix(path, "/analyze/") {
		return true
	}
	return strings.HasPrefix(path, "/links")
}
