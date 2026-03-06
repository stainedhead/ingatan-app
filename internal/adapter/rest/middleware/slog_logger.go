// Package middleware: slog_logger provides a structured JSON request logger for Chi.
package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// SlogLogger returns a Chi middleware that logs each HTTP request as a structured
// JSON log line using the provided slog.Logger. It records:
//   - method, path, status_code, duration_ms, request_id, remote_addr
//   - principal_id (if a principal is in the request context)
func SlogLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				reqID := middleware.GetReqID(r.Context())
				principal := PrincipalFromContext(r.Context())
				principalID := ""
				if principal != nil {
					principalID = principal.ID
				}

				durationMS := float64(time.Since(start).Microseconds()) / 1000.0

				attrs := []any{
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.Status(),
					"duration_ms", durationMS,
					"request_id", reqID,
					"remote_addr", r.RemoteAddr,
				}
				if principalID != "" {
					attrs = append(attrs, "principal_id", principalID)
				}

				switch {
				case ww.Status() >= 500:
					logger.Error("request", attrs...)
				case ww.Status() >= 400:
					logger.Warn("request", attrs...)
				default:
					logger.Info("request", attrs...)
				}
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
