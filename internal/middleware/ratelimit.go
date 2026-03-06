package middleware

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type visitor struct {
	tokens   float64
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     float64 // tokens per second
	burst    float64 // max tokens
}

func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     float64(requestsPerMinute) / 60.0,
		burst:    float64(requestsPerMinute),
	}

	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// allow checks if a request from the given IP is allowed and returns
// the current token count and the time when the bucket will be full again.
func (rl *RateLimiter) allow(ip string) (bool, float64, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	now := time.Now()

	if !exists {
		rl.visitors[ip] = &visitor{tokens: rl.burst - 1, lastSeen: now}
		resetTime := now.Add(time.Duration(1/rl.rate) * time.Second)
		return true, rl.burst - 1, resetTime
	}

	elapsed := now.Sub(v.lastSeen).Seconds()
	v.tokens += elapsed * rl.rate
	if v.tokens > rl.burst {
		v.tokens = rl.burst
	}
	v.lastSeen = now

	// Calculate reset time: when will the bucket be full again?
	deficit := rl.burst - v.tokens
	resetDuration := time.Duration(deficit/rl.rate) * time.Second
	resetTime := now.Add(resetDuration)

	if v.tokens < 1 {
		return false, v.tokens, resetTime
	}

	v.tokens--
	return true, v.tokens, resetTime
}

// Middleware enforces rate limiting and adds standard rate limit headers to every response:
//   - X-RateLimit-Limit: maximum requests per window
//   - X-RateLimit-Remaining: remaining requests in current window
//   - X-RateLimit-Reset: Unix timestamp when the limit resets
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)

		allowed, remaining, resetTime := rl.allow(ip)

		// Always set rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(int(rl.burst)))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(int(math.Max(0, math.Floor(remaining)))))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime.Unix()))

		if !allowed {
			retryAfter := int(math.Ceil(time.Until(resetTime).Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "rate limit exceeded",
				"code":  429,
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first (for reverse proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func ReadLimiter() *RateLimiter {
	return NewRateLimiter(60)
}

func WriteLimiter() *RateLimiter {
	return NewRateLimiter(10)
}
