package ssrf

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

// ClientOptions 构建用于「抓取用户 URL」的 HTTP 客户端。
type ClientOptions struct {
	Policy       Policy
	Timeout      time.Duration
	MaxRedirects int
}

// NewUserURLHTTPClient 返回配置了 SSRF 防护的 Client：Dial 前解析并校验 IP、限制重定向并在每一步校验目标 URL。
func NewUserURLHTTPClient(opt ClientOptions) *http.Client {
	if opt.Timeout <= 0 {
		opt.Timeout = 15 * time.Second
	}
	if opt.MaxRedirects <= 0 {
		opt.MaxRedirects = 5
	}
	maxRedir := opt.MaxRedirects
	policy := opt.Policy

	resolver := &net.Resolver{PreferGo: true}
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           safeDialContext(resolver, policy),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	rt := &validatingRoundTripper{
		policy: policy,
		next: &headerRoundTripper{
			next: tr,
		},
	}

	return &http.Client{
		Timeout:   opt.Timeout,
		Transport: rt,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedir {
				return fmt.Errorf("ssrf: 重定向次数超过上限 %d", maxRedir)
			}
			if req.URL == nil {
				return fmt.Errorf("ssrf: 重定向目标无效")
			}
			if err := policy.ValidateRequestURL(req.URL); err != nil {
				return err
			}
			return nil
		},
	}
}

// safeDialContext 在建立 TCP 前解析主机名并校验全部解析结果，降低 DNS 重绑定风险。
func safeDialContext(resolver *net.Resolver, policy Policy) func(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if ph := net.ParseIP(host); ph != nil {
			if policy.ipBlocked(ph) {
				return nil, fmt.Errorf("ssrf: 禁止访问该 IP 地址")
			}
			return d.DialContext(ctx, network, addr)
		}
		ips, err := resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("ssrf: 主机无可用解析记录")
		}
		if policy.addrsBlocked(ips) {
			return nil, fmt.Errorf("ssrf: 解析结果命中禁止网段（私网/环回/链路本地等）")
		}
		var lastErr error
		for _, ipa := range ips {
			target := net.JoinHostPort(ipa.IP.String(), port)
			c, err := d.DialContext(ctx, network, target)
			if err == nil {
				return c, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}

// validatingRoundTripper 在每次出站前校验 URL（含首次请求，不仅重定向）。
type validatingRoundTripper struct {
	policy Policy
	next   http.RoundTripper
}

func (v *validatingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL != nil {
		if err := v.policy.ValidateRequestURL(req.URL); err != nil {
			return nil, err
		}
	}
	return v.next.RoundTrip(req)
}

// headerRoundTripper 避免 Transport 层丢失默认 User-Agent（若上层未设置）。
type headerRoundTripper struct {
	next http.RoundTripper
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "shorturl-ssrf-guard/1.0")
	}
	return h.next.RoundTrip(req)
}
