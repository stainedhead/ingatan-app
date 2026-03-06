package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stainedhead/ingatan/internal/adapter/rest"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stainedhead/ingatan/internal/infrastructure/config"
	"github.com/stainedhead/ingatan/internal/infrastructure/index"
	"github.com/stainedhead/ingatan/internal/infrastructure/ingest"
	"github.com/stainedhead/ingatan/internal/infrastructure/storage"
	conversationuc "github.com/stainedhead/ingatan/internal/usecase/conversation"
	memoryuc "github.com/stainedhead/ingatan/internal/usecase/memory"
	principaluc "github.com/stainedhead/ingatan/internal/usecase/principal"
	storeuc "github.com/stainedhead/ingatan/internal/usecase/store"
	"github.com/stretchr/testify/require"
)

// testServer wraps an httptest.Server with helpers for integration tests.
type testServer struct {
	URL    string
	Secret []byte
}

func (ts *testServer) token(t *testing.T, sub, name string, role domain.InstanceRole) string {
	t.Helper()
	return makeJWT(t, ts.Secret, sub, name, role)
}

func (ts *testServer) do(t *testing.T, method, path, token string, body any) *http.Response {
	t.Helper()
	return doRequest(t, method, ts.URL+path, token, body)
}

// testStoreAccess implements memoryuc.StoreAccess for integration tests.
type testStoreAccess struct {
	repo storeuc.Repository
}

func (a *testStoreAccess) GetMemberRole(ctx context.Context, storeName, principalID string) (domain.StoreRole, error) {
	s, err := a.repo.Get(ctx, storeName)
	if err != nil {
		return "", err
	}
	return s.MemberRole(principalID), nil
}

// testSystemService implements rest.SystemService for integration tests.
type testSystemService struct{}

func (s *testSystemService) Health() *rest.HealthStatus {
	return &rest.HealthStatus{Status: "ok", Version: "test"}
}

// newTestServer wires up all real services with file storage in a temp directory
// and returns a running httptest.Server.
func newTestServer(t *testing.T) *testServer {
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

	// Infrastructure: chunker (real)
	chunker := ingest.NewRecursiveChunker(config.ChunkingConfig{
		Strategy:        "recursive",
		ChunkSize:       512,
		ChunkOverlap:    64,
		MaxContentBytes: 1048576,
	})

	// No embedder — keyword search only
	var embedder memoryuc.Embedder

	// Infrastructure: indexes (per-store registries)
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

	// Handlers
	memoryHandler := rest.NewMemoryHandler(memorySvc)
	searchHandler := rest.NewSearchHandler(memorySvc)
	ingestHandler := rest.NewIngestHandler(memorySvc)
	storeHandler := rest.NewStoreHandler(storeSvc)
	principalHandler := rest.NewPrincipalHandler(principalSvc)
	conversationHandler := rest.NewConversationHandler(conversationSvc)
	backupHandler := rest.NewBackupHandler(nil, dataDir)

	// JWT secret and principal lookup
	testSecret := []byte("integration-test-secret")
	lookup := func(ctx context.Context, claims apimw.JWTClaims) (*domain.Principal, error) {
		return principalSvc.GetOrCreate(ctx, claims)
	}
	sysSvc := &testSystemService{}

	// Router (no OTel, no rate limit)
	router := rest.NewRouter(testSecret, lookup, sysSvc, rest.ServerOptions{},
		memoryHandler, searchHandler, ingestHandler,
		storeHandler, principalHandler, conversationHandler,
		backupHandler,
	)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return &testServer{
		URL:    srv.URL,
		Secret: testSecret,
	}
}

// makeJWT creates a signed JWT token for integration tests.
func makeJWT(t *testing.T, secret []byte, sub, name string, role domain.InstanceRole) string {
	t.Helper()
	claims := apimw.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Name: name,
		Type: domain.PrincipalTypeHuman,
		Role: role,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	require.NoError(t, err)
	return signed
}

// doRequest sends an HTTP request with optional JSON body and auth token.
func doRequest(t *testing.T, method, url, token string, body any) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// decodeJSON decodes a JSON response body into v.
func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(v))
}
