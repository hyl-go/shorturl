package connect

import (
	"io"
	"net/http"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// client 用于转链前的可达性探测（轻量 GET，丢弃 body）。
var client = &http.Client{
	Transport: &http.Transport{
		DisableKeepAlives: true,
	},
	Timeout: 5 * time.Second,
}

const probeUserAgent = "Mozilla/5.0 (compatible; ShortURL-Bot/1.0) Go-http-client/1.1"

// Get 判断 URL 是否「大概率可访问」。
//
// 规则（相对宽松、业界常见做法）：
//   - HTTP 状态码为 2xx（200–299）视为成功；
//   - 默认 Client 会跟随重定向，最终状态参与判断；
//   - 网络错误、超时、非 2xx（如 403/404/5xx）视为失败。
//
// 说明：部分站点对默认 Go User-Agent 返回 403，故设置常见 UA 以降低误判。
func Get(rawURL string) bool {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", probeUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		logx.Errorw("connect probe failed", logx.LogField{Key: "error", Value: err.Error()})
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	code := resp.StatusCode
	return code >= http.StatusOK && code < http.StatusMultipleChoices
}
