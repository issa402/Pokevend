// ============================================================
// FILE: server/middleware/ratelimit.go
// TYPE: Middleware — Request Rate Limiting
//
// WHAT IS RATE LIMITING?
// Rate limiting restricts how many requests a single client can make
// in a given time window. Without it:
//   - Bots scrape your entire database in minutes
//   - One angry user can trigger thousands of DB queries
//   - Auth endpoints are vulnerable to brute-force password guessing
//
// FAANG STANDARD: Every public API has rate limiting.
// Stripe: 100 requests/second per API key
// GitHub: 60 requests/hour unauthenticated, 5000 authenticated
// Twitter: 300 requests/15 minutes per endpoint
// Our API: 100 requests/second per IP
//
// ALGORITHM: Token Bucket (what golang.org/x/time/rate implements)
// Imagine a bucket that holds 100 tokens, refilled at 100 tokens/second.
// Each request takes one token. When the bucket is empty → 429 Too Many Requests.
// Burst: allows short spikes above the limit (empty bucket fills instantly).
//
// WHY PER-IP, NOT PER-USER?
// For unauthenticated routes (login, register), we don't have a user ID yet.
// Limiting per IP catches bots and scrapers before they can authenticate.
// For authenticated routes: per-user limiting would be more precise
// (a user on a shared IP shouldn't affect others).
//
// GO CONCEPTS:
//   sync.Map (concurrent map), golang.org/x/time/rate.Limiter,
//   r.RemoteAddr (caller's IP), strings.Split
// ============================================================
package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter stores per-IP limiter state.
// sync.Map is a concurrent-safe map — safe to read/write from multiple goroutines.
// Regular map in Go is NOT goroutine-safe; concurrent access causes a race condition/crash.
//
// We use sync.Map because multiple goroutines handle requests simultaneously
// and all of them read/write this map to check/update IP rate limits.
type IPRateLimiter struct {
	limiters sync.Map         // IP string → *rate.Limiter
	rps      rate.Limit       // requests per second
	burst    int              // max burst size
}

// RateLimit is the middleware factory — creates a rate limiting middleware.
// rps = requests per second allowed per IP
//
// GO PATTERN: Middleware Factory (a function that returns a middleware)
// This lets us configure the rate limit from main.go without a global variable.
func RateLimit(rps float64) func(http.Handler) http.Handler {
	limiter := &IPRateLimiter{
		rps:   rate.Limit(rps),
		burst: int(rps), // allow bursting to rps requests at once
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract the real client IP from the request.
			// r.RemoteAddr = "1.2.3.4:50234" (IP:port format)
			// strings.Split(addr, ":")[0] extracts just "1.2.3.4"
			ip := strings.Split(r.RemoteAddr, ":")[0]

			// Get or create a limiter for this IP
			// sync.Map.LoadOrStore: if key exists → return existing; else store new value
			// This atomically handles concurrent requests from the same IP correctly
			l, _ := limiter.limiters.LoadOrStore(ip,
				rate.NewLimiter(limiter.rps, limiter.burst))

			// l.Allow() = check if a request is allowed right now.
			// If yes: consume one token and return true.
			// If no (rate exceeded): return false (don't consume token).
			if !l.(*rate.Limiter).Allow() {
				// HTTP 429 Too Many Requests — the standard rate limit response
				// Retry-After header tells the client when to try again (good API design)
				w.Header().Set("Retry-After", "1")
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// cleanupOldLimiters removes limiters for IPs that haven't made requests recently.
// Call this on a goroutine periodically to prevent memory growth:
//   go func() {
//     for range time.NewTicker(1 * time.Hour).C {
//       cleanupOldLimiters(limiter)
//     }
//   }()
// (This is a TODO left for the reader — see below)
func cleanupOldLimiters(l *IPRateLimiter) {
	// sync.Map.Range iterates over all key-value pairs
	// The range callback returns false to stop early, true to continue
	l.limiters.Range(func(key, _ interface{}) bool {
		// In a more complete implementation: check lastSeen timestamp
		// For now: just an example of how sync.Map.Range works
		_ = time.Second // placeholder
		return true
	})
}

// TODO #1 (Practice): Track "last seen" time per IP and evict old limiters
// Currently limiters are created and never deleted — memory grows forever.
// Create a struct { limiter *rate.Limiter; lastSeen time.Time }
// Update lastSeen on every request. In cleanupOldLimiters, delete entries
// where lastSeen > 1 hour ago. This bounds memory usage to active IPs only.

// TODO #2 (Practice): Read limits from Redis for distributed rate limiting
// Our current limiter is IN-MEMORY — only works for a SINGLE server instance.
// If you scale to 3 Go server instances, each has its own limiter.
// A user could make 100 req/s * 3 servers = 300 req/s and bypass the limit.
// Fix: store request counts in Redis (shared across all instances):
//   INCR ratelimit:<ip> EX 1  (increment + set TTL of 1 second)
//   If count > limit → reject
// Research: "Redis INCR rate limiting pattern" — FAANG standard for distributed RL
