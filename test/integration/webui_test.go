package integration_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stainedhead/ingatan/internal/adapter/rest"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/adapter/webui"
	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stainedhead/ingatan/internal/infrastructure/config"
	"github.com/stainedhead/ingatan/internal/infrastructure/index"
	"github.com/stainedhead/ingatan/internal/infrastructure/ingest"
	"github.com/stainedhead/ingatan/internal/infrastructure/storage"
	conversationuc "github.com/stainedhead/ingatan/internal/usecase/conversation"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
	principaluc "github.com/stainedhead/ingatan/internal/usecase/principal"
	storeuc "github.com/stainedhead/ingatan/internal/usecase/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const webuiTestToken = "integration-test-webui-token-abc123"

// webuiTestServer wraps an httptest.Server that serves both the REST API and the
// Admin WebUI mounted on the same root chi.Router.
type webuiTestServer struct {
	srv   *httptest.Server
	Token string
}

// URL returns the base URL of the test server.
func (s *webuiTestServer) URL() string { return s.srv.URL }

// newWebuiTestServer creates a full server with real services and both the REST
// sub-router and the WebUI handler mounted on a root chi.Router.
func newWebuiTestServer(t *testing.T) *webuiTestServer {
	t.Helper()

	dataDir := t.TempDir()

	// Infrastructure: storage
	fs := storage.NewFileStore(dataDir)
	memRepo := storage.NewMemoryRepo(fs)
	chunkRepo := storage.NewChunkRepo(fs)
	storeRepo := storage.NewStoreRepo(fs)
	principalRepo := storage.NewPrincipalRepo(fs)
	conversationRepo := storage.NewConversationRepo(fs)
	msgRepo := storage.NewMessageRepo(fs)

	// Infrastructure: chunker (real, no embedder)
	chunker := ingest.NewRecursiveChunker(config.ChunkingConfig{
		Strategy:        "recursive",
		ChunkSize:       512,
		ChunkOverlap:    64,
		MaxContentBytes: 1048576,
	})

	var embedder memoryuc.Embedder

	// Infrastructure: per-store index registries
	dims := 1536
	hnswStore := index.NewHNSWStore(dataDir, dims)
	bm25Store := index.NewBM25Store(dataDir)

	// Services
	storeSvc := storeuc.NewService(storeRepo)
	principalSvc := principaluc.NewService(principalRepo, storeRepo)
	storeAccess := &testStoreAccess{repo: storeRepo}
	ingestOpts := memoryuc.IngestOptions{MaxContentBytes: 1048576}
	memorySvc := memoryuc.NewService(memRepo, chunkRepo, chunker, embedder, hnswStore, bm25Store, nil, nil, ingestOpts, storeAccess)
	conversationSvc := conversationuc.NewService(conversationRepo, msgRepo, nil, nil, conversationuc.AutoSummarizeConfig{})

	// REST handlers
	memoryHandler := rest.NewMemoryHandler(memorySvc)
	searchHandler := rest.NewSearchHandler(memorySvc)
	ingestHandler := rest.NewIngestHandler(memorySvc)
	storeHandler := rest.NewStoreHandler(storeSvc)
	principalHandler := rest.NewPrincipalHandler(principalSvc)
	conversationHandler := rest.NewConversationHandler(conversationSvc)
	backupHandler := rest.NewBackupHandler(nil, dataDir)

	// JWT secret and principal lookup
	testSecret := []byte("webui-integration-secret")
	lookup := func(ctx context.Context, claims apimw.JWTClaims) (*domain.Principal, error) {
		return principalSvc.GetOrCreate(ctx, claims)
	}
	sysSvc := &testSystemService{}

	// REST sub-router (no OTel, no rate limit)
	restRouter := rest.NewRouter(testSecret, lookup, sysSvc, rest.ServerOptions{},
		memoryHandler, searchHandler, ingestHandler,
		storeHandler, principalHandler, conversationHandler,
		backupHandler,
	)

	// WebUI handler
	sessions := webui.NewSessionStore(24 * time.Hour)
	t.Cleanup(sessions.Close)
	webuiHandler := webui.NewHandler(webuiTestToken, sessions, principalSvc, storeSvc, nil)

	// Root router: mount REST and WebUI together
	rootRouter := chi.NewRouter()
	rootRouter.Mount("/", restRouter)
	webuiHandler.Register(rootRouter)

	srv := httptest.NewServer(rootRouter)
	t.Cleanup(srv.Close)

	return &webuiTestServer{srv: srv, Token: webuiTestToken}
}

