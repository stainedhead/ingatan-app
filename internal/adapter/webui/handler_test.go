package webui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/adapter/webui"
	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stainedhead/ingatan/internal/infrastructure/backup"
	principaluc "github.com/stainedhead/ingatan/internal/usecase/principal"
	storeuc "github.com/stainedhead/ingatan/internal/usecase/store"
)

const testToken = "test-startup-token-abc123"

// buildRouter mounts the WebUI handler on a fresh chi router.
func buildRouter(h *webui.Handler) http.Handler {
	r := chi.NewRouter()
	h.Register(r)
	return r
}

// localReq creates a GET request from a loopback address.
func localReq(method, path string, body ...string) *http.Request {
	var bodyReader *strings.Reader
	if len(body) > 0 {
		bodyReader = strings.NewReader(body[0])
	} else {
		bodyReader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.RemoteAddr = "127.0.0.1:55001"
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return req
}

// doLogin performs a successful login and returns the session cookie value.
func doLogin(t *testing.T, router http.Handler, token string) string {
	t.Helper()
	form := url.Values{"token": {token}}
	req := localReq(http.MethodPost, "/webui/login", form.Encode())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("login: expected 303, got %d — %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "ingatan-admin-session" {
			return c.Value
		}
	}
	t.Fatal("login: no session cookie in response")
	return ""
}

// authedReq returns a request with the given session cookie attached.
func authedReq(method, path, sessionID string) *http.Request {
	req := localReq(method, path)
	req.AddCookie(&http.Cookie{Name: "ingatan-admin-session", Value: sessionID})
	return req
}

func newHandler() *webui.Handler {
	sessions := webui.NewSessionStore(time.Hour)
	return webui.NewHandler(testToken, sessions, &stubPrincipalSvc{}, &stubStoreSvc{}, nil)
}

// containsAll asserts that body contains every expected substring.
func containsAll(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("expected body to contain %q", want)
		}
	}
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestWebUI_NonLocalhost_Returns403(t *testing.T) {
	h := newHandler()
	router := buildRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/webui/login", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-localhost, got %d", w.Code)
	}
}

func TestWebUI_LoginGet_Returns200(t *testing.T) {
	h := newHandler()
	router := buildRouter(h)

	req := localReq(http.MethodGet, "/webui/login")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for login page, got %d", w.Code)
	}
	body := w.Body.String()
	// Check for key semantic content present in both inline HTML and templ output.
	containsAll(t, body,
		"ingatan Admin",
		"Sign in",
		`action="/webui/login"`,
		"<form",
		"<input",
	)
}

func TestWebUI_LoginPost_WrongToken_Returns200WithError(t *testing.T) {
	h := newHandler()
	router := buildRouter(h)

	form := url.Values{"token": {"wrong-token"}}
	req := localReq(http.MethodPost, "/webui/login", form.Encode())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on wrong token, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Incorrect token") {
		t.Error("expected 'Incorrect token' error message in response body")
	}
	// No session cookie must be set on failure.
	for _, c := range w.Result().Cookies() {
		if c.Name == "ingatan-admin-session" {
			t.Error("must not set session cookie on failed login")
		}
	}
}

func TestWebUI_LoginPost_CorrectToken_SetsSessionAndRedirects(t *testing.T) {
	h := newHandler()
	router := buildRouter(h)

	form := url.Values{"token": {testToken}}
	req := localReq(http.MethodPost, "/webui/login", form.Encode())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on correct token, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/webui/dashboard" {
		t.Fatalf("expected redirect to /webui/dashboard, got %s", loc)
	}

	var found bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "ingatan-admin-session" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected session cookie in login response")
	}
}

func TestWebUI_Dashboard_WithoutSession_Redirects(t *testing.T) {
	h := newHandler()
	router := buildRouter(h)

	req := localReq(http.MethodGet, "/webui/dashboard")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect without session, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/webui/login" {
		t.Fatalf("expected redirect to /webui/login, got %s", loc)
	}
}

func TestWebUI_Dashboard_WithValidSession_Returns200(t *testing.T) {
	h := newHandler()
	router := buildRouter(h)

	sessionID := doLogin(t, router, testToken)

	req := authedReq(http.MethodGet, "/webui/dashboard", sessionID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on dashboard with valid session, got %d", w.Code)
	}
	body := w.Body.String()
	// Navigation chrome and page heading must be present.
	containsAll(t, body,
		"Dashboard",
		"ingatan Admin",
		"Principals",
		"Stores",
		"System",
		"<nav",
	)
	// Logout form must be reachable from every authenticated page.
	containsAll(t, body,
		`action="/webui/logout"`,
		"<form",
	)
}

func TestWebUI_Logout_ClearsSessionAndRedirects(t *testing.T) {
	h := newHandler()
	router := buildRouter(h)

	sessionID := doLogin(t, router, testToken)

	// POST /webui/logout
	req := authedReq(http.MethodPost, "/webui/logout", sessionID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 after logout, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/webui/login" {
		t.Fatalf("expected redirect to /webui/login after logout, got %s", loc)
	}

	// Subsequent dashboard request with same session must be rejected.
	req2 := authedReq(http.MethodGet, "/webui/dashboard", sessionID)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 after using revoked session, got %d", w2.Code)
	}
}

