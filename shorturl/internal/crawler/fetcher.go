package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/zeromicro/go-zero/core/breaker"
)

type PageInfo struct {
	Title       string
	Description string
}

type Fetcher struct {
	client  *http.Client
	maxBody int64
}

// Options 构建 Fetcher；HTTPClient 须为 ssrf.NewUserURLHTTPClient 等受控客户端，勿传裸 http.Client。
type Options struct {
	HTTPClient   *http.Client
	MaxBodyBytes int64
}

func NewFetcher(opt Options) *Fetcher {
	if opt.MaxBodyBytes <= 0 {
		opt.MaxBodyBytes = 2 * 1024 * 1024
	}
	return &Fetcher{
		client:  opt.HTTPClient,
		maxBody: opt.MaxBodyBytes,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*PageInfo, error) {
	if f.client == nil {
		return nil, fmt.Errorf("fetcher: http client not configured")
	}
	var info *PageInfo
	err := breaker.GetBreaker("http-fetch").DoCtx(ctx, func() error {
		var e error
		info, e = f.fetchOnce(ctx, rawURL)
		return e
	})
	return info, err
}

func (f *Fetcher) fetchOnce(ctx context.Context, rawURL string) (*PageInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "shorturl-bot/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetch failed status=%d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBody))
	if err != nil {
		return nil, err
	}
	html := string(body)

	return &PageInfo{
		Title:       extractTitle(html),
		Description: extractMetaDescription(html),
	}, nil
}

func extractTitle(html string) string {
	re := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(stripTags(matches[1]))
}

func extractMetaDescription(html string) string {
	re := regexp.MustCompile(`(?is)<meta[^>]*name=["']description["'][^>]*content=["'](.*?)["'][^>]*>`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(stripTags(matches[1]))
}

func stripTags(s string) string {
	re := regexp.MustCompile(`(?is)<[^>]+>`)
	return re.ReplaceAllString(s, "")
}
