package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLocalhostOnly_Loopback_IPv4(t *testing.T) {
	handler := LocalhostOnly(okHandler())
	r := requestWithAddr("127.0.0.1:12345")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for 127.0.0.1, got %d", w.Code)
	}
}

func TestLocalhostOnly_Loopback_IPv6(t *testing.T) {
	handler := LocalhostOnly(okHandler())
	r := requestWithAddr("[::1]:12345")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for ::1, got %d", w.Code)
	}
}

func TestLocalhostOnly_NonLocal(t *testing.T) {
	handler := LocalhostOnly(okHandler())
	r := requestWithAddr("192.168.1.100:12345")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-local IP, got %d", w.Code)
	}
}

func TestLocalhostOnly_InvalidAddr(t *testing.T) {
	handler := LocalhostOnly(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "not-an-address"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid RemoteAddr, got %d", w.Code)
	}
}

func TestSessionAuth_ValidCookie(t *testing.T) {
	sessions := NewSessionStore(time.Hour)
	defer sessions.Close()

	id := sessions.Create()
	handler := SessionAuth(sessions)(okHandler())

	r := localRequest()
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: id})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid session, got %d", w.Code)
	}
}

func TestSessionAuth_MissingCookie(t *testing.T) {
	sessions := NewSessionStore(time.Hour)
	defer sessions.Close()

	handler := SessionAuth(sessions)(okHandler())
	r := localRequest()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect without cookie, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/webui/login" {
		t.Fatalf("expected redirect to /webui/login, got %s", loc)
	}
}

func TestSessionAuth_InvalidCookie(t *testing.T) {
	sessions := NewSessionStore(time.Hour)
	defer sessions.Close()

	handler := SessionAuth(sessions)(okHandler())
	r := localRequest()
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "bad-session-id"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect with invalid session, got %d", w.Code)
	}
}

// helpers

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func requestWithAddr(remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/webui/", nil)
	req.RemoteAddr = remoteAddr
	return req
}

func localRequest() *http.Request {
	return requestWithAddr("127.0.0.1:12345")
}
