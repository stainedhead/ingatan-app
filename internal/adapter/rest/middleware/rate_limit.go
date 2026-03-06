package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// ipLimiter holds a per-IP token bucket limiter.
type ipLimiter struct {
	limiter *rate.Limiter
}

// RateLimitMiddleware returns a per-IP token bucket rate limiter middleware.
// rps is tokens per second and burst is the maximum burst size.
// When the rate is exceeded it responds 429 with a structured JSON error body.
// Limiters are keyed by the client IP address (port stripped from RemoteAddr).
func RateLimitMiddleware(rps float64, burst int) func(http.Handler) http.Handler {
	var limiters sync.Map // map[string]*ipLimiter

	getOrCreate := func(ip string) *rate.Limiter {
		v, _ := limiters.LoadOrStore(ip, &ipLimiter{
			limiter: rate.NewLimiter(rate.Limit(rps), burst),
		})
		return v.(*ipLimiter).limiter //nolint:forcetypeassert // sync.Map always stores *ipLimiter
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := remoteIP(r.RemoteAddr)
			lim := getOrCreate(ip)

			if !lim.Allow() {
				writeRateLimited(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// remoteIP extracts the host portion from addr (strips port).
// Falls back to addr unchanged if it cannot be parsed.
func remoteIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// writeRateLimited writes a 429 Too Many Requests JSON response.
func writeRateLimited(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "RATE_LIMITED",
			"message": "too many requests",
		},
	})
}
