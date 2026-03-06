package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRateLimitMiddleware_AllowsWithinBurst(t *testing.T) {
	// rps=1, burst=3: first 3 requests should pass.
	mw := RateLimitMiddleware(1.0, 3)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	for i := range 3 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should be allowed", i+1)
	}
}

func TestRateLimitMiddleware_BlocksWhenExceeded(t *testing.T) {
	// rps=1, burst=2: first 2 allowed, 3rd blocked.
	mw := RateLimitMiddleware(1.0, 2)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	// Drain the burst.
	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.5:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// This one should be rate-limited.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.5:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestRateLimitMiddleware_IndependentPerIP(t *testing.T) {
	// rps=1, burst=1: each IP has its own limiter.
	mw := RateLimitMiddleware(1.0, 1)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	// IP A: first request allowed.
	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.RemoteAddr = "1.2.3.4:80"
	rrA := httptest.NewRecorder()
	handler.ServeHTTP(rrA, reqA)
	assert.Equal(t, http.StatusOK, rrA.Code)

	// IP A: second request blocked.
	reqA2 := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA2.RemoteAddr = "1.2.3.4:80"
	rrA2 := httptest.NewRecorder()
	handler.ServeHTTP(rrA2, reqA2)
	assert.Equal(t, http.StatusTooManyRequests, rrA2.Code)

	// IP B: first request allowed (independent limiter).
	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.RemoteAddr = "5.6.7.8:80"
	rrB := httptest.NewRecorder()
	handler.ServeHTTP(rrB, reqB)
	assert.Equal(t, http.StatusOK, rrB.Code)
}

func TestRateLimitMiddleware_429ResponseBody(t *testing.T) {
	mw := RateLimitMiddleware(1.0, 1)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	ip := "9.9.9.9:1111"
	// Drain burst.
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = ip
	handler.ServeHTTP(httptest.NewRecorder(), req1)

	// Second request should 429.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = ip
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req2)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Contains(t, rr.Body.String(), "RATE_LIMITED")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}
