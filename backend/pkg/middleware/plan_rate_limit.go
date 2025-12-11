package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// PlanBasedRateLimiter quản lý rate limiting dựa trên plan
type PlanBasedRateLimiter struct {
	limiters map[string]*RateLimiter // key: userID hoặc IP
	mu       sync.RWMutex
	window   time.Duration
	cleanup  *time.Ticker
}

// planRateLimits định nghĩa rate limit cho mỗi plan
// 0 = no limit
var planRateLimits = map[string]int{
	"standard": 0, // No limit
	"premium":  0, // No limit
}

// NewPlanBasedRateLimiter tạo rate limiter mới dựa trên plan
func NewPlanBasedRateLimiter(window time.Duration) *PlanBasedRateLimiter {
	rl := &PlanBasedRateLimiter{
		limiters: make(map[string]*RateLimiter),
		window:   window,
		cleanup:  time.NewTicker(5 * time.Minute),
	}

	// Cleanup old limiters
	go func() {
		for range rl.cleanup.C {
			rl.cleanupLimiters()
		}
	}()

	return rl
}

// cleanupLimiters xóa các limiter cũ
func (rl *PlanBasedRateLimiter) cleanupLimiters() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Cleanup được xử lý bởi từng RateLimiter riêng
	// Chỉ cần cleanup map nếu cần
}

// getLimiter lấy hoặc tạo limiter mới cho user/IP
func (rl *PlanBasedRateLimiter) getLimiter(key string, plan string) *RateLimiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[key]
	if !exists {
		rate := planRateLimits[plan]
		if rate == 0 {
			rate = planRateLimits["standard"] // Default to standard
		}
		limiter = NewRateLimiter(rate, rl.window)
		rl.limiters[key] = limiter
	} else {
		// Update rate if plan changed
		rate := planRateLimits[plan]
		if rate == 0 {
			rate = planRateLimits["standard"]
		}
		limiter.mu.Lock()
		limiter.rate = rate
		limiter.mu.Unlock()
	}

	return limiter
}

// Allow kiểm tra xem request có được phép không
func (rl *PlanBasedRateLimiter) Allow(key string, plan string) bool {
	rate := planRateLimits[plan]
	if rate == 0 {
		rate = planRateLimits["standard"]
	}
	// Nếu rate limit = 0, không giới hạn
	if rate == 0 {
		return true
	}
	limiter := rl.getLimiter(key, plan)
	return limiter.Allow(key)
}

// PlanBasedRateLimit middleware áp dụng rate limiting dựa trên plan
// getPlanFunc: function để lấy plan từ request context
// getUserIDFunc: function để lấy user ID từ request context
func PlanBasedRateLimit(window time.Duration, getPlanFunc func(*http.Request) string, getUserIDFunc func(*http.Request) string) func(http.Handler) http.Handler {
	limiter := NewPlanBasedRateLimiter(window)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Lấy plan từ context
			plan := getPlanFunc(r)
			if plan == "" {
				// Nếu không có plan (unauthenticated), dùng standard plan
				plan = "standard"
			}

			// Kiểm tra rate limit cho plan này
			rate := planRateLimits[plan]
			if rate == 0 {
				rate = planRateLimits["standard"]
			}
			// Nếu rate limit = 0, không giới hạn - cho phép request
			if rate == 0 {
				next.ServeHTTP(w, r)
				return
			}

			// Sử dụng userID nếu có, nếu không thì dùng IP
			key := getUserIDFunc(r)
			if key == "" {
				key = getClientIP(r)
			}

			if !limiter.Allow(key, plan) {
				corrID := GetCorrelationID(r.Context())
				log.Warn().
					Str("correlation_id", corrID).
					Str("key", key).
					Str("plan", plan).
					Msg("Rate limit exceeded")

				WriteErrorResponse(w, r.Context(), http.StatusTooManyRequests, "Rate limit exceeded", "RATE_LIMIT_EXCEEDED", "")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

