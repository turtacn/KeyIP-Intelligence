package middleware

import (
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]int
}

var limiter = &rateLimiter{buckets: make(map[string]int)}

func RateLimit(max int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limiter.mu.Lock()
			count := limiter.buckets[r.RemoteAddr]
			if count >= max {
				limiter.mu.Unlock()
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			limiter.buckets[r.RemoteAddr]++
			limiter.mu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}

//Personal.AI order the ending
