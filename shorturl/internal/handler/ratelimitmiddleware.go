package handler

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"shorturl/internal/config"

	"golang.org/x/time/rate"
)

const (
	defaultPerIPQPS        = 40
	defaultPerIPBurst      = 80
	defaultHeavyPerIPQPS   = 8
	defaultHeavyPerIPBurst = 16
	defaultMaxTrackedIPs   = 50000
)

type dualLimiter struct {
	light *rate.Limiter
	heavy *rate.Limiter
}

type rateLimitState struct {
	mu            sync.Mutex
	byIP          map[string]*dualLimiter
	light         rate.Limit
	lBurst        int
	heavy         rate.Limit
	hBurst        int
	maxTrackedIPs int
}

// RateLimitMiddleware 按 IP 限流；重路径（/convert、/analyze）使用更严配额。
func RateLimitMiddleware(c config.Config) func(http.HandlerFunc) http.HandlerFunc {
	if !c.RateLimit.Enabled {
		return nil
	}
	lq := c.RateLimit.PerIPQPS
	lb := c.RateLimit.PerIPBurst
	hq := c.RateLimit.HeavyPerIPQPS
	hb := c.RateLimit.HeavyPerIPBurst
	if lq <= 0 {
		lq = defaultPerIPQPS
	}
	if lb <= 0 {
		lb = defaultPerIPBurst
	}
	if hq <= 0 {
		hq = defaultHeavyPerIPQPS
	}
	if hb <= 0 {
		hb = defaultHeavyPerIPBurst
	}
	maxIP := c.RateLimit.MaxTrackedIPs
	if maxIP <= 0 {
		maxIP = defaultMaxTrackedIPs
	}

	st := &rateLimitState{
		byIP:          make(map[string]*dualLimiter),
		light:         rate.Limit(lq),
		lBurst:        lb,
		heavy:         rate.Limit(hq),
		hBurst:        hb,
		maxTrackedIPs: maxIP,
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			lim := st.getOrCreate(ip)
			var ok bool
			if isHeavyPath(r.Method, r.URL.Path) {
				ok = lim.heavy.Allow()
			} else {
				ok = lim.light.Allow()
			}
			if !ok {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte("请求过于频繁，请稍后再试"))
				return
			}
			next(w, r)
		}
	}
}

func (st *rateLimitState) getOrCreate(ip string) *dualLimiter {
	st.mu.Lock()
	defer st.mu.Unlock()
	if d, ok := st.byIP[ip]; ok {
		return d
	}
	if len(st.byIP) >= st.maxTrackedIPs {
		st.evictLimiters()
	}
	d := &dualLimiter{
		light: rate.NewLimiter(st.light, st.lBurst),
		heavy: rate.NewLimiter(st.heavy, st.hBurst),
	}
	st.byIP[ip] = d
	return d
}

// evictLimiters 在达到上限时删除一批条目（依赖 map 迭代随机性），避免长期运行内存线性增长。
func (st *rateLimitState) evictLimiters() {
	target := st.maxTrackedIPs / 10
	if target < 1 {
		target = 1
	}
	n := 0
	for k := range st.byIP {
		delete(st.byIP, k)
		n++
		if n >= target {
			break
		}
	}
}

func isHeavyPath(method, path string) bool {
	if path == "/convert" && method == http.MethodPost {
		return true
	}
	if path == "/analyze" && method == http.MethodPost {
		return true
	}
	return false
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