func TestWebUI_PrincipalsList_WithSession(t *testing.T) {
	h := newHandler()
	router := buildRouter(h)

	sessionID := doLogin(t, router, testToken)
	req := authedReq(http.MethodGet, "/webui/principals", sessionID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for principals list, got %d", w.Code)
	}
	body := w.Body.String()
	containsAll(t, body,
		"Principals",
		"<table",
		"ingatan Admin",
	)
}

func TestWebUI_StoresList_WithSession(t *testing.T) {
	h := newHandler()
	router := buildRouter(h)

	sessionID := doLogin(t, router, testToken)
	req := authedReq(http.MethodGet, "/webui/stores", sessionID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for stores list, got %d", w.Code)
	}
	body := w.Body.String()
	containsAll(t, body,
		"Stores",
		"<table",
		"ingatan Admin",
	)
}

func TestWebUI_System_WithSession(t *testing.T) {
	h := newHandler()
	router := buildRouter(h)

	sessionID := doLogin(t, router, testToken)
	req := authedReq(http.MethodGet, "/webui/system", sessionID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for system page, got %d", w.Code)
	}
	body := w.Body.String()
	containsAll(t, body,
		"System",
		"ingatan Admin",
	)
}

func TestWebUI_Redirect_Root_To_Dashboard(t *testing.T) {
	h := newHandler()
	router := buildRouter(h)

	sessionID := doLogin(t, router, testToken)
	req := authedReq(http.MethodGet, "/webui/", sessionID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect from /webui/ to /webui/dashboard, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/webui/dashboard" {
		t.Fatalf("expected redirect to /webui/dashboard, got %s", loc)
	}
}

func TestWebUI_Backup_NoProviders(t *testing.T) {
	h := newHandler()
	router := buildRouter(h)

	sessionID := doLogin(t, router, testToken)
	req := authedReq(http.MethodPost, "/webui/system/backup", sessionID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for backup with no providers, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "No backup providers ran") {
		t.Error("expected 'No backup providers ran' message")
	}
}

func TestWebUI_Backup_WithProvider_Success(t *testing.T) {
	sessions := webui.NewSessionStore(time.Hour)
	stub := &stubBackup{name: "test-backup"}
	h := webui.NewHandler(testToken, sessions, &stubPrincipalSvc{}, &stubStoreSvc{}, []backup.Backuper{stub})
	router := buildRouter(h)

	sessionID := doLogin(t, router, testToken)
	req := authedReq(http.MethodPost, "/webui/system/backup?provider=test-backup", sessionID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for backup, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "success") {
		t.Errorf("expected 'success' in backup result, got: %s", w.Body.String())
	}
}

// ─── Stub implementations ────────────────────────────────────────────────────

type stubPrincipalSvc struct{}

func (s *stubPrincipalSvc) GetOrCreate(_ context.Context, _ apimw.JWTClaims) (*domain.Principal, error) {
	return nil, nil
}
func (s *stubPrincipalSvc) WhoAmI(_ context.Context, p *domain.Principal) (*principaluc.WhoAmIResponse, error) {
	return &principaluc.WhoAmIResponse{Principal: p}, nil
}
func (s *stubPrincipalSvc) List(_ context.Context, _ *domain.Principal) ([]*domain.Principal, error) {
	return []*domain.Principal{}, nil
}
func (s *stubPrincipalSvc) Create(_ context.Context, _ *domain.Principal, req principaluc.CreateRequest) (*principaluc.CreateResponse, error) {
	p := &domain.Principal{ID: "new-id", Name: req.Name, Type: req.Type, Role: req.Role}
	return &principaluc.CreateResponse{Principal: p, APIKey: "igt_testkey123"}, nil
}
func (s *stubPrincipalSvc) ReissueAPIKey(_ context.Context, _ *domain.Principal, _ string) (string, error) {
	return "igt_reissued", nil
}
func (s *stubPrincipalSvc) RevokeAPIKey(_ context.Context, _ *domain.Principal, _ string) error {
	return nil
}
func (s *stubPrincipalSvc) AuthenticateByAPIKey(_ context.Context, _ string) (*domain.Principal, error) {
	return nil, nil
}

type stubStoreSvc struct{}

func (s *stubStoreSvc) Create(_ context.Context, req storeuc.CreateRequest) (*domain.Store, error) {
	return &domain.Store{Name: req.Name, OwnerID: req.Principal.ID}, nil
}
func (s *stubStoreSvc) Get(_ context.Context, name string, _ *domain.Principal) (*domain.Store, error) {
	return &domain.Store{Name: name, OwnerID: "owner-id"}, nil
}
func (s *stubStoreSvc) List(_ context.Context, _ *domain.Principal) ([]*domain.Store, error) {
	return []*domain.Store{}, nil
}
func (s *stubStoreSvc) Delete(_ context.Context, _, _ string, _ *domain.Principal) error {
	return nil
}

type stubBackup struct {
	name string
	err  error
}

func (b *stubBackup) Backup(_ context.Context, _ string) error { return b.err }
func (b *stubBackup) Name() string                             { return b.name }
