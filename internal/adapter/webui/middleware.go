package webui

import (
	"net"
	"net/http"
)

const sessionCookieName = "ingatan-admin-session"

// LocalhostOnly is HTTP middleware that rejects requests from non-loopback addresses.
// This restricts the Admin WebUI to connections from 127.0.0.1 or ::1 only,
// requiring physical or SSH access to the host running ingatan.
func LocalhostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || !isLoopback(host) {
			http.Error(w, "Admin WebUI is only accessible from localhost", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SessionAuth returns HTTP middleware that validates the admin session cookie.
// Requests without a valid session are redirected to /webui/login.
func SessionAuth(sessions *SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || !sessions.Valid(cookie.Value) {
				http.Redirect(w, r, "/webui/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isLoopback reports whether host (without port) is a loopback address.
// Covers both 127.0.0.1 (IPv4) and ::1 (IPv6).
func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
