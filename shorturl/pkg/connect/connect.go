package connect

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/zeromicro/go-zero/core/breaker"
	"github.com/zeromicro/go-zero/core/logx"
)

const probeUserAgent = "Mozilla/5.0 (compatible; ShortURL-Bot/1.0) Go-http-client/1.1"

// Get 使用由 ssrf.NewUserURLHTTPClient 构建的 client 对 rawURL 做轻量 GET 探测（丢弃 body）。
// httpClient 为 nil 时返回 false（避免无防护的默认 Client）。
//
// 架构说明：协议/私网/重定向等约束由注入的 Client Transport 与 CheckRedirect 统一执行，见 pkg/ssrf。
func Get(ctx context.Context, httpClient *http.Client, rawURL string) bool {
	if httpClient == nil {
		return false
	}
	err := breaker.GetBreaker("connect-probe").DoCtx(ctx, func() error {
		return probeOnce(ctx, httpClient, rawURL)
	})
	if errors.Is(err, breaker.ErrServiceUnavailable) {
		logx.Infow("connect probe breaker open, degraded allow URL check")
		return true
	}
	return err == nil
}

func probeOnce(ctx context.Context, client *http.Client, rawURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", probeUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		logx.Errorw("connect probe failed", logx.LogField{Key: "error", Value: err.Error()})
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	code := resp.StatusCode
	if code >= http.StatusOK && code < http.StatusMultipleChoices {
		return nil
	}
	return errProbeBadStatus
}

var errProbeBadStatus = probeStatusError{}

type probeStatusError struct{}

func (probeStatusError) Error() string { return "probe non-2xx" }
