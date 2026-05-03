package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type PageInfo struct {
	Title       string
	Description string
}

type Fetcher struct {
	client *http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*PageInfo, error) {
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
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