// webuiLogin performs a login POST and returns the session cookie value.
// It requires the response to be a 303 redirect (successful login).
func webuiLogin(t *testing.T, client *http.Client, baseURL, token string) string {
	t.Helper()
	form := url.Values{"token": {token}}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/webui/login", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "expected 303 on successful login")
	for _, c := range resp.Cookies() {
		if c.Name == "ingatan-admin-session" {
			return c.Value
		}
	}
	t.Fatal("no ingatan-admin-session cookie in login response")
	return ""
}

// webuiGet performs an authenticated GET with the given session cookie and returns
// the response body as a string. The caller is responsible for status assertions.
func webuiGet(t *testing.T, client *http.Client, baseURL, path, sessionCookie string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	require.NoError(t, err)
	if sessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: "ingatan-admin-session", Value: sessionCookie})
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(b)
}

// webuiPost performs an authenticated form POST with the given session cookie.
// It returns the raw *http.Response so callers can inspect status and headers.
func webuiPost(t *testing.T, client *http.Client, baseURL, path, sessionCookie string, form url.Values) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+path, strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if sessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: "ingatan-admin-session", Value: sessionCookie})
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// noRedirectClient returns an *http.Client that does not follow redirects.
// This lets tests inspect 303 responses directly.
func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// TestWebUI_Integration_LoginFlow covers W4.2: the full login/logout cycle.
//
//   - GET /webui/login → 200, contains "Sign in"
//   - POST /webui/login with wrong token → 200, contains "Incorrect token"
//   - POST /webui/login with correct token → 303, sets session cookie
//   - GET /webui/dashboard with cookie → 200, contains "Dashboard"
//   - POST /webui/logout → 303 to /webui/login, clears cookie
//   - GET /webui/dashboard without valid session → 303 to /webui/login
func TestWebUI_Integration_LoginFlow(t *testing.T) {
	s := newWebuiTestServer(t)
	client := noRedirectClient()

	// --- GET /webui/login → 200, "Sign in" present ---
	statusCode, body := webuiGet(t, client, s.URL(), "/webui/login", "")
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Contains(t, body, "Sign in", "login page should contain Sign in button text")

	// --- POST /webui/login with wrong token → 200, error message ---
	wrongForm := url.Values{"token": {"definitely-wrong-token"}}
	wrongReq, err := http.NewRequest(http.MethodPost, s.URL()+"/webui/login", strings.NewReader(wrongForm.Encode()))
	require.NoError(t, err)
	wrongReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wrongResp, err := client.Do(wrongReq)
	require.NoError(t, err)
	wrongBody, err := io.ReadAll(wrongResp.Body)
	require.NoError(t, err)
	_ = wrongResp.Body.Close()
	assert.Equal(t, http.StatusOK, wrongResp.StatusCode)
	assert.Contains(t, string(wrongBody), "Incorrect token")

	// --- POST /webui/login with correct token → 303, session cookie set ---
	sessionCookie := webuiLogin(t, client, s.URL(), s.Token)
	assert.NotEmpty(t, sessionCookie)

	// --- GET /webui/dashboard with valid session → 200, "Dashboard" or layout present ---
	statusCode, body = webuiGet(t, client, s.URL(), "/webui/dashboard", sessionCookie)
	assert.Equal(t, http.StatusOK, statusCode)
	// The layout title is "Dashboard — ingatan Admin" and the nav contains "Dashboard".
	assert.Contains(t, body, "Dashboard")
	assert.Contains(t, body, "ingatan Admin")

	// --- POST /webui/logout → 303 to /webui/login, MaxAge=-1 (cleared cookie) ---
	logoutResp := webuiPost(t, client, s.URL(), "/webui/logout", sessionCookie, url.Values{})
	assert.Equal(t, http.StatusSeeOther, logoutResp.StatusCode)
	assert.Equal(t, "/webui/login", logoutResp.Header.Get("Location"))
	// The cleared cookie should have MaxAge <= 0.
	var clearedCookie *http.Cookie
	for _, c := range logoutResp.Cookies() {
		if c.Name == "ingatan-admin-session" {
			clearedCookie = c
		}
	}
	require.NotNil(t, clearedCookie, "logout response should set the session cookie to clear it")
	assert.LessOrEqual(t, clearedCookie.MaxAge, 0, "logout should clear cookie with MaxAge <= 0")

	// --- GET /webui/dashboard after logout → 303 to /webui/login (session invalid) ---
	dashReq, err := http.NewRequest(http.MethodGet, s.URL()+"/webui/dashboard", nil)
	require.NoError(t, err)
	// Pass the now-invalidated session cookie to prove it no longer grants access.
	dashReq.AddCookie(&http.Cookie{Name: "ingatan-admin-session", Value: sessionCookie})
	dashResp, err := client.Do(dashReq)
	require.NoError(t, err)
	_ = dashResp.Body.Close()
	assert.Equal(t, http.StatusSeeOther, dashResp.StatusCode)
	assert.Equal(t, "/webui/login", dashResp.Header.Get("Location"))
}

