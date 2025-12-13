package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// RateLimiter quản lý rate limiting
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int           // Số request cho phép
	window   time.Duration // Khoảng thời gian
	cleanup  *time.Ticker
}

type visitor struct {
	count    int
	lastSeen time.Time
}

// NewRateLimiter tạo rate limiter mới
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		window:   window,
		cleanup:  time.NewTicker(1 * time.Minute),
	}

	// Cleanup old visitors
	go func() {
		for range rl.cleanup.C {
			rl.cleanupVisitors()
		}
	}()

	return rl
}

// cleanupVisitors xóa các visitor cũ
func (rl *RateLimiter) cleanupVisitors() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, v := range rl.visitors {
		if now.Sub(v.lastSeen) > rl.window {
			delete(rl.visitors, ip)
		}
	}
}

// getVisitor lấy hoặc tạo visitor mới
func (rl *RateLimiter) getVisitor(ip string) *visitor {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		v = &visitor{
			count:    0,
			lastSeen: time.Now(),
		}
		rl.visitors[ip] = v
	}

	return v
}

// Allow checks if the request is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	v := rl.getVisitor(ip)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Reset if window has expired
	if now.Sub(v.lastSeen) > rl.window {
		v.count = 0
		v.lastSeen = now
	}

	// Check rate limit
	if v.count >= rl.rate {
		return false
	}

	v.count++
	v.lastSeen = now
	return true
}

// RateLimit middleware áp dụng rate limiting
func RateLimit(rate int, window time.Duration) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(rate, window)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			if !limiter.Allow(ip) {
				corrID := GetCorrelationID(r.Context())
				log.Warn().
					Str("correlation_id", corrID).
					Str("ip", ip).
					Msg("Rate limit exceeded")

				WriteErrorResponse(w, r.Context(), http.StatusTooManyRequests, "Rate limit exceeded", "RATE_LIMIT_EXCEEDED", "")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP gets the client IP address
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (when behind proxy)
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}

	// Check X-Real-IP header
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Fallback về RemoteAddr
	return r.RemoteAddr
}
