// ============================================================
// middleware/ratelimit.go — Simple in-memory rate limiter
// Token bucket per IP address, 100 requests per minute default
// ============================================================
package middleware

import (
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	count    int
	resetAt  time.Time
}

var (
	mu      sync.Mutex
	buckets = map[string]*bucket{}
)

// RateLimit returns middleware limiting each IP to `limit` req/min
func RateLimit(limit int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for SSE stream (it's a long-lived connection)
			if r.URL.Path == "/api/stream" {
				next.ServeHTTP(w, r)
				return
			}

			ip := r.RemoteAddr
			mu.Lock()
			b, ok := buckets[ip]
			if !ok || time.Now().After(b.resetAt) {
				buckets[ip] = &bucket{count: 1, resetAt: time.Now().Add(time.Minute)}
				mu.Unlock()
				next.ServeHTTP(w, r)
				return
			}
			b.count++
			if b.count > limit {
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			mu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}