// TestWebUI_Integration_PrincipalCreate covers W4.3: the principal management pages.
//
//   - Login
//   - GET /webui/principals → 200, contains "Principals" and a table
//   - POST /webui/principals → 200, contains "Principal Created" and the "igt_" API key prefix
func TestWebUI_Integration_PrincipalCreate(t *testing.T) {
	s := newWebuiTestServer(t)
	client := noRedirectClient()

	sessionCookie := webuiLogin(t, client, s.URL(), s.Token)

	// --- GET /webui/principals → 200, list page ---
	statusCode, body := webuiGet(t, client, s.URL(), "/webui/principals", sessionCookie)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Contains(t, body, "Principals")
	assert.Contains(t, body, "<table")

	// --- POST /webui/principals → create principal → 200, show created page ---
	createForm := url.Values{
		"name":  {"Test User"},
		"type":  {"human"},
		"role":  {"user"},
		"email": {""},
	}
	createReq, err := http.NewRequest(http.MethodPost, s.URL()+"/webui/principals", strings.NewReader(createForm.Encode()))
	require.NoError(t, err)
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	createReq.AddCookie(&http.Cookie{Name: "ingatan-admin-session", Value: sessionCookie})
	createResp, err := client.Do(createReq)
	require.NoError(t, err)
	createBody, err := io.ReadAll(createResp.Body)
	require.NoError(t, err)
	_ = createResp.Body.Close()

	assert.Equal(t, http.StatusOK, createResp.StatusCode)
	createBodyStr := string(createBody)
	assert.Contains(t, createBodyStr, "Principal Created")
	// The real service generates keys with the "igt_" prefix.
	assert.Contains(t, createBodyStr, "igt_", "API key should contain the igt_ prefix")
	// The new principal's name should appear on the confirmation page.
	assert.Contains(t, createBodyStr, "Test User")
}

// TestWebUI_Integration_StoreList covers W4.4: the store management list page.
//
//   - Login
//   - GET /webui/stores → 200, contains "Stores" and a table
func TestWebUI_Integration_StoreList(t *testing.T) {
	s := newWebuiTestServer(t)
	client := noRedirectClient()

	sessionCookie := webuiLogin(t, client, s.URL(), s.Token)

	statusCode, body := webuiGet(t, client, s.URL(), "/webui/stores", sessionCookie)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Contains(t, body, "Stores")
	assert.Contains(t, body, "<table")
}

// TestWebUI_Integration_SystemPage verifies the system page renders successfully.
//
//   - Login
//   - GET /webui/system → 200, contains "System"
func TestWebUI_Integration_SystemPage(t *testing.T) {
	s := newWebuiTestServer(t)
	client := noRedirectClient()

	sessionCookie := webuiLogin(t, client, s.URL(), s.Token)

	statusCode, body := webuiGet(t, client, s.URL(), "/webui/system", sessionCookie)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Contains(t, body, "System")
}
