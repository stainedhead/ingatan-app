package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestOTelMiddleware_CallsNextHandler(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	mw := OTelMiddleware(tracer)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOTelMiddleware_CapturesStatusCode200(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	mw := OTelMiddleware(tracer)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOTelMiddleware_CapturesStatusCode500(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	mw := OTelMiddleware(tracer)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/bad", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestNewOTelProvider_NoopEndpoint(t *testing.T) {
	provider, err := NewOTelProvider(OTelConfig{
		Endpoint:    "",
		ServiceName: "test",
	})
	require.NoError(t, err)
	require.NotNil(t, provider)

	err = provider.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestNewOTelProvider_StdoutEndpoint(t *testing.T) {
	provider, err := NewOTelProvider(OTelConfig{
		Endpoint:    "stdout",
		ServiceName: "test",
	})
	require.NoError(t, err)
	require.NotNil(t, provider)

	err = provider.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestPrincipalEnrichSpan_NilPrincipal(t *testing.T) {
	// No principal in context — should not panic, just call through.
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	PrincipalEnrichSpan(next).ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}
