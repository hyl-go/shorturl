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

type ProbeStatus string

const (
	ProbeReachable ProbeStatus = "reachable"
	ProbeRedirect  ProbeStatus = "redirect"
	ProbeRejected  ProbeStatus = "rejected"
	ProbeError     ProbeStatus = "error"
)

type ProbeResult struct {
	Status     ProbeStatus
	StatusCode int
	Err        error
}

func (r ProbeResult) IsValidURL() bool {
	return r.Status == ProbeReachable || r.Status == ProbeRedirect
}

// Get 使用由 ssrf.NewUserURLHTTPClient 构建的 client 对 rawURL 做轻量探测。
// 默认先 HEAD，必要时回退 GET（例如目标站不支持 HEAD）。
// httpClient 为 nil 时返回 false（避免无防护的默认 Client）。
//
// 架构说明：协议/私网/重定向等约束由注入的 Client Transport 与 CheckRedirect 统一执行，见 pkg/ssrf。
func Get(ctx context.Context, httpClient *http.Client, rawURL string) bool {
	return Probe(ctx, httpClient, rawURL).IsValidURL()
}

// Probe 返回可达性探测结果：2xx=reachable，3xx=redirect（也视为有效链接），其余为 rejected/error。
func Probe(ctx context.Context, httpClient *http.Client, rawURL string) ProbeResult {
	if httpClient == nil {
		return ProbeResult{Status: ProbeError, Err: errors.New("nil probe client")}
	}
	var out ProbeResult
	err := breaker.GetBreaker("connect-probe").DoCtx(ctx, func() error {
		res, err := probeOnce(ctx, httpClient, rawURL)
		out = res
		return err
	})
	if errors.Is(err, breaker.ErrServiceUnavailable) {
		logx.Infow("connect probe breaker open, degraded allow URL check")
		return ProbeResult{Status: ProbeReachable}
	}
	if err != nil && out.Status == "" {
		out = ProbeResult{Status: ProbeError, Err: err}
	}
	return out
}

func probeOnce(ctx context.Context, client *http.Client, rawURL string) (ProbeResult, error) {
	// HEAD 优先，省流量；若目标不支持 HEAD 或异常，再回退 GET。
	headRes, headErr := probeWithMethod(ctx, client, rawURL, http.MethodHead)
	if headErr == nil {
		return headRes, nil
	}
	if shouldFallbackGet(headRes) || headRes.Status == ProbeError {
		return probeWithMethod(ctx, client, rawURL, http.MethodGet)
	}
	return headRes, headErr
}

func probeWithMethod(ctx context.Context, client *http.Client, rawURL, method string) (ProbeResult, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return ProbeResult{Status: ProbeError, Err: err}, err
	}
	req.Header.Set("User-Agent", probeUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		logx.Errorw("connect probe failed", logx.LogField{Key: "error", Value: err.Error()})
		return ProbeResult{Status: ProbeError, Err: err}, err
	}
	defer resp.Body.Close()
	if method != http.MethodHead {
		_, _ = io.Copy(io.Discard, resp.Body)
	}

	code := resp.StatusCode
	if code >= http.StatusOK && code < http.StatusMultipleChoices {
		return ProbeResult{Status: ProbeReachable, StatusCode: code}, nil
	}
	if code >= http.StatusMultipleChoices && code < http.StatusBadRequest {
		return ProbeResult{Status: ProbeRedirect, StatusCode: code}, nil
	}
	return ProbeResult{Status: ProbeRejected, StatusCode: code, Err: errProbeBadStatus}, errProbeBadStatus
}

func shouldFallbackGet(res ProbeResult) bool {
	switch res.StatusCode {
	case http.StatusMethodNotAllowed, http.StatusNotImplemented:
		return true
	default:
		return false
	}
}

var errProbeBadStatus = probeStatusError{}

type probeStatusError struct{}

func (probeStatusError) Error() string { return "probe non-2xx/3xx" }
