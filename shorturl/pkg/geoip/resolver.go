package geoip

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Resolver struct {
	endpoint string
	client   *http.Client
	cacheTTL time.Duration
	mu       sync.RWMutex
	cache    map[string]cacheItem
}

type cacheItem struct {
	country string
	region  string
	expAt   time.Time
}

type ipAPIResp struct {
	Status     string `json:"status"`
	Country    string `json:"country"`
	RegionName string `json:"regionName"`
	City       string `json:"city"`
	Message    string `json:"message"`
}

func NewResolver(endpoint string, timeout, cacheTTL time.Duration) *Resolver {
	ep := strings.TrimSpace(endpoint)
	if ep == "" {
		ep = "http://ip-api.com/json"
	}
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	if cacheTTL <= 0 {
		cacheTTL = 24 * time.Hour
	}
	return &Resolver{
		endpoint: strings.TrimRight(ep, "/"),
		client: &http.Client{
			Timeout: timeout,
		},
		cacheTTL: cacheTTL,
		cache:    make(map[string]cacheItem),
	}
}

func (r *Resolver) Resolve(ctx context.Context, ip string) (country, region string, err error) {
	ip = strings.TrimSpace(ip)
	if ip == "" || !isPublicIP(ip) {
		return "", "", nil
	}

	if c, rg, ok := r.getCache(ip); ok {
		return c, rg, nil
	}

	u := fmt.Sprintf("%s/%s?lang=zh-CN&fields=status,country,regionName,city,message", r.endpoint, url.PathEscape(ip))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", "", err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("geoip status %d", resp.StatusCode)
	}

	var out ipAPIResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", err
	}
	if out.Status != "success" {
		if out.Message == "" {
			out.Message = "lookup failed"
		}
		return "", "", fmt.Errorf("geoip %s", out.Message)
	}

	country = strings.TrimSpace(out.Country)
	region = strings.TrimSpace(out.RegionName)
	if region == "" {
		region = strings.TrimSpace(out.City)
	}

	r.setCache(ip, country, region)
	return country, region, nil
}

func (r *Resolver) getCache(ip string) (country, region string, ok bool) {
	r.mu.RLock()
	item, exists := r.cache[ip]
	r.mu.RUnlock()
	if !exists || time.Now().After(item.expAt) {
		return "", "", false
	}
	return item.country, item.region, true
}

func (r *Resolver) setCache(ip, country, region string) {
	r.mu.Lock()
	r.cache[ip] = cacheItem{
		country: country,
		region:  region,
		expAt:   time.Now().Add(r.cacheTTL),
	}
	r.mu.Unlock()
}

func isPublicIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if parsed.IsLoopback() || parsed.IsUnspecified() || parsed.IsMulticast() {
		return false
	}
	if isPrivate(parsed) {
		return false
	}
	return true
}

func isPrivate(ip net.IP) bool {
	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range privateCIDRs {
		_, block, _ := net.ParseCIDR(cidr)
		if block != nil && block.Contains(ip) {
			return true
		}
	}
	return false
}
